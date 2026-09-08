// Package compose builds a chart.View from a feed for one trading
// date and session. cmd/render and cmd/serve share it so SVG and
// the browser are the same page. Chart still does not import feed
// or orderflow; this package is the only place that maps both.
package compose

import (
	"fmt"
	"io"
	"time"

	"github.com/morteza-sakifard/market-data-lab/internal/aggregation"
	"github.com/morteza-sakifard/market-data-lab/internal/chart"
	"github.com/morteza-sakifard/market-data-lab/internal/core"
	"github.com/morteza-sakifard/market-data-lab/internal/feed"
	"github.com/morteza-sakifard/market-data-lab/internal/marketdata"
	"github.com/morteza-sakifard/market-data-lab/internal/orderflow"
	"github.com/morteza-sakifard/market-data-lab/internal/session"
)

// Spec is the page to build: one trading date, one session set,
// one bar width. Date is civil midnight in the calendar location.
type Spec struct {
	Date     time.Time
	Session  session.Session
	Sessions session.Set
	Anchor   aggregation.Anchor
	Interval time.Duration
}

const (
	footprintTicksPerRow = 4
	footprintRatio       = 3
	footprintMinStack    = 3
)

// Collect keeps trades whose Classify trading date and session
// match. Quotes are ignored. The decoder reuses one Event, so each
// kept trade is copied.
func Collect(src feed.Source, cal session.Calendar, date time.Time, sessions session.Set) ([]marketdata.Event, error) {
	var trades []marketdata.Event
	var ev marketdata.Event
	for {
		err := src.Next(&ev)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if ev.Kind != marketdata.KindTrade {
			continue
		}
		td, sess := cal.Classify(ev.EventTime())
		if !sameCivil(td, date) || !sessions.Contains(sess) {
			continue
		}
		trades = append(trades, ev)
	}
	return trades, nil
}

// FromTrades aggregates already-collected trades and snapshots
// VWAP, CVD, volume profile, footprints, and TPO into a View.
func FromTrades(trades []marketdata.Event, inst core.Instrument, cal session.Calendar, s Spec) (chart.View, error) {
	kept := Keep(trades, cal, s.Date, s.Sessions)
	agg, err := aggregation.New(aggregation.BarSpec{
		Kind:     aggregation.KindTime,
		Interval: s.Interval,
		Anchor:   s.Anchor,
		Location: cal.Location,
		Sessions: s.Sessions,
	}, cal)
	if err != nil {
		return chart.View{}, err
	}
	for i := range kept {
		agg.Add(&kept[i])
	}
	agg.Flush()
	bars := agg.Bars()
	if len(bars) == 0 {
		return chart.View{}, fmt.Errorf("compose: no trades for %s %s %s",
			inst.Symbol, s.Date.Format("2006-01-02"), s.Session)
	}

	vwap, cvd := SampleFlow(kept, bars, cal)
	var vp orderflow.VolumeProfile
	for i := range kept {
		vp.OnTrade(&kept[i])
	}
	return chart.View{
		Instrument: inst,
		Header: chart.Header{
			Symbol:      inst.Symbol,
			TradingDate: s.Date,
			Session:     s.Session,
		},
		Bars:     bars,
		Overlays: []chart.Series{{Name: "VWAP", Values: vwap}},
		Panels: []chart.Panel{{
			Name:   "CVD",
			Series: []chart.Series{{Name: "CVD", Values: cvd}},
		}},
		Profile:   SnapshotProfile(&vp),
		Footprint: SnapshotFootprints(kept, bars),
		TPO:       SnapshotTPO(kept, cal, s),
	}, nil
}

// Build is Collect + FromTrades for one pass over src.
func Build(src feed.Source, inst core.Instrument, cal session.Calendar, s Spec) (chart.View, error) {
	trades, err := Collect(src, cal, s.Date, s.Sessions)
	if err != nil {
		return chart.View{}, err
	}
	return FromTrades(trades, inst, cal, s)
}

// Keep returns trades whose Classify trading date and session match.
func Keep(trades []marketdata.Event, cal session.Calendar, date time.Time, sessions session.Set) []marketdata.Event {
	out := trades[:0:0]
	for i := range trades {
		td, sess := cal.Classify(trades[i].EventTime())
		if sameCivil(td, date) && sessions.Contains(sess) {
			out = append(out, trades[i])
		}
	}
	return out
}

func sameCivil(a, b time.Time) bool {
	y1, m1, d1 := a.Date()
	y2, m2, d2 := b.Date()
	return y1 == y2 && m1 == m2 && d1 == d2
}

// SampleFlow walks trades in TsEvent order, resets on session
// boundaries, and snapshots VWAP and CVD after the last trade of
// each bar. Sampling lives here because orderflow must not import
// aggregation.
func SampleFlow(trades []marketdata.Event, bars []aggregation.Bar, cal session.Calendar) (vwap, cvd []core.Ticks) {
	vwap = make([]core.Ticks, len(bars))
	cvd = make([]core.Ticks, len(bars))
	if len(bars) == 0 {
		return vwap, cvd
	}
	from, to := bars[0].Start, bars[len(bars)-1].End
	if len(trades) > 0 {
		if t0 := trades[0].EventTime(); t0.Before(from) {
			from = t0
		}
		if t1 := trades[len(trades)-1].EventTime(); !t1.Before(to) {
			to = t1.Add(time.Nanosecond)
		}
	}
	var accVWAP orderflow.VWAP
	var accCVD orderflow.CVD
	o := orderflow.New(cal.Boundaries(from, to), &accVWAP, &accCVD)
	j := 0
	for i := range bars {
		end := bars[i].End
		for j < len(trades) && trades[j].EventTime().Before(end) {
			o.OnEvent(&trades[j])
			j++
		}
		vwap[i] = accVWAP.Value()
		cvd[i] = core.Ticks(accCVD.Value())
	}
	return vwap, cvd
}

// SnapshotProfile copies a session profile into the chart DTO.
func SnapshotProfile(vp *orderflow.VolumeProfile) *chart.ProfileView {
	levels := vp.Levels()
	if len(levels) == 0 {
		return nil
	}
	val, poc, vah := vp.ValueArea()
	out := make([]chart.ProfileLevel, len(levels))
	for i, lv := range levels {
		out[i] = chart.ProfileLevel{Price: lv.Price, Volume: lv.Volume}
	}
	return &chart.ProfileView{Levels: out, POC: poc, VAL: val, VAH: vah}
}

// SnapshotFootprints builds one grouped footprint per bar from the
// same trades that made the bar.
func SnapshotFootprints(trades []marketdata.Event, bars []aggregation.Bar) *chart.FootprintView {
	out := make([]chart.BarFootprint, len(bars))
	j := 0
	for i := range bars {
		var fp orderflow.Footprint
		end := bars[i].End
		for j < len(trades) && trades[j].EventTime().Before(end) {
			fp.OnTrade(&trades[j])
			j++
		}
		out[i] = SnapshotBarFootprint(&fp)
	}
	return &chart.FootprintView{TicksPerRow: footprintTicksPerRow, Bars: out}
}

// SnapshotBarFootprint is one grouped column with imbalance marks.
func SnapshotBarFootprint(fp *orderflow.Footprint) chart.BarFootprint {
	levels := fp.Grouped(footprintTicksPerRow)
	cells := make([]chart.FootprintLevel, len(levels))
	for i, lv := range levels {
		cells[i] = chart.FootprintLevel{Price: lv.Price, Buy: lv.Buy, Sell: lv.Sell}
	}
	stacks := fp.StackedImbalances(footprintMinStack, footprintRatio, footprintTicksPerRow)
	imbs := fp.Imbalances(footprintRatio, footprintTicksPerRow)
	marks := make([]chart.FootprintImb, len(imbs))
	for i, im := range imbs {
		marks[i] = chart.FootprintImb{
			Price:   im.Price,
			Dir:     im.Dir,
			Stacked: inStack(stacks, im.Price, im.Dir, footprintTicksPerRow),
		}
	}
	return chart.BarFootprint{Levels: cells, Imbs: marks}
}

func inStack(stacks []orderflow.Stack, px core.Ticks, dir core.Side, n int) bool {
	step := core.Ticks(n)
	for _, s := range stacks {
		if s.Dir != dir || px < s.From || px > s.To {
			continue
		}
		if step > 0 && (px-s.From)%step == 0 {
			return true
		}
	}
	return false
}

func tpoAnchor(cal session.Calendar, s Spec) time.Time {
	h := cal.Schedule.HoursFor(s.Date)
	if s.Session == session.ETH {
		return h.ETHOpen
	}
	return h.RTHOpen
}

// SnapshotTPO builds a market profile from the session open, not
// from the first trade.
func SnapshotTPO(trades []marketdata.Event, cal session.Calendar, s Spec) *chart.TPOView {
	tpo := orderflow.NewTPO(tpoAnchor(cal, s))
	for i := range trades {
		tpo.OnTrade(&trades[i])
	}
	return TPOViewFrom(tpo)
}

// TPOViewFrom copies the current TPO. Live replay uses this so each
// trade does not rebuild the profile from the whole tape.
func TPOViewFrom(tpo *orderflow.TPO) *chart.TPOView {
	levels := tpo.Levels()
	if len(levels) == 0 {
		return nil
	}
	out := make([]chart.TPOViewLevel, len(levels))
	for i, lv := range levels {
		letters := make([]byte, 0, len(lv.Periods))
		for _, p := range lv.Periods {
			letters = append(letters, orderflow.PeriodLetter(p)...)
		}
		out[i] = chart.TPOViewLevel{
			Price:   lv.Price,
			Letters: string(letters),
			Single:  lv.Count() == 1,
		}
	}
	ibLow, ibHigh, hasIB := tpo.InitialBalance()
	return &chart.TPOView{
		Levels:      out,
		POC:         tpo.POC(),
		IBLow:       ibLow,
		IBHigh:      ibHigh,
		HasIB:       hasIB,
		PeriodCount: tpo.PeriodCount(),
	}
}
