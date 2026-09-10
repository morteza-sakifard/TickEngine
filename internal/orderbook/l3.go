package orderbook

import (
	"sort"

	"github.com/morteza-sakifard/TickEngine/internal/core"
	"github.com/morteza-sakifard/TickEngine/internal/marketdata"
)

// BookOrder is one resting order in FIFO order at a price.
type BookOrder struct {
	ID   uint64
	Side core.Side
	Px   core.Ticks
	Qty  core.Qty
}

type bookOrder struct {
	id   uint64
	side core.Side
	px   core.Ticks
	qty  core.Qty
}

// L3 is an MBO book. Each order keeps its place until it cancels,
// fills, or a modify that CME treats as a new order (price change
// or size increase). Size decrease keeps priority. The simulated
// order is not inserted here.
type L3 struct {
	byID map[uint64]bookOrder
	bids map[core.Ticks][]uint64
	asks map[core.Ticks][]uint64
}

func (b *L3) Reset() {
	if b == nil {
		return
	}
	*b = L3{}
}

func (b *L3) Apply(ev *marketdata.Event) {
	if b == nil || ev == nil || ev.Kind != marketdata.KindBook {
		return
	}
	switch ev.Book.Action {
	case marketdata.BookClear:
		b.Reset()
	case marketdata.BookAdd:
		b.add(ev.Book)
	case marketdata.BookCancel:
		b.cancel(ev.Book.OrderID)
	case marketdata.BookModify:
		b.modify(ev.Book)
	}
}

func (b *L3) add(bk marketdata.Book) {
	if bk.OrderID == 0 || bk.Qty <= 0 {
		return
	}
	if _, ok := b.byID[bk.OrderID]; ok {
		b.cancel(bk.OrderID)
	}
	if b.byID == nil {
		b.byID = make(map[uint64]bookOrder)
		b.bids = make(map[core.Ticks][]uint64)
		b.asks = make(map[core.Ticks][]uint64)
	}
	b.byID[bk.OrderID] = bookOrder{id: bk.OrderID, side: bk.Side, px: bk.Px, qty: bk.Qty}
	b.push(bk.Side, bk.Px, bk.OrderID)
}

func (b *L3) cancel(id uint64) {
	o, ok := b.byID[id]
	if !ok {
		return
	}
	b.drop(o.side, o.px, id)
	delete(b.byID, id)
}

func (b *L3) modify(bk marketdata.Book) {
	cur, ok := b.byID[bk.OrderID]
	if !ok {
		b.add(bk)
		return
	}
	if bk.Qty <= 0 {
		b.cancel(bk.OrderID)
		return
	}
	lose := bk.Px != cur.px || bk.Side != cur.side || bk.Qty > cur.qty
	if lose {
		b.cancel(bk.OrderID)
		b.add(bk)
		return
	}
	cur.qty = bk.Qty
	b.byID[bk.OrderID] = cur
}

func (b *L3) push(side core.Side, px core.Ticks, id uint64) {
	if side == core.SideAsk {
		b.asks[px] = append(b.asks[px], id)
		return
	}
	b.bids[px] = append(b.bids[px], id)
}

func (b *L3) drop(side core.Side, px core.Ticks, id uint64) {
	m := b.bids
	if side == core.SideAsk {
		m = b.asks
	}
	ids := m[px]
	n := 0
	for _, x := range ids {
		if x != id {
			ids[n] = x
			n++
		}
	}
	if n == 0 {
		delete(m, px)
		return
	}
	m[px] = ids[:n]
}

func (b *L3) FIFO(side core.Side, px core.Ticks) []BookOrder {
	if b == nil {
		return nil
	}
	var ids []uint64
	if side == core.SideAsk {
		ids = b.asks[px]
	} else {
		ids = b.bids[px]
	}
	if len(ids) == 0 {
		return nil
	}
	out := make([]BookOrder, 0, len(ids))
	for _, id := range ids {
		o, ok := b.byID[id]
		if !ok || o.qty <= 0 {
			continue
		}
		out = append(out, BookOrder{ID: o.id, Side: o.side, Px: o.px, Qty: o.qty})
	}
	return out
}

func (b *L3) BestBid() Level {
	return bestOf(b.bids, b.byID, true)
}

func (b *L3) BestAsk() Level {
	return bestOf(b.asks, b.byID, false)
}

func (b *L3) Spread() core.Ticks {
	return b.BestAsk().Px - b.BestBid().Px
}

func (b *L3) Bids(depth int) []Level {
	return levelsOf(b.bids, b.byID, true, depth)
}

func (b *L3) Asks(depth int) []Level {
	return levelsOf(b.asks, b.byID, false, depth)
}

func (b *L3) Quote() marketdata.Quote {
	bb, ba := b.BestBid(), b.BestAsk()
	return marketdata.Quote{
		BidPx: bb.Px, AskPx: ba.Px,
		BidQty: bb.Qty, AskQty: ba.Qty,
		BidCt: bb.Count, AskCt: ba.Count,
	}
}

func (b *L3) Depth() marketdata.Depth {
	var d marketdata.Depth
	bids, asks := b.Bids(marketdata.MaxDepth), b.Asks(marketdata.MaxDepth)
	for i := 0; i < len(bids) && i < marketdata.MaxDepth; i++ {
		d.Bids[i] = bids[i]
	}
	for i := 0; i < len(asks) && i < marketdata.MaxDepth; i++ {
		d.Asks[i] = asks[i]
	}
	return d
}

func bestOf(levels map[core.Ticks][]uint64, byID map[uint64]bookOrder, bid bool) Level {
	var best Level
	for px, ids := range levels {
		lv := sumLevel(ids, byID, px)
		if lv.Qty <= 0 {
			continue
		}
		if best.Qty == 0 || (bid && lv.Px > best.Px) || (!bid && lv.Px < best.Px) {
			best = lv
		}
	}
	return best
}

func levelsOf(levels map[core.Ticks][]uint64, byID map[uint64]bookOrder, bid bool, depth int) []Level {
	if depth <= 0 || len(levels) == 0 {
		return nil
	}
	out := make([]Level, 0, len(levels))
	for px, ids := range levels {
		lv := sumLevel(ids, byID, px)
		if lv.Qty > 0 {
			out = append(out, lv)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if bid {
			return out[i].Px > out[j].Px
		}
		return out[i].Px < out[j].Px
	})
	if len(out) > depth {
		out = out[:depth]
	}
	return out
}

func sumLevel(ids []uint64, byID map[uint64]bookOrder, px core.Ticks) Level {
	var q core.Qty
	var n uint32
	for _, id := range ids {
		o, ok := byID[id]
		if !ok || o.qty <= 0 {
			continue
		}
		q += o.qty
		n++
	}
	if n == 0 {
		return Level{}
	}
	return Level{Px: px, Qty: q, Count: n}
}
