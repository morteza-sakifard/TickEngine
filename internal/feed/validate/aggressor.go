// Package validate cross-checks a feed's own claims against facts the
// feed itself makes derivable, instead of trusting them. See
// docs/00-architecture.md 5.6 and docs/01-roadmap.md step 5: right now
// its only job is proving that Databento's side column means what its
// docs say it means, because every CVD sign built in step 10 depends on
// that being true.
package validate

import (
	"math"

	"github.com/morteza-sakifard/TickEngine/internal/core"
	"github.com/morteza-sakifard/TickEngine/internal/marketdata"
)

// Category classifies one Trade against the Quote that was in force
// immediately before it. The first two values are the two consistent
// outcomes Databento's own contract predicts; the rest name every way
// reality can diverge from it, so nothing is silently dropped into a
// generic "other" bucket.
type Category uint8

const (
	// CategoryBuyerAtAsk: side=B (buyer aggressor) traded exactly at the
	// prior ask -- lifting the offer, exactly as advertised.
	CategoryBuyerAtAsk Category = iota + 1
	// CategorySellerAtBid: side=A (seller aggressor) traded exactly at
	// the prior bid -- hitting the bid, the other advertised case.
	CategorySellerAtBid
	// CategoryInsideSpread: the trade printed strictly between the
	// prior bid and ask. Not a contradiction -- a resting order inside
	// the spread, or a quote a moment from updating -- but not the
	// clean textbook case either.
	CategoryInsideSpread
	// CategoryOutsideSpread: the trade printed outside [bid, ask]
	// entirely. Usually a quote that had already gone stale by the
	// time this print arrived.
	CategoryOutsideSpread
	// CategoryContradictory: side says one aggressor and the price says
	// the other -- side=B priced at the prior bid, or side=A priced at
	// the prior ask. This is the category that would falsify the
	// aggressor-side assumption if it ever dominated.
	CategoryContradictory
	// CategoryNoAggressor: side=N. Databento is not claiming an
	// aggressor here, so there is nothing to check -- counted so it
	// stays visible in a report, but excluded from Judged and
	// ConsistentRate.
	CategoryNoAggressor
	// CategoryNoQuote: this trade arrived before Observe had seen any
	// Quote, so there is no "before" state to compare it to. Only
	// possible if a decoder's very first event is a Trade.
	CategoryNoQuote
	// CategoryOneSidedBook: the prior quote's bid or ask was
	// Databento's UNDEF_PRICE sentinel, propagated as
	// core.Ticks(math.MaxInt64) (see
	// docs/steps/04-feed-databento.reference.md), so there is no
	// complete spread to compare the trade against.
	CategoryOneSidedBook
)

func (c Category) String() string {
	switch c {
	case CategoryBuyerAtAsk:
		return "buyer at ask (consistent)"
	case CategorySellerAtBid:
		return "seller at bid (consistent)"
	case CategoryInsideSpread:
		return "inside spread"
	case CategoryOutsideSpread:
		return "outside spread"
	case CategoryContradictory:
		return "contradictory"
	case CategoryNoAggressor:
		return "no aggressor (side=N)"
	case CategoryNoQuote:
		return "no quote observed yet"
	case CategoryOneSidedBook:
		return "one-sided book (UNDEF_PRICE)"
	default:
		return "unknown"
	}
}

// isUndef reports whether px is Databento's UNDEF_PRICE sentinel, the
// same math.MaxInt64/MinInt64 check feed/databento.Decoder uses when it
// converts nanounits to Ticks.
func isUndef(px core.Ticks) bool {
	return px == core.Ticks(math.MaxInt64) || px == core.Ticks(math.MinInt64)
}

// Classify reports which Category trade falls into against quote, the
// top-of-book that was current immediately before trade arrived.
// Getting "before" right is the caller's job, not Classify's -- see
// Aggressor.Observe below, which gets it for free from the order
// feed/databento already emits events in.
func Classify(quote marketdata.Quote, trade marketdata.Trade) Category {
	if trade.Aggressor == core.SideNone {
		return CategoryNoAggressor
	}
	if isUndef(quote.BidPx) || isUndef(quote.AskPx) {
		return CategoryOneSidedBook
	}

	px := trade.Px
	if trade.Aggressor == core.SideBid {
		switch {
		case px == quote.AskPx:
			return CategoryBuyerAtAsk
		case px == quote.BidPx:
			return CategoryContradictory
		case px > quote.BidPx && px < quote.AskPx:
			return CategoryInsideSpread
		default:
			return CategoryOutsideSpread
		}
	}

	// trade.Aggressor == core.SideAsk: the only value core.Side has left.
	switch {
	case px == quote.BidPx:
		return CategorySellerAtBid
	case px == quote.AskPx:
		return CategoryContradictory
	case px > quote.BidPx && px < quote.AskPx:
		return CategoryInsideSpread
	default:
		return CategoryOutsideSpread
	}
}

// Aggressor accumulates Classify's verdict across a stream of events.
// It has no notion of a file or a decoder: it only expects to be fed
// one event at a time, in order, the same contract
// docs/00-architecture.md L6 gives replay.Handler.OnEvent -- so it can
// sit behind any feed.Source's output, historical today or live later.
type Aggressor struct {
	haveQuote bool
	quote     marketdata.Quote
	counts    map[Category]int64
}

// NewAggressor returns an Aggressor that has observed nothing yet.
func NewAggressor() *Aggressor {
	return &Aggressor{counts: make(map[Category]int64, 8)}
}

// Observe feeds one event into the accumulator. Call it with every
// event in the order a feed.Source produced them: on a KindQuote, it
// records the new top-of-book for the *next* trade to be judged
// against; on a KindTrade, it judges this one against whatever quote
// it is currently holding. Observe does not retain e.
//
// That "currently holding" quote is correct only because
// feed/databento.Decoder emits a T row's Trade before the Quote that
// row implies (docs/steps/04-feed-databento.reference.md): by the time
// Observe sees that Trade, the quote it still holds is the one from
// the *previous* row -- the state before this trade, exactly what step
// 5 needs, for free, from an ordering decision step 4 made for an
// unrelated reason (a decoder cannot hand back two events from one
// call).
func (a *Aggressor) Observe(e *marketdata.Event) {
	switch e.Kind {
	case marketdata.KindQuote:
		a.quote = e.Quote
		a.haveQuote = true
	case marketdata.KindTrade:
		cat := CategoryNoQuote
		if a.haveQuote {
			cat = Classify(a.quote, e.Trade)
		}
		a.counts[cat]++
	}
}

// Count returns how many trades Observe has placed in category c.
func (a *Aggressor) Count(c Category) int64 { return a.counts[c] }

// Total is every trade Observe has classified, across every category.
func (a *Aggressor) Total() int64 {
	var n int64
	for _, v := range a.counts {
		n += v
	}
	return n
}

// Judged excludes CategoryNoAggressor, CategoryNoQuote and
// CategoryOneSidedBook: none of them is Databento claiming an
// aggressor that could turn out right or wrong, so none belongs in a
// rate that is supposed to answer "is the side column trustworthy?".
func (a *Aggressor) Judged() int64 {
	return a.Total() - a.Count(CategoryNoAggressor) - a.Count(CategoryNoQuote) - a.Count(CategoryOneSidedBook)
}

// ConsistentRate is (BuyerAtAsk+SellerAtBid) / Judged, the number
// docs/01-roadmap.md step 5 requires to be above 0.95 before step 6. It
// is 0, not NaN, when Judged is 0.
func (a *Aggressor) ConsistentRate() float64 {
	judged := a.Judged()
	if judged == 0 {
		return 0
	}
	consistent := a.Count(CategoryBuyerAtAsk) + a.Count(CategorySellerAtBid)
	return float64(consistent) / float64(judged)
}
