package execution

import (
	"github.com/morteza-sakifard/market-data-lab/internal/core"
	"github.com/morteza-sakifard/market-data-lab/internal/marketdata"
	"github.com/morteza-sakifard/market-data-lab/internal/orderbook"
)

func (v *Venue) ensureBook() *orderbook.L3 {
	if v.book == nil {
		v.book = &orderbook.L3{}
	}
	return v.book
}

func (v *Venue) applyBook(ev *marketdata.Event) {
	b := v.ensureBook()
	b.Apply(ev)
	q := b.Quote()
	if v.queueModel() == QueueL3 {
		v.SetQuote(q)
		v.onBook(ev)
		return
	}
	v.applyQuote(q)
}

func (v *Venue) restL3(o Order) {
	r := resting{order: o, remaining: o.Qty, joined: true}
	v.snapshotAhead(&r)
	v.working = append(v.working, r)
}

func (v *Venue) snapshotAhead(r *resting) {
	if v.book == nil {
		return
	}
	fifo := v.book.FIFO(r.order.Side, r.order.Px)
	r.aheadIDs = r.aheadIDs[:0]
	r.ahead = 0
	for _, o := range fifo {
		r.aheadIDs = append(r.aheadIDs, aheadOrd{id: o.ID, qty: o.Qty})
		r.ahead += o.Qty
	}
}

func (v *Venue) onBook(ev *marketdata.Event) {
	bk := ev.Book
	for i := range v.working {
		r := &v.working[i]
		if r.remaining == 0 {
			continue
		}
		switch bk.Action {
		case marketdata.BookClear:
			r.aheadIDs = nil
			r.ahead = 0
		case marketdata.BookCancel:
			v.cutAhead(r, bk.OrderID, 0)
		case marketdata.BookModify:
			old := aheadQty(r, bk.OrderID)
			if old == 0 {
				break
			}
			if bk.Px != r.order.Px || bk.Side != r.order.Side || bk.Qty > old {
				v.cutAhead(r, bk.OrderID, 0)
				break
			}
			v.cutAhead(r, bk.OrderID, bk.Qty)
		}
	}
}

func aheadQty(r *resting, id uint64) core.Qty {
	for _, a := range r.aheadIDs {
		if a.id == id {
			return a.qty
		}
	}
	return 0
}

// cutAhead sets that order's remaining ahead size. qty 0 removes it.
func (v *Venue) cutAhead(r *resting, id uint64, qty core.Qty) {
	n := 0
	var sum core.Qty
	for _, a := range r.aheadIDs {
		if a.id == id {
			if qty <= 0 {
				continue
			}
			a.qty = qty
		}
		r.aheadIDs[n] = a
		sum += a.qty
		n++
	}
	r.aheadIDs = r.aheadIDs[:n]
	r.ahead = sum
}

func (v *Venue) onTradeL3(ev *marketdata.Event) []OrderEvent {
	t := ev.Trade
	ts := ev.TsRecv
	var out []OrderEvent
	for i := range v.working {
		r := &v.working[i]
		if r.remaining == 0 {
			continue
		}
		if tradedThrough(r.order, t.Px) {
			out = append(out, v.take(r, r.remaining, t.Px, ts))
		}
	}
	left := t.Qty
	for i := range v.working {
		if left == 0 {
			break
		}
		r := &v.working[i]
		if r.remaining == 0 || !hitAtPrice(r.order, t) || !r.joined {
			continue
		}
		left = consumeAheadFIFO(r, left)
		if left == 0 {
			break
		}
		take := left
		if take > r.remaining {
			take = r.remaining
		}
		if take > 0 {
			out = append(out, v.take(r, take, r.order.Px, ts))
			left -= take
		}
	}
	v.dropFilled()
	return out
}

func consumeAheadFIFO(r *resting, qty core.Qty) core.Qty {
	n := 0
	var sum core.Qty
	for _, a := range r.aheadIDs {
		if qty == 0 {
			r.aheadIDs[n] = a
			sum += a.qty
			n++
			continue
		}
		if qty >= a.qty {
			qty -= a.qty
			continue
		}
		a.qty -= qty
		qty = 0
		r.aheadIDs[n] = a
		sum += a.qty
		n++
	}
	r.aheadIDs = r.aheadIDs[:n]
	r.ahead = sum
	return qty
}
