package orderflow

import (
	"testing"
	"time"

	"github.com/morteza-sakifard/market-data-lab/internal/core"
	"github.com/morteza-sakifard/market-data-lab/internal/marketdata"
	"github.com/morteza-sakifard/market-data-lab/internal/session"
)

func tr(px, qty int64, side core.Side, t time.Time) marketdata.Event {
	return marketdata.Event{
		Kind:    marketdata.KindTrade,
		TsEvent: t.UnixNano(),
		Trade:   marketdata.Trade{Px: core.Ticks(px), Qty: core.Qty(qty), Aggressor: side},
	}
}

func TestVWAPHandComputed(t *testing.T) {
	// Independent of the implementation: Σpx*qty / Σqty, truncated.
	var v VWAP
	if v.Value() != 0 {
		t.Fatalf("empty Value = %d, want 0", v.Value())
	}
	e1 := tr(100, 1, core.SideBid, time.Time{})
	e2 := tr(200, 3, core.SideAsk, time.Time{})
	v.OnTrade(&e1)
	v.OnTrade(&e2)
	// (100*1 + 200*3) / 4 = 175
	if got := v.Value(); got != 175 {
		t.Fatalf("VWAP = %d, want 175", got)
	}
}

func TestVWAPIncludesSideNone(t *testing.T) {
	var v VWAP
	none := tr(100, 1, core.SideNone, time.Time{})
	bid := tr(200, 1, core.SideBid, time.Time{})
	v.OnTrade(&none)
	v.OnTrade(&bid)
	if got := v.Value(); got != 150 {
		t.Fatalf("VWAP = %d, want 150 (SideNone still counts as volume)", got)
	}
}

func TestVWAPInsideMinMax(t *testing.T) {
	var v VWAP
	prices := []int64{26800, 26812, 26790, 26804}
	for _, px := range prices {
		e := tr(px, 1, core.SideBid, time.Time{})
		v.OnTrade(&e)
	}
	got := v.Value()
	if got < 26790 || got > 26812 {
		t.Fatalf("VWAP %d outside session range [26790, 26812]", got)
	}
}

func TestResetReturnsToZeroValue(t *testing.T) {
	e := tr(26800, 2, core.SideBid, time.Time{})
	var v VWAP
	var d Delta
	var c CVD
	v.OnTrade(&e)
	d.OnTrade(&e)
	c.OnTrade(&e)
	v.Reset()
	d.Reset()
	c.Reset()
	var emptyV VWAP
	var emptyD Delta
	var emptyC CVD
	if v.Value() != emptyV.Value() || d.Value() != emptyD.Value() || c.Value() != emptyC.Value() {
		t.Fatal("Reset did not restore the zero value")
	}
	v.OnTrade(&e)
	fresh := VWAP{}
	fresh.OnTrade(&e)
	if v.Value() != fresh.Value() {
		t.Fatalf("after Reset+OnTrade VWAP = %d, want %d", v.Value(), fresh.Value())
	}
}

func TestDeltaIgnoresSideNone(t *testing.T) {
	var d Delta
	b := tr(100, 5, core.SideBid, time.Time{})
	a := tr(100, 2, core.SideAsk, time.Time{})
	n := tr(100, 9, core.SideNone, time.Time{})
	d.OnTrade(&b)
	d.OnTrade(&a)
	d.OnTrade(&n)
	if got := d.Value(); got != 3 {
		t.Fatalf("Delta = %d, want 3 (SideNone must not count)", got)
	}
}

func TestCVDRunningSum(t *testing.T) {
	var c CVD
	t0 := time.Date(2025, 9, 23, 14, 0, 0, 0, time.UTC)
	e1 := tr(100, 5, core.SideBid, t0)
	e2 := tr(100, 2, core.SideAsk, t0.Add(time.Second))
	e3 := tr(100, 1, core.SideBid, t0.Add(2*time.Second))
	c.OnTrade(&e1)
	c.OnTrade(&e2)
	c.OnTrade(&e3)
	if got := c.Value(); got != 4 {
		t.Fatalf("CVD = %d, want 4", got)
	}
}

func TestOrchestratorResetsExactlyAtBoundary(t *testing.T) {
	cal, err := session.ForProduct("ES")
	if err != nil {
		t.Fatal(err)
	}
	day := time.Date(2025, time.September, 23, 0, 0, 0, 0, cal.Location)
	open := cal.Schedule.HoursFor(day).RTHOpen
	from := open.Add(-time.Hour)
	to := open.Add(time.Hour)
	bounds := cal.Boundaries(from, to)

	var v VWAP
	o := New(bounds, &v)

	eth := tr(100, 1, core.SideBid, open.Add(-time.Minute))
	o.OnEvent(&eth)
	if got := v.Value(); got != 100 {
		t.Fatalf("ETH VWAP = %d, want 100", got)
	}

	// A trade at the boundary belongs to the new session: Reset first,
	// then OnTrade. Combined VWAP would be 150; session VWAP is 200.
	rth := tr(200, 1, core.SideBid, open)
	o.OnEvent(&rth)
	if got := v.Value(); got != 200 {
		t.Fatalf("VWAP at RTH open = %d, want 200 (reset at the boundary, not later)", got)
	}

	rth2 := tr(200, 1, core.SideBid, open.Add(time.Minute))
	o.OnEvent(&rth2)
	if got := v.Value(); got != 200 {
		t.Fatalf("VWAP after second RTH trade = %d, want 200", got)
	}
}

func TestOrchestratorDoesNotResetEarly(t *testing.T) {
	cal, err := session.ForProduct("ES")
	if err != nil {
		t.Fatal(err)
	}
	day := time.Date(2025, time.September, 23, 0, 0, 0, 0, cal.Location)
	open := cal.Schedule.HoursFor(day).RTHOpen
	bounds := cal.Boundaries(open.Add(-time.Hour), open.Add(time.Hour))

	var v VWAP
	o := New(bounds, &v)
	a := tr(100, 1, core.SideBid, open.Add(-2*time.Second))
	b := tr(300, 1, core.SideBid, open.Add(-time.Nanosecond))
	o.OnEvent(&a)
	o.OnEvent(&b)
	if got := v.Value(); got != 200 {
		t.Fatalf("VWAP 1ns before RTH open = %d, want 200 (must not reset early)", got)
	}
}
