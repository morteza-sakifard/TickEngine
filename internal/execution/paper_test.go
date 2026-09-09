package execution

import (
	"errors"
	"testing"

	"github.com/morteza-sakifard/market-data-lab/internal/core"
	"github.com/morteza-sakifard/market-data-lab/internal/marketdata"
)

func TestPaperEnqueueGatesVenue(t *testing.T) {
	p, err := NewPaper(Fees{}, Limits{MaxQty: 1})
	if err != nil {
		t.Fatal(err)
	}
	p.Venue().SetQuote(marketdata.Quote{BidPx: 100, AskPx: 101})
	if _, err := p.Enqueue(Order{Side: core.SideBid, Qty: 2}, 0, 0, 1); !errors.Is(err, ErrRiskQty) {
		t.Fatalf("oversize: %v", err)
	}
	if evs := p.Venue().Settle(1); evs != nil {
		t.Fatalf("rejected order must not sit in the venue: %+v", evs)
	}
	id, err := p.Enqueue(Order{Side: core.SideBid, Qty: 1}, 0, 0, 2)
	if err != nil || id != 1 {
		t.Fatalf("accepted id=%d err=%v, want 1 (reject must not consume an id)", id, err)
	}
	evs := p.Venue().Settle(2)
	if len(evs) != 1 || evs[0].Status != StatusFilled || evs[0].Fill.Px != 101 {
		t.Fatalf("paper fill = %+v, want simulated ask 101", evs)
	}
}

func TestPaperKillLeavesInboxEmpty(t *testing.T) {
	p, err := NewPaper(Fees{}, Limits{})
	if err != nil {
		t.Fatal(err)
	}
	p.Venue().SetQuote(marketdata.Quote{BidPx: 1, AskPx: 2})
	p.Kill()
	if _, err := p.Enqueue(Order{Side: core.SideBid, Qty: 1}, 0, 0, 1); !errors.Is(err, ErrRiskKilled) {
		t.Fatalf("killed paper: %v", err)
	}
	if evs := p.Venue().Settle(1); evs != nil {
		t.Fatalf("kill must not enqueue: %+v", evs)
	}
	p.Arm()
	if _, err := p.Enqueue(Order{Side: core.SideBid, Qty: 1}, 0, 0, 2); err != nil {
		t.Fatal(err)
	}
}

func TestPaperCancelAfterKill(t *testing.T) {
	p, err := NewPaper(Fees{}, Limits{})
	if err != nil {
		t.Fatal(err)
	}
	p.Venue().SetQuote(marketdata.Quote{BidPx: 1, AskPx: 2})
	id, err := p.Enqueue(Order{Side: core.SideBid, Qty: 1, Kind: KindLimit, Px: 1}, 0, 0, 1)
	if err != nil {
		t.Fatal(err)
	}
	p.Kill()
	if err := p.Venue().Cancel(id); err != nil {
		t.Fatalf("cancel is not a submit: %v", err)
	}
}

func TestNewPaperRejectsBadLimitsAndFees(t *testing.T) {
	if _, err := NewPaper(Fees{}, Limits{MaxOrders: 1}); err == nil {
		t.Fatal("want error for rate without window")
	}
	if _, err := NewPaper(Fees{CommissionCents: -1}, Limits{}); err == nil {
		t.Fatal("want error for negative fees")
	}
}

func TestNilPaperEnqueue(t *testing.T) {
	var p *Paper
	if _, err := p.Enqueue(Order{Side: core.SideBid, Qty: 1}, 0, 0, 1); err == nil {
		t.Fatal("nil paper")
	}
}
