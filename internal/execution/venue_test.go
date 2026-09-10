package execution

import (
	"os"
	"strings"
	"testing"

	"github.com/morteza-sakifard/TickEngine/internal/core"
	"github.com/morteza-sakifard/TickEngine/internal/marketdata"
)

func TestMarketBuyFillsAtAsk(t *testing.T) {
	v := NewVenue(Fees{})
	v.SetQuote(marketdata.Quote{BidPx: 100, AskPx: 101})
	id, err := v.Enqueue(Order{Side: core.SideBid, Qty: 2})
	if err != nil || id != 1 {
		t.Fatalf("Enqueue = %d, %v", id, err)
	}
	evs := v.Settle(50)
	if len(evs) != 1 || evs[0].Status != StatusFilled {
		t.Fatalf("Settle = %+v", evs)
	}
	f := evs[0].Fill
	if f.Px != 101 || f.Qty != 2 || f.Side != core.SideBid || f.OrderID != 1 || f.Ts != 50 {
		t.Fatalf("fill = %+v, want ask 101 qty 2", f)
	}
}

func TestMarketSellFillsAtBid(t *testing.T) {
	v := NewVenue(Fees{})
	v.SetQuote(marketdata.Quote{BidPx: 100, AskPx: 101})
	if _, err := v.Enqueue(Order{Side: core.SideAsk, Qty: 1}); err != nil {
		t.Fatal(err)
	}
	evs := v.Settle(1)
	if len(evs) != 1 || evs[0].Fill.Px != 100 {
		t.Fatalf("sell fill = %+v, want bid 100", evs)
	}
}

func TestSettleRejectsWithoutQuote(t *testing.T) {
	v := NewVenue(Fees{})
	if _, err := v.Enqueue(Order{Side: core.SideBid, Qty: 1}); err != nil {
		t.Fatal(err)
	}
	evs := v.Settle(1)
	if len(evs) != 1 || evs[0].Status != StatusRejected {
		t.Fatalf("want rejected, got %+v", evs)
	}
}

func TestFeesOnFill(t *testing.T) {
	v := NewVenue(Fees{CommissionCents: 100, FeeCents: 12})
	v.SetQuote(marketdata.Quote{BidPx: 1, AskPx: 2})
	if _, err := v.Enqueue(Order{Side: core.SideBid, Qty: 3}); err != nil {
		t.Fatal(err)
	}
	f := v.Settle(0)[0].Fill
	if f.FeeCents != 336 {
		t.Fatalf("FeeCents = %d, want 112*3", f.FeeCents)
	}
}

func TestCancelBeforeSettle(t *testing.T) {
	v := NewVenue(Fees{})
	v.SetQuote(marketdata.Quote{BidPx: 1, AskPx: 2})
	id, err := v.Enqueue(Order{Side: core.SideBid, Qty: 1})
	if err != nil {
		t.Fatal(err)
	}
	if err := v.Cancel(id); err != nil {
		t.Fatal(err)
	}
	if evs := v.Settle(1); evs != nil {
		t.Fatalf("canceled order settled: %+v", evs)
	}
	if err := v.Cancel(id); err == nil {
		t.Fatal("want error for already-canceled id")
	}
}

func TestEnqueueRejectsBadOrder(t *testing.T) {
	v := NewVenue(Fees{})
	if _, err := v.Enqueue(Order{Side: core.SideBid, Qty: 0}); err == nil {
		t.Fatal("want error for qty 0")
	}
	if _, err := v.Enqueue(Order{Side: core.SideNone, Qty: 1}); err == nil {
		t.Fatal("want error for side none")
	}
	if _, err := v.Enqueue(Order{Side: core.SideBid, Qty: 1, Kind: 99}); err == nil {
		t.Fatal("want error for unknown kind")
	}
}

func TestSetFeesRejectsNegative(t *testing.T) {
	v := NewVenue(Fees{})
	if err := v.SetFees(Fees{CommissionCents: -1}); err == nil {
		t.Fatal("want error for negative commission")
	}
}

func TestMatchRestingRecordsQuote(t *testing.T) {
	v := NewVenue(Fees{})
	if evs := v.MatchResting(&marketdata.Event{
		Kind:  marketdata.KindQuote,
		Quote: marketdata.Quote{BidPx: 5, AskPx: 6},
	}); evs != nil {
		t.Fatalf("resting fills in step 20: %+v", evs)
	}
	if _, err := v.Enqueue(Order{Side: core.SideBid, Qty: 1}); err != nil {
		t.Fatal(err)
	}
	if px := v.Settle(1)[0].Fill.Px; px != 6 {
		t.Fatalf("px = %d, want ask from MatchResting", px)
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
			"internal/strategy", "internal/portfolio", "internal/feed", "internal/replay",
		} {
			if strings.Contains(s, bad) {
				t.Errorf("%s contains %q", name, strings.TrimSpace(bad))
			}
		}
	}
}
