package execution

import (
	"testing"

	"github.com/morteza-sakifard/market-data-lab/internal/core"
	"github.com/morteza-sakifard/market-data-lab/internal/marketdata"
)

func addBook(id uint64, side core.Side, px core.Ticks, qty core.Qty) *marketdata.Event {
	return &marketdata.Event{
		Kind: marketdata.KindBook,
		Book: marketdata.Book{Action: marketdata.BookAdd, OrderID: id, Side: side, Px: px, Qty: qty},
	}
}

func cancelBook(id uint64) *marketdata.Event {
	return &marketdata.Event{
		Kind: marketdata.KindBook,
		Book: marketdata.Book{Action: marketdata.BookCancel, OrderID: id},
	}
}

func restL3Limit(t *testing.T, aheadID uint64, aheadQty core.Qty) *Venue {
	t.Helper()
	v := NewVenue(Fees{})
	if err := v.SetQueueModel(QueueL3); err != nil {
		t.Fatal(err)
	}
	v.MatchResting(addBook(aheadID, core.SideBid, 100, aheadQty))
	v.MatchResting(addBook(aheadID+10, core.SideAsk, 101, 1))
	if _, err := v.Enqueue(Order{Kind: KindLimit, Side: core.SideBid, Qty: 1, Px: 100}); err != nil {
		t.Fatal(err)
	}
	if evs := v.Settle(0); evs != nil {
		t.Fatalf("limit should rest, got %+v", evs)
	}
	return v
}

func TestL3NoFillBeforeAheadTrades(t *testing.T) {
	v := restL3Limit(t, 1, 100)
	for i := 0; i < 100; i++ {
		if evs := v.MatchResting(hit(100, 1, core.SideAsk)); evs != nil {
			t.Fatalf("fill after %d; 100 were ahead", i+1)
		}
	}
	evs := v.MatchResting(hit(100, 1, core.SideAsk))
	if len(evs) != 1 || evs[0].Fill.Qty != 1 || evs[0].Fill.Px != 100 {
		t.Fatalf("want fill on contract 101, got %+v", evs)
	}
}

func TestL3CancelMovesYouUp(t *testing.T) {
	v := restL3Limit(t, 1, 100)
	v.MatchResting(cancelBook(1))
	evs := v.MatchResting(hit(100, 1, core.SideAsk))
	if len(evs) != 1 {
		t.Fatalf("cancel of the ahead order should let the next print fill, got %+v", evs)
	}
}

func TestL3NewAddStaysBehind(t *testing.T) {
	v := restL3Limit(t, 1, 10)
	v.MatchResting(addBook(99, core.SideBid, 100, 50))
	if evs := v.MatchResting(hit(100, 10, core.SideAsk)); evs != nil {
		t.Fatalf("new adds are behind the ghost, 10 ahead should still block: %+v", evs)
	}
	evs := v.MatchResting(hit(100, 1, core.SideAsk))
	if len(evs) != 1 {
		t.Fatalf("want fill after original 10 traded, got %+v", evs)
	}
}

func TestL1VsL3PnLSameTape(t *testing.T) {
	tape := []*marketdata.Event{
		addBook(1, core.SideBid, 100, 100),
		addBook(2, core.SideAsk, 101, 1),
		cancelBook(1),
		hit(100, 1, core.SideAsk),
		addBook(3, core.SideBid, 99, 1),
	}
	pnl := func(m QueueModel) int64 {
		v := NewVenue(Fees{})
		if err := v.SetQueueModel(m); err != nil {
			t.Fatal(err)
		}
		var qty core.Qty
		var avg, lastPx core.Ticks
		book := func(evs []OrderEvent) {
			for _, oe := range evs {
				if oe.Status != StatusFilled {
					continue
				}
				if oe.Fill.Side == core.SideBid {
					qty += oe.Fill.Qty
					avg = oe.Fill.Px
				} else {
					qty -= oe.Fill.Qty
					lastPx = oe.Fill.Px
				}
			}
		}
		for _, ev := range tape[:2] {
			v.MatchResting(ev)
		}
		if _, err := v.Enqueue(Order{Kind: KindLimit, Side: core.SideBid, Qty: 1, Px: 100}); err != nil {
			t.Fatal(err)
		}
		if evs := v.Settle(0); evs != nil {
			t.Fatalf("rest: %+v", evs)
		}
		for _, ev := range tape[2:] {
			book(v.MatchResting(ev))
		}
		if qty > 0 {
			if _, err := v.Enqueue(Order{Side: core.SideAsk, Qty: qty}); err != nil {
				t.Fatal(err)
			}
			book(v.Settle(1))
		}
		if qty != 0 || avg == 0 {
			return 0
		}
		return int64(lastPx-avg) * 1250 // ES cents per tick; no portfolio import (cycle)
	}
	l1, l3 := pnl(QueuePessimistic), pnl(QueueL3)
	if l1 == l3 {
		t.Fatalf("L1 and L3 PnL matched (%d); the cancel should be free only on L3", l1)
	}
	if l3 != -1250 {
		t.Fatalf("L3 bought 100 sold 99, realized=%d", l3)
	}
	if l1 != 0 {
		t.Fatalf("L1 should never fill, realized=%d", l1)
	}
}
