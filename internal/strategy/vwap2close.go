package strategy

import (
	"time"

	"github.com/morteza-sakifard/TickEngine/internal/aggregation"
	"github.com/morteza-sakifard/TickEngine/internal/core"
	"github.com/morteza-sakifard/TickEngine/internal/execution"
	"github.com/morteza-sakifard/TickEngine/internal/marketdata"
	"github.com/morteza-sakifard/TickEngine/internal/orderflow"
	"github.com/morteza-sakifard/TickEngine/internal/session"
)

const (
	DefaultSLTicks core.Ticks = 16
	DefaultTPTicks core.Ticks = 32
)

// VWAP2Close is an RTH 1-minute strategy: two closes above session
// VWAP go long, two closes below go short. Stop and target are a
// fixed tick distance from the fill, watched on quotes. One position;
// an opposite 2-close signal is ignored until SL/TP or OnStop.
type VWAP2Close struct {
	SLTicks core.Ticks
	TPTicks core.Ticks
	Qty     core.Qty

	cal        session.Calendar
	agg        aggregation.Aggregator
	vwap       orderflow.VWAP
	seenClosed int
	above      int
	below      int
	want       core.Side
	working    bool
	entry      core.Ticks
	hasTD      bool
	lastTD     time.Time
}

func NewVWAP2Close() *VWAP2Close {
	return &VWAP2Close{SLTicks: DefaultSLTicks, TPTicks: DefaultTPTicks, Qty: 1}
}

func (s *VWAP2Close) OnStart(ctx Context) error {
	if s == nil {
		return nil
	}
	var product string
	if ctx != nil {
		product = ctx.Instrument().Product
	}
	cal, err := session.ForProduct(product)
	if err != nil {
		return err
	}
	agg, err := aggregation.New(aggregation.BarSpec{
		Kind:     aggregation.KindTime,
		Interval: time.Minute,
		Anchor:   aggregation.AnchorRTHOpen,
		Sessions: session.SetRTH,
	}, cal)
	if err != nil {
		return err
	}
	s.cal = cal
	s.agg = agg
	s.vwap.Reset()
	s.seenClosed = 0
	s.above = 0
	s.below = 0
	s.want = core.SideNone
	s.working = false
	s.entry = 0
	s.hasTD = false
	s.lastTD = time.Time{}
	return nil
}

func (s *VWAP2Close) OnEvent(ctx Context, ev *marketdata.Event) error {
	if s == nil || ctx == nil || ev == nil || s.agg == nil {
		return nil
	}
	if ev.Kind == marketdata.KindTrade {
		s.onTrade(ev)
	}
	return s.act(ctx, ev)
}

func (s *VWAP2Close) OnOrder(ctx Context, ev execution.OrderEvent) error {
	if s == nil {
		return nil
	}
	if ev.Status != execution.StatusFilled {
		s.working = false
		return nil
	}
	s.working = false
	if ctx != nil && ctx.Position().Qty == 0 {
		s.want = core.SideNone
		s.above = 0
		s.below = 0
		s.entry = 0
		return nil
	}
	s.entry = ev.Fill.Px
	s.want = core.SideNone
	return nil
}

func (s *VWAP2Close) OnStop(ctx Context) error {
	if s == nil || ctx == nil {
		return nil
	}
	return s.flatten(ctx)
}

func (s *VWAP2Close) onTrade(ev *marketdata.Event) {
	td, sess := s.cal.Classify(ev.EventTime())
	if sess != session.RTH {
		return
	}
	dateChanged := s.hasTD && !td.Equal(s.lastTD)
	s.agg.Add(ev)
	if n := closedBars(s.agg); n > s.seenClosed {
		bars := s.agg.Bars()
		s.onClosed(bars[n-1])
		s.seenClosed = n
	}
	if dateChanged {
		s.vwap.Reset()
		s.above = 0
		s.below = 0
		s.want = core.SideNone
	}
	s.lastTD = td
	s.hasTD = true
	s.vwap.OnTrade(ev)
}

func (s *VWAP2Close) onClosed(b aggregation.Bar) {
	v := s.vwap.Value()
	switch {
	case b.Close > v:
		s.above++
		s.below = 0
	case b.Close < v:
		s.below++
		s.above = 0
	default:
		s.above = 0
		s.below = 0
	}
}

func (s *VWAP2Close) act(ctx Context, ev *marketdata.Event) error {
	if s.working {
		return nil
	}
	if ctx.Position().Qty != 0 {
		if ev.Kind != marketdata.KindQuote {
			return nil
		}
		return s.maybeExit(ctx)
	}
	if s.want == core.SideNone {
		if s.above >= 2 {
			s.want = core.SideBid
		} else if s.below >= 2 {
			s.want = core.SideAsk
		}
	}
	if s.want == core.SideNone || ev.Kind != marketdata.KindQuote {
		return nil
	}
	_, err := ctx.Submit(execution.Order{Side: s.want, Qty: s.qty()})
	if err != nil {
		return err
	}
	s.working = true
	return nil
}

func (s *VWAP2Close) maybeExit(ctx Context) error {
	q, ok := ctx.Quote()
	if !ok {
		return nil
	}
	qty := ctx.Position().Qty
	sl, tp := s.sl(), s.tp()
	if qty > 0 {
		if q.BidPx <= s.entry-sl || q.BidPx >= s.entry+tp {
			return s.flatten(ctx)
		}
	}
	if qty < 0 {
		if q.AskPx >= s.entry+sl || q.AskPx <= s.entry-tp {
			return s.flatten(ctx)
		}
	}
	return nil
}

func (s *VWAP2Close) sl() core.Ticks {
	if s != nil && s.SLTicks > 0 {
		return s.SLTicks
	}
	return DefaultSLTicks
}

func (s *VWAP2Close) tp() core.Ticks {
	if s != nil && s.TPTicks > 0 {
		return s.TPTicks
	}
	return DefaultTPTicks
}

func (s *VWAP2Close) qty() core.Qty {
	if s != nil && s.Qty > 0 {
		return s.Qty
	}
	return 1
}

func closedBars(a aggregation.Aggregator) int {
	n := len(a.Bars())
	if n == 0 {
		return 0
	}
	return n - 1
}

func (s *VWAP2Close) flatten(ctx Context) error {
	q := ctx.Position().Qty
	if q == 0 {
		return nil
	}
	side := core.SideAsk
	if q < 0 {
		side = core.SideBid
		q = -q
	}
	_, err := ctx.Submit(execution.Order{Side: side, Qty: q})
	if err != nil {
		return err
	}
	s.working = true
	return nil
}
