package execution

import (
	"os"
	"strings"
	"testing"

	"github.com/morteza-sakifard/TickEngine/internal/core"
	"github.com/morteza-sakifard/TickEngine/internal/marketdata"
)

func restLimit(t *testing.T, side core.Side, px core.Ticks, qty, ahead core.Qty) *Venue {
	t.Helper()
	v := NewVenue(Fees{})
	q := marketdata.Quote{BidPx: 100, AskPx: 101, BidQty: 1, AskQty: 1}
	if side == core.SideBid {
		q.BidPx, q.BidQty = px, ahead
	} else {
		q.AskPx, q.AskQty = px, ahead
	}
	v.SetQuote(q)
	if _, err := v.Enqueue(Order{Kind: KindLimit, Side: side, Qty: qty, Px: px}); err != nil {
		t.Fatal(err)
	}
	if evs := v.Settle(0); evs != nil {
		t.Fatalf("limit should rest, got %+v", evs)
	}
	return v
}

func hit(px core.Ticks, qty core.Qty, ag core.Side) *marketdata.Event {
	return &marketdata.Event{
		Kind:  marketdata.KindTrade,
		Trade: marketdata.Trade{Px: px, Qty: qty, Aggressor: ag},
	}
}

func TestPessimisticNoFillBeforeAheadTrades(t *testing.T) {
	v := restLimit(t, core.SideBid, 100, 1, 100)
	for i := 0; i < 100; i++ {
		if evs := v.MatchResting(hit(100, 1, core.SideAsk)); evs != nil {
			t.Fatalf("fill after %d contracts passed; 100 were ahead", i+1)
		}
	}
	evs := v.MatchResting(hit(100, 1, core.SideAsk))
	if len(evs) != 1 || evs[0].Status != StatusFilled || evs[0].Fill.Qty != 1 || evs[0].Fill.Px != 100 {
		t.Fatalf("want fill on contract 101, got %+v", evs)
	}
}

func TestPessimisticOnePrintOfOneFifty(t *testing.T) {
	v := restLimit(t, core.SideBid, 100, 10, 100)
	evs := v.MatchResting(hit(100, 150, core.SideAsk))
	if len(evs) != 1 || evs[0].Fill.Qty != 10 {
		t.Fatalf("want one fill of 10 after 100 ahead + 50, got %+v", evs)
	}
}

func TestPessimisticCancelDoesNotHelp(t *testing.T) {
	v := restLimit(t, core.SideBid, 100, 1, 100)
	v.MatchResting(&marketdata.Event{
		Kind:  marketdata.KindQuote,
		Quote: marketdata.Quote{BidPx: 100, AskPx: 101, BidQty: 1, AskQty: 1},
	})
	if evs := v.MatchResting(hit(100, 50, core.SideAsk)); evs != nil {
		t.Fatalf("pessimistic filled on a cancel: %+v", evs)
	}
}

func TestProportionalCancelShrinksAhead(t *testing.T) {
	v := restLimit(t, core.SideBid, 100, 1, 100)
	if err := v.SetQueueModel(QueueProportional); err != nil {
		t.Fatal(err)
	}
	v.MatchResting(&marketdata.Event{
		Kind:  marketdata.KindQuote,
		Quote: marketdata.Quote{BidPx: 100, AskPx: 101, BidQty: 50, AskQty: 1},
	})
	if evs := v.MatchResting(hit(100, 50, core.SideAsk)); evs != nil {
		t.Fatalf("ahead should be 50 after proportional shrink, got %+v", evs)
	}
	evs := v.MatchResting(hit(100, 1, core.SideAsk))
	if len(evs) != 1 {
		t.Fatalf("want fill once the remaining 50 traded, got %+v", evs)
	}
}

func TestTradeThroughFillsLimit(t *testing.T) {
	v := restLimit(t, core.SideBid, 100, 2, 100)
	evs := v.MatchResting(hit(99, 1, core.SideAsk))
	if len(evs) != 1 || evs[0].Fill.Qty != 2 || evs[0].Fill.Px != 99 {
		t.Fatalf("trade-through should fill all at the print, got %+v", evs)
	}
}

func TestTradeAtPriceWrongAggressorIgnored(t *testing.T) {
	v := restLimit(t, core.SideBid, 100, 1, 0)
	if evs := v.MatchResting(hit(100, 5, core.SideBid)); evs != nil {
		t.Fatalf("lifting the ask must not fill a bid, got %+v", evs)
	}
}

func TestMarketableLimitFillsAtAsk(t *testing.T) {
	v := NewVenue(Fees{})
	v.SetQuote(marketdata.Quote{BidPx: 100, AskPx: 101, BidQty: 1, AskQty: 4})
	if _, err := v.Enqueue(Order{Kind: KindLimit, Side: core.SideBid, Qty: 1, Px: 101}); err != nil {
		t.Fatal(err)
	}
	evs := v.Settle(7)
	if len(evs) != 1 || evs[0].Fill.Px != 101 || evs[0].Fill.Ts != 7 {
		t.Fatalf("marketable buy = %+v, want ask 101", evs)
	}
}

func TestLimitSellQueue(t *testing.T) {
	v := restLimit(t, core.SideAsk, 101, 1, 100)
	for i := 0; i < 100; i++ {
		if evs := v.MatchResting(hit(101, 1, core.SideBid)); evs != nil {
			t.Fatalf("sell filled after %d, 100 were ahead", i+1)
		}
	}
	if evs := v.MatchResting(hit(101, 1, core.SideBid)); len(evs) != 1 {
		t.Fatalf("want sell fill on 101, got %+v", evs)
	}
}

func TestCancelRestingLimit(t *testing.T) {
	v := restLimit(t, core.SideBid, 100, 1, 0)
	if err := v.Cancel(1); err != nil {
		t.Fatal(err)
	}
	if evs := v.MatchResting(hit(100, 10, core.SideAsk)); evs != nil {
		t.Fatalf("canceled limit filled: %+v", evs)
	}
}

func TestLimitRequiresPrice(t *testing.T) {
	v := NewVenue(Fees{})
	if _, err := v.Enqueue(Order{Kind: KindLimit, Side: core.SideBid, Qty: 1}); err == nil {
		t.Fatal("want error for limit without price")
	}
}

func TestImproveJoinsEmptyLevel(t *testing.T) {
	v := NewVenue(Fees{})
	v.SetQuote(marketdata.Quote{BidPx: 100, AskPx: 102, BidQty: 50, AskQty: 50})
	if _, err := v.Enqueue(Order{Kind: KindLimit, Side: core.SideBid, Qty: 1, Px: 101}); err != nil {
		t.Fatal(err)
	}
	if evs := v.Settle(0); evs != nil {
		t.Fatal(evs)
	}
	evs := v.MatchResting(hit(101, 1, core.SideAsk))
	if len(evs) != 1 {
		t.Fatalf("empty improved level should be first, got %+v", evs)
	}
}

func TestQueueDocStatesNoMarketImpact(t *testing.T) {
	b, err := os.ReadFile("queue.go")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "market impact") {
		t.Fatal("queue.go must state that market impact is not modeled")
	}
}
