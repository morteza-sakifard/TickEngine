package marketdata

import "github.com/morteza-sakifard/market-data-lab/internal/core"

// MaxDepth is Databento MBP-10. MBP-1 fills only slot 0.
const MaxDepth = 10

// Level is one aggregated price on one side. Count is the number of
// resting orders in that Qty, not a rank.
type Level struct {
	Px    core.Ticks
	Qty   core.Qty
	Count uint32
}

// Depth is the book after the event that carries it. Bids[0] and
// Asks[0] are the touch and must match Quote on a KindQuote event.
// A zero Qty means that slot is empty, including UNDEF_PRICE.
type Depth struct {
	Bids [MaxDepth]Level
	Asks [MaxDepth]Level
}

// DepthFromQuote puts L1 into slot 0 so an MBP-1 quote can rebuild L2
// without a second code path.
func DepthFromQuote(q Quote) Depth {
	var d Depth
	d.Bids[0] = Level{Px: q.BidPx, Qty: q.BidQty, Count: q.BidCt}
	d.Asks[0] = Level{Px: q.AskPx, Qty: q.AskQty, Count: q.AskCt}
	return d
}
