package marketdata

import "github.com/morteza-sakifard/market-data-lab/internal/core"

// Quote is the top-of-book state after an event, exactly as MBP-1
// carries it in bid_px_00..ask_ct_00: BidCt/AskCt are the number of
// individual resting orders aggregated into that one price level, not
// a rank.
type Quote struct {
	BidPx, AskPx   core.Ticks
	BidQty, AskQty core.Qty
	BidCt, AskCt   uint32
}

// Spread is AskPx - BidPx. Step 1's census counts both locked
// (Spread == 0) and crossed (Spread < 0) books in the real file, so
// callers must not assume it is always positive.
func (q Quote) Spread() core.Ticks {
	return q.AskPx - q.BidPx
}

// Mid is the unweighted midpoint, truncated toward zero on an odd sum
// (for a normal positive-price book, that means toward BidPx). On a
// zero-value Quote it is 0, not a division panic: BidPx and AskPx are
// both 0, and dividing by the literal 2 can never fail. The guard that
// matters for "do not panic on an empty quote" lives in Microprice and
// Imbalance below, where the divisor is BidQty+AskQty and really can
// be zero.
func (q Quote) Mid() core.Ticks {
	return (q.BidPx + q.AskPx) / 2
}

// Microprice is the queue-imbalance-weighted mid. It leans toward
// whichever side is thinner (less resting quantity), because a thin
// side is easier for the next trade to move through: heavy size on one
// side acts like a wall that price tends to move away from.
//
// It returns float64 on purpose (see docs/02-conventions.md 2.5, which
// allows float64 for statistical metrics): Microprice is a fair-value
// estimator, not a price feeding tick-exact arithmetic like footprint
// grouping. Rounding it to core.Ticks would erase the signal whenever
// the spread is one tick, which for ES is most of the time, since the
// weighted average of two adjacent integers always truncates back to
// one of them. Zero on an empty quote, instead of a divide-by-zero
// panic.
func (q Quote) Microprice() float64 {
	total := q.BidQty + q.AskQty
	if total == 0 {
		return 0
	}
	weighted := float64(q.AskPx)*float64(q.BidQty) + float64(q.BidPx)*float64(q.AskQty)
	return weighted / float64(total)
}

// Imbalance is (BidQty-AskQty)/(BidQty+AskQty), in [-1, 1]. Positive
// means more resting size on the bid than the ask. Same float64
// reasoning as Microprice, and zero on an empty quote.
func (q Quote) Imbalance() float64 {
	total := q.BidQty + q.AskQty
	if total == 0 {
		return 0
	}
	return float64(q.BidQty-q.AskQty) / float64(total)
}
