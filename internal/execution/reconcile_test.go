package execution

import (
	"testing"

	"github.com/morteza-sakifard/market-data-lab/internal/core"
	"github.com/morteza-sakifard/market-data-lab/internal/marketdata"
)

func TestReconcileGhostAdoptKeep(t *testing.T) {
	local := []Order{
		{ID: 1, Side: core.SideBid, Qty: 1, Kind: KindLimit, Px: 100},
		{ID: 2, Side: core.SideBid, Qty: 1, Kind: KindLimit, Px: 99},
	}
	snap := Snapshot{Working: []Order{
		{ID: 2, Side: core.SideBid, Qty: 1, Kind: KindLimit, Px: 99},
		{ID: 3, Side: core.SideAsk, Qty: 2, Kind: KindLimit, Px: 101},
	}, LastID: 3}
	r := Reconcile(local, snap)
	if len(r.Keep) != 1 || r.Keep[0].ID != 2 {
		t.Fatalf("keep = %+v, want id 2", r.Keep)
	}
	if len(r.Ghost) != 1 || r.Ghost[0].ID != 1 {
		t.Fatalf("ghost = %+v, want id 1", r.Ghost)
	}
	if len(r.Adopt) != 1 || r.Adopt[0].ID != 3 {
		t.Fatalf("adopt = %+v, want id 3", r.Adopt)
	}
	live := r.Live()
	if len(live) != 2 || live[0].ID != 2 || live[1].ID != 3 {
		t.Fatalf("live = %+v, want ids 2,3", live)
	}
}

func TestReconcileEmptyIsEmpty(t *testing.T) {
	r := Reconcile(nil, Snapshot{})
	if len(r.Keep)+len(r.Adopt)+len(r.Ghost) != 0 {
		t.Fatalf("empty: %+v", r)
	}
}

func TestVenueAdoptRestoresLimit(t *testing.T) {
	v := NewVenue(Fees{})
	v.SetQuote(marketdata.Quote{BidPx: 100, AskPx: 101})
	o := Order{ID: 7, Side: core.SideBid, Qty: 1, Kind: KindLimit, Px: 100}
	if err := v.Adopt([]Order{o}, 7); err != nil {
		t.Fatal(err)
	}
	got := v.Working()
	if len(got) != 1 || got[0].ID != 7 {
		t.Fatalf("Working = %+v", got)
	}
	if v.LastID() != 7 {
		t.Fatalf("LastID = %d, want 7", v.LastID())
	}
	id, err := v.Enqueue(Order{Side: core.SideBid, Qty: 1})
	if err != nil || id != 8 {
		t.Fatalf("next id = %d, %v, want 8", id, err)
	}
}

func TestVenueAdoptRejectsZeroID(t *testing.T) {
	v := NewVenue(Fees{})
	err := v.Adopt([]Order{{Side: core.SideBid, Qty: 1, Kind: KindLimit, Px: 1}}, 0)
	if err == nil {
		t.Fatal("adopt without id")
	}
	if v.Working() != nil {
		t.Fatal("failed adopt must leave the venue empty")
	}
}

func TestVenueAdoptReplacesSet(t *testing.T) {
	v := NewVenue(Fees{})
	v.SetQuote(marketdata.Quote{BidPx: 1, AskPx: 2})
	if err := v.Adopt([]Order{{ID: 1, Side: core.SideBid, Qty: 1, Kind: KindLimit, Px: 1}}, 1); err != nil {
		t.Fatal(err)
	}
	if err := v.Adopt([]Order{{ID: 2, Side: core.SideAsk, Qty: 1, Kind: KindLimit, Px: 2}}, 2); err != nil {
		t.Fatal(err)
	}
	got := v.Working()
	if len(got) != 1 || got[0].ID != 2 {
		t.Fatalf("replace: %+v", got)
	}
}
