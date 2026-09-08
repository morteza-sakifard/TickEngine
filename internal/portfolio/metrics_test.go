package portfolio

import (
	"crypto/sha256"
	"math"
	"strings"
	"testing"

	"github.com/morteza-sakifard/market-data-lab/internal/core"
	"github.com/morteza-sakifard/market-data-lab/internal/execution"
)

func oneTickFills() (execution.Fill, execution.Fill, int64) {
	inst := core.ESZ5()
	const x core.Ticks = 26800
	per := int64(112)
	a := execution.Fill{OrderID: 1, Ts: 10, Px: x, Qty: 1, Side: core.SideBid, FeeCents: per}
	b := execution.Fill{OrderID: 2, Ts: 20, Px: x + 1, Qty: 1, Side: core.SideAsk, FeeCents: per}
	return a, b, CentsPerTick(inst) - 2*per
}

func book(fills ...execution.Fill) *Blotter {
	inst := core.ESZ5()
	var b Blotter
	var p Position
	for _, f := range fills {
		before := p
		p.Apply(inst, f)
		b.Record(f, before, p)
	}
	return &b
}

func TestOneTickMetrics(t *testing.T) {
	a, b, want := oneTickFills()
	bl := book(a, b)
	m := bl.Metrics()
	if m.Trades != 1 || m.Wins != 1 || m.Losses != 0 {
		t.Fatalf("trades=%d wins=%d losses=%d", m.Trades, m.Wins, m.Losses)
	}
	if m.NetPnL != want || m.Expectancy != want || m.AvgWin != want {
		t.Fatalf("net=%d exp=%d avg_win=%d, want %d", m.NetPnL, m.Expectancy, m.AvgWin, want)
	}
	if m.WinRate != 1 || !math.IsInf(m.ProfitFactor, 1) {
		t.Fatalf("win_rate=%v pf=%v", m.WinRate, m.ProfitFactor)
	}
	if m.MaxDrawdown != 112 {
		// first fill is −fee, peak 0 → dd 112
		t.Fatalf("max_dd=%d, want 112 (open fee)", m.MaxDrawdown)
	}
}

func TestLosingRoundTrip(t *testing.T) {
	inst := core.ESZ5()
	var b Blotter
	var p Position
	fills := []execution.Fill{
		{Ts: 1, Px: 100, Qty: 1, Side: core.SideBid},
		{Ts: 2, Px: 99, Qty: 1, Side: core.SideAsk},
	}
	for _, f := range fills {
		before := p
		p.Apply(inst, f)
		b.Record(f, before, p)
	}
	m := b.Metrics()
	want := -CentsPerTick(inst)
	if m.Trades != 1 || m.Losses != 1 || m.AvgLoss != -want || m.NetPnL != want {
		t.Fatalf("%+v, want 1 loss of %d", m, -want)
	}
}

func TestBlotterLines(t *testing.T) {
	a, b, _ := oneTickFills()
	text := book(a, b).Text()
	for _, part := range []string{"id=1", "id=2", "side=bid", "side=ask"} {
		if !strings.Contains(text, part) {
			t.Fatalf("blotter missing %q:\n%s", part, text)
		}
	}
}

func TestMetricsDeterministic(t *testing.T) {
	hash := func() [32]byte {
		a, b, _ := oneTickFills()
		bl := book(a, b)
		return sha256.Sum256([]byte(bl.Text() + bl.Metrics().Text()))
	}
	if a, b := hash(), hash(); a != b {
		t.Fatal("same fills produced different blotter+metrics bytes")
	}
}
