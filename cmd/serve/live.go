package main

import (
	"bytes"
	"context"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/morteza-sakifard/market-data-lab/internal/aggregation"
	"github.com/morteza-sakifard/market-data-lab/internal/chart"
	"github.com/morteza-sakifard/market-data-lab/internal/compose"
	"github.com/morteza-sakifard/market-data-lab/internal/core"
	"github.com/morteza-sakifard/market-data-lab/internal/feed"
	"github.com/morteza-sakifard/market-data-lab/internal/marketdata"
	"github.com/morteza-sakifard/market-data-lab/internal/orderflow"
	"github.com/morteza-sakifard/market-data-lab/internal/replay"
	"github.com/morteza-sakifard/market-data-lab/internal/session"
)

// gate is play/pause/speed for one socket. Hold blocks the engine
// when paused. Speed is read from Sleep so replay.Pacer is not
// mutated from two goroutines.
type gate struct {
	mu    sync.Mutex
	cond  *sync.Cond
	play  bool
	speed float64
}

func newGate() *gate {
	g := &gate{}
	g.cond = sync.NewCond(&g.mu)
	return g
}

func (g *gate) Hold() {
	g.mu.Lock()
	for !g.play {
		g.cond.Wait()
	}
	g.mu.Unlock()
}

func (g *gate) SetPlay(on bool) {
	g.mu.Lock()
	g.play = on
	g.cond.Broadcast()
	g.mu.Unlock()
}

func (g *gate) SetSpeed(s float64) {
	g.mu.Lock()
	if s < 0 {
		s = 0
	}
	g.speed = s
	g.mu.Unlock()
}

func (g *gate) Speed() float64 {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.speed
}

func (s *server) serveReplay(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("session")
	if name == "" {
		name = s.defSess
	}
	sess, set, anchor, err := parseSession(name)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	trades := compose.Keep(s.trades, s.cal, s.date, set)
	c, err := upgradeWS(w, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	defer c.Close()

	header := chart.Header{Symbol: s.inst.Symbol, TradingDate: s.date, Session: sess}
	inst := s.inst
	if err := writeFrame(c, chart.Frame{Type: chart.FrameHello, Instrument: &inst, Header: &header}); err != nil {
		return
	}
	if len(trades) == 0 {
		_ = writeFrame(c, chart.Frame{Type: chart.FrameError, Error: "no trades for " + name})
		return
	}

	g := newGate()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	defer g.SetPlay(true)

	var startOnce sync.Once
	start := func() {
		startOnce.Do(func() {
			go s.runLive(ctx, c, g, trades, sess, set, anchor)
		})
	}

	for {
		raw, err := c.ReadText()
		if err != nil {
			return
		}
		fr, err := chart.ReadFrame(bytes.NewReader(raw))
		if err != nil {
			continue
		}
		switch fr.Type {
		case chart.FramePlay:
			g.SetPlay(true)
			start()
		case chart.FramePause:
			g.SetPlay(false)
		case chart.FrameSpeed:
			g.SetSpeed(fr.Speed)
		}
	}
}

func (s *server) runLive(ctx context.Context, c *wsConn, g *gate, trades []marketdata.Event, sess session.Session, set session.Set, anchor aggregation.Anchor) {
	agg, err := aggregation.New(aggregation.BarSpec{
		Kind:     aggregation.KindTime,
		Interval: s.interval,
		Anchor:   anchor,
		Location: s.cal.Location,
		Sessions: set,
	}, s.cal)
	if err != nil {
		_ = writeFrame(c, chart.Frame{Type: chart.FrameError, Error: err.Error()})
		return
	}
	var vwap orderflow.VWAP
	var cvd orderflow.CVD
	var vp orderflow.VolumeProfile
	tpo := orderflow.NewTPO(tpoOpen(s.cal, s.date, sess))
	var fps []orderflow.Footprint
	from, to := sessionRange(s.cal, s.date, sess)
	orch := orderflow.New(s.cal.Boundaries(from, to), &vwap, &cvd)
	var lastIdx = -1
	var lastSnap time.Time

	eng := replay.New(&sliceSrc{evs: trades})
	eng.SetPacer(&replay.Pacer{
		Speed: 1,
		Step:  true,
		Hold:  g.Hold,
		Sleep: func(d time.Duration) {
			if ctx.Err() != nil {
				return
			}
			rate := g.Speed()
			if rate <= 0 || d <= 0 {
				return
			}
			timer := time.NewTimer(time.Duration(float64(d) / rate))
			defer timer.Stop()
			select {
			case <-ctx.Done():
			case <-timer.C:
			}
		},
	})
	eng.Subscribe(replay.HandlerFunc(func(ev *marketdata.Event) {
		if ctx.Err() != nil {
			return
		}
		orch.OnEvent(ev)
		agg.Add(ev)
		vp.OnTrade(ev)
		tpo.OnTrade(ev)
		bar, idx, ok := lastBar(agg)
		if !ok {
			return
		}
		for len(fps) <= idx {
			fps = append(fps, orderflow.Footprint{})
		}
		fps[idx].OnTrade(ev)
		fp := compose.SnapshotBarFootprint(&fps[idx])
		b := bar
		fr := chart.Frame{
			Type:      chart.FrameBar,
			Index:     idx,
			Bar:       &b,
			VWAP:      vwap.Value(),
			CVD:       core.Ticks(cvd.Value()),
			Footprint: &fp,
		}
		if ev.Kind == marketdata.KindTrade {
			fr.Trade = &chart.TapePrint{
				TsEvent: ev.TsEvent,
				Px:      ev.Trade.Px,
				Qty:     ev.Trade.Qty,
				Side:    ev.Trade.Aggressor,
			}
		}
		now := time.Now()
		if idx != lastIdx || lastSnap.IsZero() || now.Sub(lastSnap) >= 50*time.Millisecond {
			fr.Profile = compose.SnapshotProfile(&vp)
			fr.TPO = compose.TPOViewFrom(tpo)
			lastSnap = now
			lastIdx = idx
		}
		_ = writeFrame(c, fr)
	}))
	if err := eng.Run(ctx); err != nil && ctx.Err() == nil {
		log.Printf("replay: %v", err)
	}
	if ctx.Err() == nil {
		_ = writeFrame(c, chart.Frame{
			Type:    chart.FrameDone,
			Profile: compose.SnapshotProfile(&vp),
			TPO:     compose.TPOViewFrom(tpo),
		})
	}
}

func writeFrame(c *wsConn, f chart.Frame) error {
	var buf bytes.Buffer
	if err := chart.WriteFrame(&buf, f); err != nil {
		return err
	}
	return c.WriteText(buf.Bytes())
}

func lastBar(agg aggregation.Aggregator) (aggregation.Bar, int, bool) {
	bars := agg.Bars()
	if len(bars) == 0 {
		return aggregation.Bar{}, 0, false
	}
	return bars[len(bars)-1], len(bars) - 1, true
}

func tpoOpen(cal session.Calendar, day time.Time, sess session.Session) time.Time {
	h := cal.Schedule.HoursFor(day)
	if sess == session.ETH {
		return h.ETHOpen
	}
	return h.RTHOpen
}

func sessionRange(cal session.Calendar, day time.Time, sess session.Session) (from, to time.Time) {
	h := cal.Schedule.HoursFor(day)
	if sess == session.ETH {
		return h.ETHOpen, h.ETHClose.Add(time.Nanosecond)
	}
	return h.RTHOpen, h.RTHClose.Add(time.Nanosecond)
}

type sliceSrc struct {
	evs []marketdata.Event
	i   int
}

func (s *sliceSrc) Next(dst *marketdata.Event) error {
	if s.i >= len(s.evs) {
		return io.EOF
	}
	*dst = s.evs[s.i]
	s.i++
	return nil
}

func (s *sliceSrc) Close() error { return nil }

var _ feed.Source = (*sliceSrc)(nil)
