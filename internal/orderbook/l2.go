package orderbook

import (
	"fmt"

	"github.com/morteza-sakifard/market-data-lab/internal/core"
	"github.com/morteza-sakifard/market-data-lab/internal/marketdata"
)

// Level is one aggregated price. Same shape as marketdata.Level so
// a reconstructed book can be compared to the quote that produced it.
type Level = marketdata.Level

// Gap is a break in the publisher sequence. After a snapshot the
// next sequence is a new baseline, not a gap — reconnects start over.
type Gap struct {
	Prev, Got uint32
}

// L2 is an MBP-N book. Apply replaces the visible levels from the
// event: MBP-10 rows are full snapshots of ten levels, not incremental
// adds. FlagSnapshot still Reset first so a recovery picture cannot
// sit on top of pre-disconnect levels. The reconstructed book does
// not include your own orders.
type L2 struct {
	bids    [marketdata.MaxDepth]Level
	asks    [marketdata.MaxDepth]Level
	lastSeq uint32
	haveSeq bool
	gaps    int
	lastGap Gap
	crossed bool
}

func (b *L2) Reset() {
	if b == nil {
		return
	}
	*b = L2{}
}

func (b *L2) Apply(ev *marketdata.Event) {
	if b == nil || ev == nil || ev.Kind != marketdata.KindQuote {
		return
	}
	if ev.Flags&marketdata.FlagSnapshot != 0 {
		b.Reset()
	}
	if b.haveSeq && ev.Sequence != b.lastSeq && ev.Sequence != b.lastSeq+1 {
		b.lastGap = Gap{Prev: b.lastSeq, Got: ev.Sequence}
		b.gaps++
	}
	b.lastSeq = ev.Sequence
	b.haveSeq = true

	d := ev.Depth
	if d.Bids[0].Qty == 0 && d.Asks[0].Qty == 0 && ev.Quote.BidQty+ev.Quote.AskQty > 0 {
		d = marketdata.DepthFromQuote(ev.Quote)
	}
	b.bids = d.Bids
	b.asks = d.Asks
	bb, ba := b.BestBid(), b.BestAsk()
	b.crossed = bb.Qty > 0 && ba.Qty > 0 && bb.Px > ba.Px
}

func (b *L2) BestBid() Level {
	if b == nil {
		return Level{}
	}
	for i := range b.bids {
		if b.bids[i].Qty > 0 {
			return b.bids[i]
		}
	}
	return Level{}
}

func (b *L2) BestAsk() Level {
	if b == nil {
		return Level{}
	}
	for i := range b.asks {
		if b.asks[i].Qty > 0 {
			return b.asks[i]
		}
	}
	return Level{}
}

func (b *L2) Spread() core.Ticks {
	return b.BestAsk().Px - b.BestBid().Px
}

func (b *L2) Bids(depth int) []Level {
	return takeSide(b.bids[:], depth)
}

func (b *L2) Asks(depth int) []Level {
	return takeSide(b.asks[:], depth)
}

func takeSide(src []Level, depth int) []Level {
	if depth <= 0 {
		return nil
	}
	out := make([]Level, 0, depth)
	for i := 0; i < len(src) && len(out) < depth; i++ {
		if src[i].Qty > 0 {
			out = append(out, src[i])
		}
	}
	return out
}

func (b *L2) Crossed() bool {
	return b != nil && b.crossed
}

func (b *L2) Gaps() int {
	if b == nil {
		return 0
	}
	return b.gaps
}

func (b *L2) LastGap() (Gap, bool) {
	if b == nil || b.gaps == 0 {
		return Gap{}, false
	}
	return b.lastGap, true
}

func (b *L2) AssertNotCrossed() error {
	if b == nil || !b.crossed {
		return nil
	}
	bb, ba := b.BestBid(), b.BestAsk()
	return fmt.Errorf("orderbook: crossed book bid=%d ask=%d", bb.Px, ba.Px)
}
