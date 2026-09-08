package compose

import (
	"io"
	"testing"
	"time"

	"github.com/morteza-sakifard/market-data-lab/internal/aggregation"
	"github.com/morteza-sakifard/market-data-lab/internal/core"
	"github.com/morteza-sakifard/market-data-lab/internal/marketdata"
	"github.com/morteza-sakifard/market-data-lab/internal/orderflow"
	"github.com/morteza-sakifard/market-data-lab/internal/session"
)

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

func esCal(t *testing.T) session.Calendar {
	t.Helper()
	cal, err := session.ForProduct("ES")
	if err != nil {
		t.Fatal(err)
	}
	return cal
}

func rthTrade(t *testing.T, cal session.Calendar, hour, min int, px, qty int64, side core.Side) marketdata.Event {
	t.Helper()
	ts := time.Date(2025, time.September, 23, hour, min, 0, 0, cal.Location)
	return marketdata.Event{
		Kind:    marketdata.KindTrade,
		TsEvent: ts.UnixNano(),
		Trade:   marketdata.Trade{Px: core.Ticks(px), Qty: core.Qty(qty), Aggressor: side},
	}
}

func TestSampleFlowVWAPAndCVD(t *testing.T) {
	cal := esCal(t)
	trades := []marketdata.Event{
		rthTrade(t, cal, 8, 30, 26800, 5, core.SideBid),
		rthTrade(t, cal, 8, 31, 26804, 2, core.SideAsk),
		rthTrade(t, cal, 8, 36, 26802, 1, core.SideBid),
	}
	agg, err := aggregation.New(aggregation.BarSpec{
		Kind:     aggregation.KindTime,
		Interval: 5 * time.Minute,
		Anchor:   aggregation.AnchorRTHOpen,
		Location: cal.Location,
		Sessions: session.SetRTH,
	}, cal)
	if err != nil {
		t.Fatal(err)
	}
	for i := range trades {
		agg.Add(&trades[i])
	}
	agg.Flush()
	bars := agg.Bars()
	if len(bars) != 2 {
		t.Fatalf("bars = %d, want 2", len(bars))
	}

	vwap, cvd := SampleFlow(trades, bars, cal)

	if vwap[0] != 26801 || vwap[1] != 26801 {
		t.Fatalf("VWAP = %v, want [26801 26801]", vwap)
	}
	lo, hi := bars[0].Low, bars[0].High
	for _, b := range bars[1:] {
		if b.Low < lo {
			lo = b.Low
		}
		if b.High > hi {
			hi = b.High
		}
	}
	for i, px := range vwap {
		if px < lo || px > hi {
			t.Fatalf("vwap[%d]=%d outside session [%d, %d]", i, px, lo, hi)
		}
	}

	var cum core.Qty
	for i, b := range bars {
		cum += b.BuyVolume - b.SellVolume
		if core.Ticks(cum) != cvd[i] {
			t.Fatalf("cvd[%d]=%d, want cumsum of bar delta %d", i, cvd[i], cum)
		}
	}
	if cvd[0] != 3 || cvd[1] != 4 {
		t.Fatalf("CVD = %v, want [3 4]", cvd)
	}

	var vp orderflow.VolumeProfile
	for i := range trades {
		vp.OnTrade(&trades[i])
	}
	prof := SnapshotProfile(&vp)
	if prof == nil {
		t.Fatal("expected a profile")
	}
	if prof.VAL > prof.POC || prof.POC > prof.VAH {
		t.Fatalf("VAL ≤ POC ≤ VAH violated: %d %d %d", prof.VAL, prof.POC, prof.VAH)
	}
	if prof.POC < lo || prof.POC > hi {
		t.Fatalf("POC %d outside session [%d, %d]", prof.POC, lo, hi)
	}
	var sum core.Qty
	for _, lv := range prof.Levels {
		sum += lv.Volume
	}
	if sum != vp.Total() {
		t.Fatalf("profile levels sum %d != total %d", sum, vp.Total())
	}

	fp := SnapshotFootprints(trades, bars)
	if fp == nil || len(fp.Bars) != len(bars) {
		t.Fatalf("footprint bars = %d, want %d", len(fp.Bars), len(bars))
	}
	for i, b := range bars {
		var buy, sell core.Qty
		for _, lv := range fp.Bars[i].Levels {
			buy += lv.Buy
			sell += lv.Sell
		}
		if buy != b.BuyVolume || sell != b.SellVolume {
			t.Fatalf("bar %d footprint buy/sell %d/%d != bar %d/%d", i, buy, sell, b.BuyVolume, b.SellVolume)
		}
	}
}

func TestSnapshotTPOFullRTHHas13Periods(t *testing.T) {
	cal := esCal(t)
	day := time.Date(2025, time.September, 23, 0, 0, 0, 0, cal.Location)
	h := cal.Schedule.HoursFor(day)
	var trades []marketdata.Event
	for i := 0; i < 13; i++ {
		at := h.RTHOpen.Add(time.Duration(i)*orderflow.TPOPeriod + time.Second)
		trades = append(trades, marketdata.Event{
			Kind:    marketdata.KindTrade,
			TsEvent: at.UnixNano(),
			Trade:   marketdata.Trade{Px: 26800, Qty: 1, Aggressor: core.SideBid},
		})
	}
	view := SnapshotTPO(trades, cal, Spec{Date: day, Session: session.RTH})
	if view == nil {
		t.Fatal("expected a TPO view")
	}
	if view.PeriodCount != 13 {
		t.Fatalf("PeriodCount = %d, want 13", view.PeriodCount)
	}
	if view.Levels[0].Letters != "ABCDEFGHIJKLM" {
		t.Fatalf("letters = %q, want A–M", view.Levels[0].Letters)
	}
}

func TestFromTradesNoTradesIsError(t *testing.T) {
	cal := esCal(t)
	_, err := FromTrades(nil, core.ESZ5(), cal, Spec{
		Date:     time.Date(2025, time.September, 23, 0, 0, 0, 0, cal.Location),
		Session:  session.RTH,
		Sessions: session.SetRTH,
		Anchor:   aggregation.AnchorRTHOpen,
		Interval: 5 * time.Minute,
	})
	if err == nil {
		t.Fatal("want error when no trades match")
	}
}

func TestKeepFiltersSession(t *testing.T) {
	cal := esCal(t)
	rth := rthTrade(t, cal, 9, 0, 26800, 1, core.SideBid)
	eth := marketdata.Event{
		Kind:    marketdata.KindTrade,
		TsEvent: time.Date(2025, time.September, 22, 18, 0, 0, 0, cal.Location).UnixNano(),
		Trade:   marketdata.Trade{Px: 26800, Qty: 1, Aggressor: core.SideBid},
	}
	day := time.Date(2025, time.September, 23, 0, 0, 0, 0, cal.Location)
	got := Keep([]marketdata.Event{rth, eth}, cal, day, session.SetRTH)
	if len(got) != 1 || got[0].TsEvent != rth.TsEvent {
		t.Fatalf("keep RTH = %d events, want the 09:00 trade only", len(got))
	}
}
