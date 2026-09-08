package portfolio

import (
	"os"
	"strings"
	"testing"

	"github.com/morteza-sakifard/market-data-lab/internal/core"
	"github.com/morteza-sakifard/market-data-lab/internal/execution"
	"github.com/morteza-sakifard/market-data-lab/internal/marketdata"
)

func TestCentsPerTickES(t *testing.T) {
	got := CentsPerTick(core.ESZ5())
	if got != 1250 {
		t.Fatalf("CentsPerTick(ESZ5) = %d, want 1250 (50 * 0.25 * 100)", got)
	}
}

func TestOneTickRoundTurnPnL(t *testing.T) {
	inst := core.ESZ5()
	const x core.Ticks = 26800
	fees := execution.Fees{CommissionCents: 100, FeeCents: 12}
	per := fees.CommissionCents + fees.FeeCents

	var p Position
	p.Apply(inst, execution.Fill{Px: x, Qty: 1, Side: core.SideBid, FeeCents: per})
	p.Apply(inst, execution.Fill{Px: x + 1, Qty: 1, Side: core.SideAsk, FeeCents: per})

	want := CentsPerTick(inst) - 2*per
	if p.Qty != 0 || p.Realized != want {
		t.Fatalf("qty=%d realized=%d, want flat and %d (1 tick − fees)", p.Qty, p.Realized, want)
	}
}

func TestOneTickNoFees(t *testing.T) {
	inst := core.ESZ5()
	var p Position
	p.Apply(inst, execution.Fill{Px: 100, Qty: 1, Side: core.SideBid})
	p.Apply(inst, execution.Fill{Px: 101, Qty: 1, Side: core.SideAsk})
	if p.Realized != CentsPerTick(inst) {
		t.Fatalf("realized = %d, want %d", p.Realized, CentsPerTick(inst))
	}
}

func TestReverseLongToShort(t *testing.T) {
	inst := core.ESZ5()
	var p Position
	p.Apply(inst, execution.Fill{Px: 100, Qty: 1, Side: core.SideBid})
	p.Apply(inst, execution.Fill{Px: 101, Qty: 2, Side: core.SideAsk})
	if p.Qty != -1 || p.AvgPx != 101 {
		t.Fatalf("pos = %+v, want short 1 @ 101", p)
	}
	if p.Realized != CentsPerTick(inst) {
		t.Fatalf("realized = %d, want 1 tick on the closed long", p.Realized)
	}
}

func TestAddToLongAverages(t *testing.T) {
	inst := core.ESZ5()
	var p Position
	p.Apply(inst, execution.Fill{Px: 100, Qty: 1, Side: core.SideBid})
	p.Apply(inst, execution.Fill{Px: 102, Qty: 1, Side: core.SideBid})
	if p.Qty != 2 || p.AvgPx != 101 {
		t.Fatalf("pos = %+v, want 2 @ 101", p)
	}
}

func TestCoverShort(t *testing.T) {
	inst := core.ESZ5()
	var p Position
	p.Apply(inst, execution.Fill{Px: 100, Qty: 1, Side: core.SideAsk})
	p.Apply(inst, execution.Fill{Px: 99, Qty: 1, Side: core.SideBid})
	if p.Qty != 0 || p.Realized != CentsPerTick(inst) {
		t.Fatalf("cover = %+v, want flat + 1 tick", p)
	}
}

func TestUnrealizedMarksAtExit(t *testing.T) {
	inst := core.ESZ5()
	p := Position{Qty: 1, AvgPx: 100}
	q := marketdata.Quote{BidPx: 101, AskPx: 102}
	if got := Unrealized(p, q, inst); got != CentsPerTick(inst) {
		t.Fatalf("long unrealized = %d, want mark at bid (1 tick)", got)
	}
	p = Position{Qty: -1, AvgPx: 100}
	if got := Unrealized(p, q, inst); got != -2*CentsPerTick(inst) {
		t.Fatalf("short unrealized = %d, want mark at ask (2 ticks against)", got)
	}
	if Total(p, q, inst) != p.Realized+Unrealized(p, q, inst) {
		t.Fatal("Total is not realized + unrealized")
	}
}

func TestPackageConstraints(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		b, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		s := string(b)
		for _, bad := range []string{
			"time.Now(", "os.Open", "\ngo ", " chan ", "\tchan ", "\nselect ", " select ",
			"internal/strategy", "internal/feed", "internal/replay",
		} {
			if strings.Contains(s, bad) {
				t.Errorf("%s contains %q", name, strings.TrimSpace(bad))
			}
		}
	}
}
