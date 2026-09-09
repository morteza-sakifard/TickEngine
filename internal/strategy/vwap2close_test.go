package strategy

import (
	"crypto/sha256"
	"testing"
	"time"

	"github.com/morteza-sakifard/market-data-lab/internal/core"
	"github.com/morteza-sakifard/market-data-lab/internal/marketdata"
	"github.com/morteza-sakifard/market-data-lab/internal/portfolio"
)

func chicagoLoc(t *testing.T) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation("America/Chicago")
	if err != nil {
		t.Fatal(err)
	}
	return loc
}

func rth(t *testing.T, hour, min, sec, nsec int) time.Time {
	t.Helper()
	return time.Date(2025, time.November, 24, hour, min, sec, nsec, chicagoLoc(t))
}

func trAt(t *testing.T, hour, min, sec int, px, qty int64) marketdata.Event {
	t.Helper()
	ts := rth(t, hour, min, sec, 0)
	return marketdata.Event{
		Kind:    marketdata.KindTrade,
		TsEvent: ts.UnixNano(),
		TsRecv:  ts.UnixNano(),
		Trade:   marketdata.Trade{Px: core.Ticks(px), Qty: core.Qty(qty), Aggressor: core.SideBid},
	}
}

func qAt(t *testing.T, hour, min, sec, nsec int, bid, ask core.Ticks) marketdata.Event {
	t.Helper()
	ts := rth(t, hour, min, sec, nsec)
	return marketdata.Event{
		Kind:    marketdata.KindQuote,
		TsEvent: ts.UnixNano(),
		TsRecv:  ts.UnixNano(),
		Quote:   marketdata.Quote{BidPx: bid, AskPx: ask},
	}
}

// twoUpCloses is two RTH 1m bars that close above VWAP. The last
// trade (08:32) closes the second bar; it does not enter.
func twoUpCloses(t *testing.T) []marketdata.Event {
	t.Helper()
	return []marketdata.Event{
		trAt(t, 8, 30, 0, 26800, 10),
		trAt(t, 8, 30, 30, 26820, 1),
		trAt(t, 8, 31, 0, 26820, 1),
		trAt(t, 8, 32, 0, 26820, 1),
	}
}

func twoDownCloses(t *testing.T) []marketdata.Event {
	t.Helper()
	return []marketdata.Event{
		trAt(t, 8, 30, 0, 26820, 10),
		trAt(t, 8, 30, 30, 26800, 1),
		trAt(t, 8, 31, 0, 26800, 1),
		trAt(t, 8, 32, 0, 26800, 1),
	}
}

func runVWAP(t *testing.T, evs []marketdata.Event) *Runtime {
	t.Helper()
	rt := NewRuntime(core.ESZ5(), nil)
	if err := Run(&sliceSrc{evs: evs}, NewVWAP2Close(), rt); err != nil {
		t.Fatal(err)
	}
	return rt
}

func TestVWAP2CloseLongTakeProfit(t *testing.T) {
	evs := twoUpCloses(t)
	evs = append(evs,
		qAt(t, 8, 32, 1, 0, 26819, 26820),
		qAt(t, 8, 32, 2, 0, 26852, 26853),
	)
	rt := runVWAP(t, evs)
	pos := rt.Position()
	want := 32 * portfolio.CentsPerTick(core.ESZ5())
	if pos.Qty != 0 || pos.Realized != want {
		t.Fatalf("qty=%d realized=%d, want 0 and +32 ticks (%d)", pos.Qty, pos.Realized, want)
	}
	if n := len(rt.Blotter().Entries()); n != 2 {
		t.Fatalf("fills = %d, want 2 (entry + TP)", n)
	}
}

func TestVWAP2CloseShortStopLoss(t *testing.T) {
	evs := twoDownCloses(t)
	evs = append(evs,
		qAt(t, 8, 32, 1, 0, 26800, 26801),
		qAt(t, 8, 32, 2, 0, 26815, 26816),
	)
	rt := runVWAP(t, evs)
	pos := rt.Position()
	want := -16 * portfolio.CentsPerTick(core.ESZ5())
	if pos.Qty != 0 || pos.Realized != want {
		t.Fatalf("qty=%d realized=%d, want 0 and -16 ticks (%d)", pos.Qty, pos.Realized, want)
	}
}

func TestVWAP2CloseIgnoresOppositeWhileOpen(t *testing.T) {
	evs := twoUpCloses(t)
	evs = append(evs,
		qAt(t, 8, 32, 1, 0, 26819, 26820),
		trAt(t, 8, 32, 10, 26750, 1),
		trAt(t, 8, 33, 0, 26750, 1),
		trAt(t, 8, 33, 10, 26750, 1),
		trAt(t, 8, 34, 0, 26750, 1),
		qAt(t, 8, 34, 1, 0, 26810, 26811),
		qAt(t, 8, 34, 2, 0, 26852, 26853),
	)
	rt := runVWAP(t, evs)
	pos := rt.Position()
	if pos.Qty != 0 {
		t.Fatalf("qty=%d, want flat after TP", pos.Qty)
	}
	if n := len(rt.Blotter().Entries()); n != 2 {
		t.Fatalf("fills = %d, want 2 (no reverse on opposite 2-close)", n)
	}
	want := 32 * portfolio.CentsPerTick(core.ESZ5())
	if pos.Realized != want {
		t.Fatalf("realized=%d, want TP %d", pos.Realized, want)
	}
}

func TestVWAP2CloseETHDoesNotSignal(t *testing.T) {
	evs := []marketdata.Event{
		trAt(t, 7, 0, 0, 26800, 10),
		trAt(t, 7, 0, 30, 26820, 1),
		trAt(t, 7, 1, 0, 26820, 1),
		trAt(t, 7, 2, 0, 26820, 1),
		qAt(t, 8, 30, 0, 0, 26819, 26820),
	}
	rt := runVWAP(t, evs)
	if rt.Position().Qty != 0 {
		t.Fatalf("ETH tape entered qty=%d", rt.Position().Qty)
	}
	if n := len(rt.Blotter().Entries()); n != 0 {
		t.Fatalf("ETH tape produced %d fills", n)
	}
}

func TestVWAP2CloseNoLookaheadOnNextBar(t *testing.T) {
	// First tick of 08:32 at 30000 would pull VWAP above the 08:31
	// close if applied before the comparison. Entry must still fire.
	evs := []marketdata.Event{
		trAt(t, 8, 30, 0, 26800, 10),
		trAt(t, 8, 30, 30, 26820, 1),
		trAt(t, 8, 31, 0, 26820, 1),
		trAt(t, 8, 32, 0, 30000, 1),
		qAt(t, 8, 32, 1, 0, 26819, 26820),
	}
	rt := runVWAP(t, evs)
	fills := rt.Blotter().Entries()
	if len(fills) == 0 || fills[0].Side != core.SideBid || fills[0].Px != 26820 {
		t.Fatalf("fills=%v, want a long entry at 26820 (08:31 close vs VWAP without the 30000 print)", fills)
	}
}

func TestVWAP2CloseEqualVWAPBreaksStreak(t *testing.T) {
	evs := []marketdata.Event{
		trAt(t, 8, 30, 0, 26800, 1),
		trAt(t, 8, 31, 0, 26800, 1),
		trAt(t, 8, 32, 0, 26800, 1),
		qAt(t, 8, 32, 1, 0, 26799, 26800),
	}
	rt := runVWAP(t, evs)
	if rt.Position().Qty != 0 {
		t.Fatalf("qty=%d, want 0 (close == VWAP is neither side)", rt.Position().Qty)
	}
}

func TestVWAP2CloseBlotterDeterministic(t *testing.T) {
	evs := twoUpCloses(t)
	evs = append(evs,
		qAt(t, 8, 32, 1, 0, 26819, 26820),
		qAt(t, 8, 32, 2, 0, 26852, 26853),
	)
	hash := func() [32]byte {
		rt := runVWAP(t, evs)
		b := rt.Blotter()
		return sha256.Sum256([]byte(b.Text() + b.Metrics().Text()))
	}
	if a, b := hash(), hash(); a != b {
		t.Fatal("same tape produced different blotter bytes")
	}
}

func TestVWAP2CloseOnStopFlattens(t *testing.T) {
	evs := twoUpCloses(t)
	evs = append(evs, qAt(t, 8, 32, 1, 0, 26819, 26820))
	rt := runVWAP(t, evs)
	pos := rt.Position()
	want := -portfolio.CentsPerTick(core.ESZ5())
	if pos.Qty != 0 || pos.Realized != want {
		t.Fatalf("qty=%d realized=%d, want flatten at last bid (−1 tick = %d)", pos.Qty, pos.Realized, want)
	}
}
