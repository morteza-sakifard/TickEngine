package main

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/morteza-sakifard/market-data-lab/internal/core"
	"github.com/morteza-sakifard/market-data-lab/internal/execution"
	"github.com/morteza-sakifard/market-data-lab/internal/feed/databento"
	"github.com/morteza-sakifard/market-data-lab/internal/marketdata"
	"github.com/morteza-sakifard/market-data-lab/internal/portfolio"
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

func TestNewStrategy(t *testing.T) {
	if _, err := newStrategy("buyhold"); err != nil {
		t.Fatal(err)
	}
	if _, err := newStrategy("vwap2close"); err != nil {
		t.Fatal(err)
	}
	if _, err := newStrategy("macd"); err == nil {
		t.Fatal("unknown strategy should error")
	}
}

func TestLookupInstrument(t *testing.T) {
	got, err := lookupInstrument("ESZ5")
	if err != nil || got.ID != core.ESZ5().ID {
		t.Fatalf("ESZ5: got %+v err=%v", got, err)
	}
	if _, err := lookupInstrument("NQZ5"); err == nil {
		t.Fatal("unknown symbol should error")
	}
}

func TestBuyHoldOneTick(t *testing.T) {
	const x core.Ticks = 26800
	src := &sliceSrc{evs: []marketdata.Event{
		{Kind: marketdata.KindQuote, TsRecv: 1, Quote: marketdata.Quote{BidPx: x - 1, AskPx: x}},
		{Kind: marketdata.KindQuote, TsRecv: 2, Quote: marketdata.Quote{BidPx: x + 1, AskPx: x + 2}},
	}}
	var buf bytes.Buffer
	pos, _, err := runBacktest(src, core.ESZ5(), execution.Fees{}, execution.Latency{}, &buf, &buyHold{})
	if err != nil {
		t.Fatal(err)
	}
	// buy at ask X on first quote; flatten on stop at last bid X+1
	want := portfolio.CentsPerTick(core.ESZ5())
	if pos.Qty != 0 || pos.Realized != want {
		t.Fatalf("qty=%d realized=%d, want 0 and %d\n%s", pos.Qty, pos.Realized, want, buf.String())
	}
	if !strings.Contains(buf.String(), "fill id=1") || !strings.Contains(buf.String(), "realized=") {
		t.Fatalf("output missing fill or summary:\n%s", buf.String())
	}
}

func TestBuyHoldOnMBP10Fixture(t *testing.T) {
	f, err := os.Open("../../testdata/mbp10_sample.csv")
	if err != nil {
		t.Fatal(err)
	}
	src, err := databento.Open(f, core.ESZ5(), databento.SchemaMBP10)
	if err != nil {
		t.Fatal(err)
	}
	defer src.Close()
	pos, _, err := runBacktest(src, core.ESZ5(), execution.Fees{}, execution.Latency{}, io.Discard, &buyHold{})
	if err != nil {
		t.Fatal(err)
	}
	// one quote: buy ask 6715.00 (26860), stop sells bid 6714.75 (26859)
	want := -portfolio.CentsPerTick(core.ESZ5())
	if pos.Qty != 0 || pos.Realized != want {
		t.Fatalf("mbp10 fixture qty=%d realized=%d, want 0 and %d", pos.Qty, pos.Realized, want)
	}
}

func TestBuyHoldFees(t *testing.T) {
	const x core.Ticks = 26800
	src := &sliceSrc{evs: []marketdata.Event{
		{Kind: marketdata.KindQuote, TsRecv: 1, Quote: marketdata.Quote{BidPx: x - 1, AskPx: x}},
	}}
	fees := execution.Fees{CommissionCents: 100, FeeCents: 12}
	pos, _, err := runBacktest(src, core.ESZ5(), fees, execution.Latency{}, io.Discard, &buyHold{})
	if err != nil {
		t.Fatal(err)
	}
	// one quote: buy at X, stop sells at same bid X-1 → −1 tick − 2 fills of fees
	want := -portfolio.CentsPerTick(core.ESZ5()) - 2*(fees.CommissionCents+fees.FeeCents)
	if pos.Realized != want {
		t.Fatalf("realized = %d, want %d", pos.Realized, want)
	}
}
