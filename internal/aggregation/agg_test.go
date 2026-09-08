package aggregation

import (
	"testing"
	"time"

	"github.com/morteza-sakifard/market-data-lab/internal/core"
	"github.com/morteza-sakifard/market-data-lab/internal/marketdata"
	"github.com/morteza-sakifard/market-data-lab/internal/session"
)

func chicago(t *testing.T) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation("America/Chicago")
	if err != nil {
		t.Fatal(err)
	}
	return loc
}

func esCal(t *testing.T) session.Calendar {
	t.Helper()
	return session.Calendar{Location: chicago(t), Schedule: session.ESRegularSchedule()}
}

func ct(t *testing.T, y int, m time.Month, d, hh, mm, ss int) time.Time {
	t.Helper()
	return time.Date(y, m, d, hh, mm, ss, 0, chicago(t))
}

func utc(y int, m time.Month, d, hh, mm, ss int) time.Time {
	return time.Date(y, m, d, hh, mm, ss, 0, time.UTC)
}

func tr(t time.Time, px, qty int64, side core.Side) marketdata.Event {
	return marketdata.Event{
		Kind:    marketdata.KindTrade,
		TsEvent: t.UnixNano(),
		Trade:   marketdata.Trade{Px: core.Ticks(px), Qty: core.Qty(qty), Aggressor: side},
	}
}

func quote(t time.Time) marketdata.Event {
	return marketdata.Event{Kind: marketdata.KindQuote, TsEvent: t.UnixNano()}
}

func mustNew(t *testing.T, spec BarSpec, cal session.Calendar) Aggregator {
	t.Helper()
	a, err := New(spec, cal)
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func flush(t *testing.T, a Aggregator) []Bar {
	t.Helper()
	a.Flush()
	return a.Bars()
}

func TestFloorDiv(t *testing.T) {
	tests := []struct {
		a, b, want int64
	}{
		{7, 4, 1},
		{-7, 4, -2},
		{-8, 4, -2},
		{0, 4, 0},
		{-1, 4, -1},
	}
	for _, tt := range tests {
		if got := floorDiv(tt.a, tt.b); got != tt.want {
			t.Errorf("floorDiv(%d,%d) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestRTH30mFirstBarIs0830(t *testing.T) {
	cal := esCal(t)
	a := mustNew(t, BarSpec{
		Kind:     KindTime,
		Interval: 30 * time.Minute,
		Anchor:   AnchorRTHOpen,
		Sessions: session.SetRTH,
	}, cal)
	ev := tr(ct(t, 2025, time.September, 23, 8, 31, 0), 26800, 1, core.SideBid)
	a.Add(&ev)
	bars := flush(t, a)
	if len(bars) != 1 {
		t.Fatalf("len = %d, want 1", len(bars))
	}
	want := ct(t, 2025, time.September, 23, 8, 30, 0)
	if !bars[0].Start.Equal(want) {
		t.Errorf("Start = %s, want %s", bars[0].Start, want)
	}
	if bars[0].Session != session.RTH {
		t.Errorf("Session = %v, want RTH", bars[0].Session)
	}
}

func TestRTH4hWhereTruncateBreaks(t *testing.T) {
	cal := esCal(t)
	a := mustNew(t, BarSpec{
		Kind:     KindTime,
		Interval: 4 * time.Hour,
		Anchor:   AnchorRTHOpen,
		Sessions: session.SetRTH,
	}, cal)
	e1 := tr(ct(t, 2025, time.September, 23, 8, 31, 0), 26800, 1, core.SideBid)
	e2 := tr(ct(t, 2025, time.September, 23, 12, 31, 0), 26805, 1, core.SideAsk)
	a.Add(&e1)
	a.Add(&e2)
	bars := flush(t, a)
	if len(bars) != 2 {
		t.Fatalf("len = %d, want 2", len(bars))
	}
	want0 := ct(t, 2025, time.September, 23, 8, 30, 0)
	want1 := ct(t, 2025, time.September, 23, 12, 30, 0)
	if !bars[0].Start.Equal(want0) {
		t.Errorf("bars[0].Start = %s, want %s", bars[0].Start, want0)
	}
	if !bars[1].Start.Equal(want1) {
		t.Errorf("bars[1].Start = %s, want %s", bars[1].Start, want1)
	}
	// Truncate(4h) on the first trade (13:31 UTC) lands on 12:00 UTC = 07:00 CT.
	trunc := ct(t, 2025, time.September, 23, 8, 31, 0).UTC().Truncate(4 * time.Hour)
	if trunc.Equal(bars[0].Start.UTC()) {
		t.Fatal("4h RTH start accidentally matches Truncate — the test is not proving the fix")
	}
}

func TestRTH1dWhereTruncateBreaks(t *testing.T) {
	cal := esCal(t)
	a := mustNew(t, BarSpec{
		Kind:     KindTime,
		Interval: 24 * time.Hour,
		Anchor:   AnchorRTHOpen,
		Sessions: session.SetRTH,
	}, cal)
	e1 := tr(ct(t, 2025, time.September, 23, 8, 31, 0), 26800, 1, core.SideBid)
	e2 := tr(ct(t, 2025, time.September, 23, 14, 59, 0), 26803, 1, core.SideAsk)
	e3 := tr(ct(t, 2025, time.September, 24, 8, 31, 0), 26810, 1, core.SideBid)
	a.Add(&e1)
	a.Add(&e2)
	a.Add(&e3)
	bars := flush(t, a)
	if len(bars) != 2 {
		t.Fatalf("len = %d, want 2 (one bar per trading date)", len(bars))
	}
	want0 := ct(t, 2025, time.September, 23, 8, 30, 0)
	want1 := ct(t, 2025, time.September, 24, 8, 30, 0)
	if !bars[0].Start.Equal(want0) {
		t.Errorf("bars[0].Start = %s, want %s", bars[0].Start, want0)
	}
	if !bars[1].Start.Equal(want1) {
		t.Errorf("bars[1].Start = %s, want %s", bars[1].Start, want1)
	}
	if bars[0].Volume != 2 || bars[1].Volume != 1 {
		t.Errorf("volumes = %d,%d want 2,1", bars[0].Volume, bars[1].Volume)
	}
	trunc := ct(t, 2025, time.September, 23, 8, 31, 0).UTC().Truncate(24 * time.Hour)
	if trunc.Equal(bars[0].Start.UTC()) {
		t.Fatal("1d RTH start accidentally matches Truncate — the test is not proving the fix")
	}
}

func TestETH30mBeforeRTHOpenUsesRealFloor(t *testing.T) {
	cal := esCal(t)
	a := mustNew(t, BarSpec{
		Kind:     KindTime,
		Interval: 30 * time.Minute,
		Anchor:   AnchorRTHOpen,
	}, cal)
	ev := tr(ct(t, 2025, time.September, 22, 17, 5, 0), 26800, 1, core.SideBid)
	a.Add(&ev)
	bars := flush(t, a)
	if len(bars) != 1 {
		t.Fatalf("len = %d, want 1", len(bars))
	}
	want := ct(t, 2025, time.September, 22, 17, 0, 0)
	if !bars[0].Start.Equal(want) {
		t.Errorf("Start = %s, want %s (negative elapsed must floor, not truncate toward zero)", bars[0].Start, want)
	}
}

// alwaysOpen is a test Schedule that trades 24 hours every civil day,
// including the Sunday 02:00 Chicago DST instants that ES itself is
// closed through. The DST tests need real trades on those instants;
// using ESRegularSchedule would Classify them Closed and drop them.
type alwaysOpen struct{}

func (alwaysOpen) HoursFor(tradingDate time.Time) session.Hours {
	loc := tradingDate.Location()
	y, m, d := tradingDate.Date()
	midnight := time.Date(y, m, d, 0, 0, 0, 0, loc)
	return session.Hours{
		ETHOpen:  midnight,
		ETHClose: midnight.Add(24 * time.Hour),
		RTHOpen:  time.Date(y, m, d, 8, 30, 0, 0, loc),
		RTHClose: time.Date(y, m, d, 15, 0, 0, 0, loc),
	}
}

func TestDSTFallBackDoesNotMergeBars(t *testing.T) {
	cal := session.Calendar{Location: chicago(t), Schedule: alwaysOpen{}}
	a := mustNew(t, BarSpec{
		Kind:     KindTime,
		Interval: 30 * time.Minute,
		Anchor:   AnchorMidnightLocal,
	}, cal)
	// Midnight 2025-11-02 Chicago is 05:00 UTC. Fall-back is 07:00 UTC.
	// Six trades, 30 real minutes apart, must be six bars — not four
	// merged by a wall-clock (hour, minute) key on the repeated 01:xx.
	start := time.Date(2025, time.November, 2, 5, 0, 0, 0, time.UTC)
	for i := 0; i < 6; i++ {
		ev := tr(start.Add(time.Duration(i)*30*time.Minute), 26800, 1, core.SideBid)
		a.Add(&ev)
	}
	bars := flush(t, a)
	if len(bars) != 6 {
		t.Fatalf("len = %d, want 6 (DST fall-back must not merge 30m bars)", len(bars))
	}
	for i, bar := range bars {
		if bar.End.Sub(bar.Start) != 30*time.Minute {
			t.Errorf("bars[%d] duration = %s, want 30m", i, bar.End.Sub(bar.Start))
		}
	}
}

func TestDSTSpringForwardKeepsRealDuration(t *testing.T) {
	cal := session.Calendar{Location: chicago(t), Schedule: alwaysOpen{}}
	a := mustNew(t, BarSpec{
		Kind:     KindTime,
		Interval: 30 * time.Minute,
		Anchor:   AnchorMidnightLocal,
	}, cal)
	// Midnight 2025-03-09 Chicago is 06:00 UTC. Spring-forward is 08:00 UTC.
	start := time.Date(2025, time.March, 9, 6, 0, 0, 0, time.UTC)
	for i := 0; i < 8; i++ {
		ev := tr(start.Add(time.Duration(i)*30*time.Minute), 26800, 1, core.SideBid)
		a.Add(&ev)
	}
	bars := flush(t, a)
	if len(bars) != 8 {
		t.Fatalf("len = %d, want 8 (spring-forward must not drop a 30m bar)", len(bars))
	}
	for i, bar := range bars {
		if bar.End.Sub(bar.Start) != 30*time.Minute {
			t.Errorf("bars[%d] duration = %s, want 30m", i, bar.End.Sub(bar.Start))
		}
	}
}

func TestUTCEpoch5mMatchesOldTruncate(t *testing.T) {
	cal := esCal(t)
	a := mustNew(t, BarSpec{
		Kind:     KindTime,
		Interval: 5 * time.Minute,
		Anchor:   AnchorUTCEpoch,
	}, cal)
	ev := tr(utc(2025, time.September, 23, 10, 31, 17), 26800, 1, core.SideBid)
	a.Add(&ev)
	bars := flush(t, a)
	want := utc(2025, time.September, 23, 10, 30, 0)
	if len(bars) != 1 || !bars[0].Start.Equal(want) {
		t.Fatalf("Start = %v, want %v", bars, want)
	}
}

func TestOHLCVOneBucketTicks(t *testing.T) {
	cal := esCal(t)
	a := mustNew(t, BarSpec{Kind: KindTime, Interval: 5 * time.Minute, Anchor: AnchorUTCEpoch}, cal)
	for _, ev := range []marketdata.Event{
		tr(utc(2025, time.September, 23, 14, 31, 0), 26800, 2, core.SideBid),
		tr(utc(2025, time.September, 23, 14, 32, 0), 26805, 1, core.SideBid),
		tr(utc(2025, time.September, 23, 14, 33, 0), 26798, 3, core.SideAsk),
		tr(utc(2025, time.September, 23, 14, 34, 0), 26803, 4, core.SideAsk),
	} {
		e := ev
		a.Add(&e)
	}
	bars := flush(t, a)
	if len(bars) != 1 {
		t.Fatalf("len = %d, want 1", len(bars))
	}
	got := bars[0]
	if got.Open != 26800 || got.High != 26805 || got.Low != 26798 || got.Close != 26803 {
		t.Errorf("OHLC = %d/%d/%d/%d, want 26800/26805/26798/26803", got.Open, got.High, got.Low, got.Close)
	}
	if got.Volume != 10 || got.BuyVolume != 3 || got.SellVolume != 7 {
		t.Errorf("vol=%d buy=%d sell=%d, want 10/3/7", got.Volume, got.BuyVolume, got.SellVolume)
	}
	if got.BuyVolume+got.SellVolume != got.Volume {
		t.Errorf("Buy+Sell != Volume")
	}
}

func TestEmptyBucketProducesNoBar(t *testing.T) {
	cal := esCal(t)
	a := mustNew(t, BarSpec{Kind: KindTime, Interval: 5 * time.Minute, Anchor: AnchorUTCEpoch}, cal)
	e1 := tr(utc(2025, time.September, 23, 14, 31, 0), 26800, 1, core.SideBid)
	e2 := tr(utc(2025, time.September, 23, 14, 41, 0), 26804, 1, core.SideBid)
	a.Add(&e1)
	a.Add(&e2)
	bars := flush(t, a)
	if len(bars) != 2 {
		t.Fatalf("len = %d, want 2", len(bars))
	}
}

func TestMaintenanceGapDoesNotCorruptNextBar(t *testing.T) {
	cal := esCal(t)
	a := mustNew(t, BarSpec{Kind: KindTime, Interval: 5 * time.Minute, Anchor: AnchorUTCEpoch}, cal)
	e1 := tr(utc(2025, time.September, 23, 20, 14, 30), 26800, 1, core.SideBid)
	e2 := tr(utc(2025, time.September, 23, 22, 6, 0), 27000, 5, core.SideAsk)
	a.Add(&e1)
	a.Add(&e2)
	bars := flush(t, a)
	if len(bars) != 2 {
		t.Fatalf("len = %d, want 2", len(bars))
	}
	if bars[0].Close != 26800 || bars[0].Volume != 1 {
		t.Errorf("bars[0] = %+v", bars[0])
	}
	if bars[1].Open != 27000 || bars[1].Volume != 5 {
		t.Errorf("bars[1] = %+v", bars[1])
	}
}

func TestBarsDoesNotCloseOpenBucket(t *testing.T) {
	cal := esCal(t)
	a := mustNew(t, BarSpec{Kind: KindTime, Interval: 5 * time.Minute, Anchor: AnchorUTCEpoch}, cal)
	e1 := tr(utc(2025, time.September, 23, 14, 31, 0), 100, 1, core.SideBid)
	e2 := tr(utc(2025, time.September, 23, 14, 32, 0), 101, 1, core.SideBid)
	e3 := tr(utc(2025, time.September, 23, 14, 34, 0), 99, 1, core.SideAsk)
	a.Add(&e1)
	if got := len(a.Bars()); got != 1 {
		t.Fatalf("after 1st: len = %d, want 1", got)
	}
	a.Add(&e2)
	if got := len(a.Bars()); got != 1 {
		t.Fatalf("after 2nd: len = %d, want 1", got)
	}
	a.Add(&e3)
	bars := flush(t, a)
	if len(bars) != 1 {
		t.Fatalf("len = %d, want 1", len(bars))
	}
	got := bars[0]
	if got.Open != 100 || got.High != 101 || got.Low != 99 || got.Close != 99 || got.Volume != 3 {
		t.Errorf("got %+v", got)
	}
}

func TestQuotesAndClosedAreIgnored(t *testing.T) {
	cal := esCal(t)
	a := mustNew(t, BarSpec{Kind: KindTime, Interval: 30 * time.Minute, Anchor: AnchorRTHOpen}, cal)
	q := quote(ct(t, 2025, time.September, 23, 8, 31, 0))
	closed := tr(ct(t, 2025, time.September, 23, 16, 30, 0), 26800, 1, core.SideBid)
	a.Add(&q)
	a.Add(&closed)
	if got := len(flush(t, a)); got != 0 {
		t.Fatalf("len = %d, want 0", got)
	}
}

func TestSessionsFilterRTHOnly(t *testing.T) {
	cal := esCal(t)
	a := mustNew(t, BarSpec{
		Kind:     KindTime,
		Interval: 30 * time.Minute,
		Anchor:   AnchorRTHOpen,
		Sessions: session.SetRTH,
	}, cal)
	eth := tr(ct(t, 2025, time.September, 22, 17, 5, 0), 26800, 1, core.SideBid)
	rth := tr(ct(t, 2025, time.September, 23, 8, 31, 0), 26800, 1, core.SideBid)
	a.Add(&eth)
	a.Add(&rth)
	bars := flush(t, a)
	if len(bars) != 1 {
		t.Fatalf("len = %d, want 1 (ETH dropped)", len(bars))
	}
	if !bars[0].Start.Equal(ct(t, 2025, time.September, 23, 8, 30, 0)) {
		t.Errorf("Start = %s", bars[0].Start)
	}
}

func TestVolumeBarExactCountExceptLast(t *testing.T) {
	cal := esCal(t)
	a := mustNew(t, BarSpec{Kind: KindVolume, Count: 500, Sessions: session.SetRTH}, cal)
	trades := []marketdata.Event{
		tr(ct(t, 2025, time.September, 23, 8, 31, 0), 26800, 200, core.SideBid),
		tr(ct(t, 2025, time.September, 23, 8, 32, 0), 26801, 300, core.SideAsk),
		tr(ct(t, 2025, time.September, 23, 8, 33, 0), 26802, 300, core.SideBid),
		tr(ct(t, 2025, time.September, 23, 8, 34, 0), 26803, 300, core.SideAsk), // split 200+100
		tr(ct(t, 2025, time.September, 23, 8, 35, 0), 26804, 50, core.SideBid),
	}
	for i := range trades {
		a.Add(&trades[i])
	}
	bars := flush(t, a)
	if len(bars) != 3 {
		t.Fatalf("len = %d, want 3", len(bars))
	}
	if bars[0].Volume != 500 || bars[1].Volume != 500 {
		t.Errorf("closed volumes = %d,%d want 500,500", bars[0].Volume, bars[1].Volume)
	}
	if bars[2].Volume != 150 {
		t.Errorf("last volume = %d, want 150", bars[2].Volume)
	}
	for i, bar := range bars {
		if bar.BuyVolume+bar.SellVolume != bar.Volume {
			t.Errorf("bars[%d] Buy+Sell (%d+%d) != Volume %d", i, bar.BuyVolume, bar.SellVolume, bar.Volume)
		}
	}
	if bars[1].SellVolume != 200 {
		t.Errorf("bars[1].SellVolume = %d, want 200 (split of the 300-lot ask)", bars[1].SellVolume)
	}
}

func TestTickBarCount(t *testing.T) {
	cal := esCal(t)
	a := mustNew(t, BarSpec{Kind: KindTick, Count: 2}, cal)
	for i, px := range []int64{26800, 26801, 26802} {
		ev := tr(ct(t, 2025, time.September, 23, 8, 31, i), px, 1, core.SideBid)
		a.Add(&ev)
	}
	bars := flush(t, a)
	if len(bars) != 2 {
		t.Fatalf("len = %d, want 2", len(bars))
	}
	if bars[0].TradeCount != 2 || bars[1].TradeCount != 1 {
		t.Errorf("counts = %d,%d want 2,1", bars[0].TradeCount, bars[1].TradeCount)
	}
}

func TestRangeBarClosesWhenBandExceeded(t *testing.T) {
	cal := esCal(t)
	a := mustNew(t, BarSpec{Kind: KindRange, Band: 4}, cal) // 4 ticks = 1.00 on ES
	e1 := tr(ct(t, 2025, time.September, 23, 8, 31, 0), 26800, 1, core.SideBid)
	e2 := tr(ct(t, 2025, time.September, 23, 8, 31, 1), 26803, 1, core.SideBid)
	e3 := tr(ct(t, 2025, time.September, 23, 8, 31, 2), 26805, 1, core.SideAsk)
	a.Add(&e1)
	a.Add(&e2)
	a.Add(&e3)
	bars := flush(t, a)
	if len(bars) != 2 {
		t.Fatalf("len = %d, want 2", len(bars))
	}
	if bars[0].High-bars[0].Low != 3 {
		t.Errorf("bar0 range = %d, want 3", bars[0].High-bars[0].Low)
	}
	if bars[1].Open != 26805 {
		t.Errorf("bar1.Open = %d, want 26805", bars[1].Open)
	}
}

func TestNewRejectsBadSpec(t *testing.T) {
	cal := esCal(t)
	if _, err := New(BarSpec{}, cal); err == nil {
		t.Fatal("want error for zero Kind")
	}
	if _, err := New(BarSpec{Kind: KindTime}, cal); err == nil {
		t.Fatal("want error for Interval 0")
	}
	if _, err := New(BarSpec{Kind: KindVolume, Count: 0}, cal); err == nil {
		t.Fatal("want error for Count 0")
	}
}
