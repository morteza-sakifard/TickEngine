package orderbook

import (
	"testing"

	"github.com/morteza-sakifard/market-data-lab/internal/core"
	"github.com/morteza-sakifard/market-data-lab/internal/marketdata"
)

func bookEv(act marketdata.BookAction, id uint64, side core.Side, px core.Ticks, qty core.Qty) marketdata.Event {
	return marketdata.Event{
		Kind: marketdata.KindBook,
		Book: marketdata.Book{Action: act, OrderID: id, Side: side, Px: px, Qty: qty},
	}
}

func TestL3FIFOAndCancel(t *testing.T) {
	var b L3
	a1 := bookEv(marketdata.BookAdd, 1, core.SideBid, 100, 10)
	a2 := bookEv(marketdata.BookAdd, 2, core.SideBid, 100, 20)
	b.Apply(&a1)
	b.Apply(&a2)
	fifo := b.FIFO(core.SideBid, 100)
	if len(fifo) != 2 || fifo[0].ID != 1 || fifo[1].ID != 2 {
		t.Fatalf("FIFO add order = %+v", fifo)
	}
	c1 := bookEv(marketdata.BookCancel, 1, core.SideBid, 100, 10)
	b.Apply(&c1)
	fifo = b.FIFO(core.SideBid, 100)
	if len(fifo) != 1 || fifo[0].ID != 2 || b.BestBid().Qty != 20 {
		t.Fatalf("after cancel %+v best=%+v", fifo, b.BestBid())
	}
}

func TestL3ModifyKeepsPriorityOnReduce(t *testing.T) {
	var b L3
	a1 := bookEv(marketdata.BookAdd, 1, core.SideBid, 100, 10)
	a2 := bookEv(marketdata.BookAdd, 2, core.SideBid, 100, 5)
	b.Apply(&a1)
	b.Apply(&a2)
	m := bookEv(marketdata.BookModify, 1, core.SideBid, 100, 4)
	b.Apply(&m)
	fifo := b.FIFO(core.SideBid, 100)
	if len(fifo) != 2 || fifo[0].ID != 1 || fifo[0].Qty != 4 || fifo[1].ID != 2 {
		t.Fatalf("reduce must keep place: %+v", fifo)
	}
}

func TestL3ModifyLosesPriorityOnIncreaseOrPrice(t *testing.T) {
	var b L3
	a1 := bookEv(marketdata.BookAdd, 1, core.SideBid, 100, 10)
	a2 := bookEv(marketdata.BookAdd, 2, core.SideBid, 100, 5)
	b.Apply(&a1)
	b.Apply(&a2)
	up := bookEv(marketdata.BookModify, 1, core.SideBid, 100, 12)
	b.Apply(&up)
	fifo := b.FIFO(core.SideBid, 100)
	if len(fifo) != 2 || fifo[0].ID != 2 || fifo[1].ID != 1 {
		t.Fatalf("increase goes to back: %+v", fifo)
	}
	px := bookEv(marketdata.BookModify, 2, core.SideBid, 99, 5)
	b.Apply(&px)
	if len(b.FIFO(core.SideBid, 100)) != 1 || b.FIFO(core.SideBid, 100)[0].ID != 1 {
		t.Fatalf("price change left 100: %+v", b.FIFO(core.SideBid, 100))
	}
	if len(b.FIFO(core.SideBid, 99)) != 1 || b.FIFO(core.SideBid, 99)[0].ID != 2 {
		t.Fatalf("price change at 99: %+v", b.FIFO(core.SideBid, 99))
	}
}

func TestL3ClearResets(t *testing.T) {
	var b L3
	a := bookEv(marketdata.BookAdd, 1, core.SideBid, 100, 3)
	b.Apply(&a)
	clr := bookEv(marketdata.BookClear, 0, core.SideNone, 0, 0)
	b.Apply(&clr)
	if b.BestBid().Qty != 0 || len(b.FIFO(core.SideBid, 100)) != 0 {
		t.Fatal("clear must empty the book")
	}
}

func TestL3AggregatesMatchL2(t *testing.T) {
	var l3 L3
	adds := []marketdata.Event{
		bookEv(marketdata.BookAdd, 1, core.SideBid, 100, 5),
		bookEv(marketdata.BookAdd, 2, core.SideBid, 100, 3),
		bookEv(marketdata.BookAdd, 3, core.SideBid, 99, 7),
		bookEv(marketdata.BookAdd, 4, core.SideAsk, 101, 4),
		bookEv(marketdata.BookAdd, 5, core.SideAsk, 102, 2),
	}
	for i := range adds {
		l3.Apply(&adds[i])
	}
	var l2 L2
	q := l3.Quote()
	l2.Apply(&marketdata.Event{Kind: marketdata.KindQuote, Quote: q, Depth: l3.Depth()})
	if l2.BestBid() != l3.BestBid() || l2.BestAsk() != l3.BestAsk() {
		t.Fatalf("touch L2=%+v/%+v L3=%+v/%+v", l2.BestBid(), l2.BestAsk(), l3.BestBid(), l3.BestAsk())
	}
	lb, la := l2.Bids(10), l3.Bids(10)
	if len(lb) != len(la) {
		t.Fatalf("bid depth %d vs %d", len(lb), len(la))
	}
	for i := range lb {
		if lb[i] != la[i] {
			t.Fatalf("bid[%d] L2=%+v L3=%+v", i, lb[i], la[i])
		}
	}
	ab, aa := l2.Asks(10), l3.Asks(10)
	for i := range ab {
		if ab[i] != aa[i] {
			t.Fatalf("ask[%d] L2=%+v L3=%+v", i, ab[i], aa[i])
		}
	}
	if l3.BestBid().Count != 2 || l3.BestAsk().Count != 1 {
		t.Fatalf("counts bid=%d ask=%d", l3.BestBid().Count, l3.BestAsk().Count)
	}
}
