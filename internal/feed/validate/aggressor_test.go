package validate

import (
	"math"
	"testing"

	"github.com/morteza-sakifard/market-data-lab/internal/core"
	"github.com/morteza-sakifard/market-data-lab/internal/marketdata"
)

func quote(bid, ask core.Ticks) marketdata.Quote {
	return marketdata.Quote{BidPx: bid, AskPx: ask}
}

func trade(side core.Side, px core.Ticks) marketdata.Trade {
	return marketdata.Trade{Aggressor: side, Px: px}
}

func TestClassify(t *testing.T) {
	q := quote(100, 104) // a 4-tick-wide book for the general cases

	tests := []struct {
		name  string
		quote marketdata.Quote
		trade marketdata.Trade
		want  Category
	}{
		{"buyer lifts the ask", q, trade(core.SideBid, 104), CategoryBuyerAtAsk},
		{"seller hits the bid", q, trade(core.SideAsk, 100), CategorySellerAtBid},
		{"buyer priced at the bid is contradictory", q, trade(core.SideBid, 100), CategoryContradictory},
		{"seller priced at the ask is contradictory", q, trade(core.SideAsk, 104), CategoryContradictory},
		{"buyer inside the spread", q, trade(core.SideBid, 102), CategoryInsideSpread},
		{"seller inside the spread", q, trade(core.SideAsk, 102), CategoryInsideSpread},
		{"buyer above the ask", q, trade(core.SideBid, 110), CategoryOutsideSpread},
		{"seller below the bid", q, trade(core.SideAsk, 90), CategoryOutsideSpread},
		{"side=N is never judged", q, trade(core.SideNone, 999), CategoryNoAggressor},
		{
			"undefined ask makes the book one-sided",
			quote(100, core.Ticks(math.MaxInt64)), trade(core.SideBid, 100),
			CategoryOneSidedBook,
		},
		{
			"undefined bid makes the book one-sided",
			quote(core.Ticks(math.MaxInt64), 104), trade(core.SideAsk, 104),
			CategoryOneSidedBook,
		},
		{
			"locked book calls the advertised side consistent",
			quote(100, 100), trade(core.SideBid, 100),
			CategoryBuyerAtAsk,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Classify(tt.quote, tt.trade); got != tt.want {
				t.Errorf("Classify() = %s, want %s", got, tt.want)
			}
		})
	}
}

func quoteEvent(bid, ask core.Ticks) *marketdata.Event {
	return &marketdata.Event{Kind: marketdata.KindQuote, Quote: quote(bid, ask)}
}

func tradeEvent(side core.Side, px core.Ticks) *marketdata.Event {
	return &marketdata.Event{Kind: marketdata.KindTrade, Trade: trade(side, px)}
}

// TestAggressorJudgesAgainstTheQuoteBeforeTheTrade pins the design
// decision in Observe's doc comment: a trade sandwiched between an
// older quote and its own row's post-trade quote must be judged
// against the *older* one.
func TestAggressorJudgesAgainstTheQuoteBeforeTheTrade(t *testing.T) {
	a := NewAggressor()
	a.Observe(quoteEvent(100, 104))          // the state before the trade below
	a.Observe(tradeEvent(core.SideAsk, 104)) // seller printed at the *old* ask: contradictory
	a.Observe(quoteEvent(104, 108))          // this row's own post-trade quote, ask moved

	if got := a.Count(CategoryContradictory); got != 1 {
		t.Errorf("CategoryContradictory = %d, want 1", got)
	}
	if got := a.Count(CategorySellerAtBid); got != 0 {
		t.Errorf("CategorySellerAtBid = %d, want 0 (would be 1 if Observe judged "+
			"against the post-trade quote instead of the pre-trade one)", got)
	}
}

func TestAggressorNoQuoteYet(t *testing.T) {
	a := NewAggressor()
	a.Observe(tradeEvent(core.SideBid, 100)) // no Quote observed before this Trade

	if got := a.Count(CategoryNoQuote); got != 1 {
		t.Errorf("CategoryNoQuote = %d, want 1", got)
	}
	if got := a.Judged(); got != 0 {
		t.Errorf("Judged() = %d, want 0", got)
	}
}

func TestAggressorTotalsJudgedAndRate(t *testing.T) {
	a := NewAggressor()
	a.Observe(quoteEvent(100, 104))
	a.Observe(tradeEvent(core.SideBid, 104)) // consistent
	a.Observe(quoteEvent(100, 104))
	a.Observe(tradeEvent(core.SideAsk, 100)) // consistent
	a.Observe(quoteEvent(100, 104))
	a.Observe(tradeEvent(core.SideBid, 100)) // contradictory
	a.Observe(quoteEvent(100, 104))
	a.Observe(tradeEvent(core.SideNone, 102)) // not judged

	if got, want := a.Total(), int64(4); got != want {
		t.Errorf("Total() = %d, want %d", got, want)
	}
	if got, want := a.Judged(), int64(3); got != want {
		t.Errorf("Judged() = %d, want %d", got, want)
	}
	if got, want := a.ConsistentRate(), 2.0/3.0; got != want {
		t.Errorf("ConsistentRate() = %v, want %v", got, want)
	}
}

func TestAggressorConsistentRateOnEmpty(t *testing.T) {
	a := NewAggressor()
	if got := a.ConsistentRate(); got != 0 {
		t.Errorf("ConsistentRate() on no trades = %v, want 0", got)
	}
}
