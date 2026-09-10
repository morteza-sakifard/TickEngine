package execution

import (
	"fmt"

	"github.com/morteza-sakifard/TickEngine/internal/core"
	"github.com/morteza-sakifard/TickEngine/internal/marketdata"
)

// QueueModel decides when a resting limit moves forward. MBP-1
// shows only the aggregated size at the touch, not order ids, so
// true FIFO needs MBO (QueueL3).
//
// The simulated order is a ghost: it does not change the registered
// book, so market impact is not modeled. Size you see is everyone
// else. Two backtests of the same file therefore cannot disagree
// about whether "your" size was in the quote.
type QueueModel uint8

const (
	// QueuePessimistic requires every contract that was ahead at
	// submit to trade at this price before you fill. A cancel that
	// shrinks the quote does not help you.
	QueuePessimistic QueueModel = iota + 1
	// QueueProportional treats a size drop without a trade as
	// cancels drawn uniformly from the level. Expected contracts
	// lost in front of you are decrease * ahead / level. No RNG:
	// the function is the expectation, so two runs match.
	QueueProportional
	// QueueL3 walks the MBO FIFO in front of the ghost order.
	// Cancels of those order ids move you up; new adds sit behind.
	QueueL3
)

func (m QueueModel) String() string {
	switch m {
	case QueuePessimistic:
		return "pessimistic"
	case QueueProportional:
		return "proportional"
	case QueueL3:
		return "l3"
	default:
		return "pessimistic"
	}
}

type resting struct {
	order     Order
	remaining core.Qty
	ahead     core.Qty
	joined    bool
	aheadIDs  []aheadOrd
}

type aheadOrd struct {
	id  uint64
	qty core.Qty
}

func (v *Venue) SetQueueModel(m QueueModel) error {
	if v == nil {
		return fmt.Errorf("execution: nil venue")
	}
	switch m {
	case 0, QueuePessimistic, QueueProportional, QueueL3:
		v.model = m
		return nil
	default:
		return fmt.Errorf("execution: unknown queue model %d", m)
	}
}

func (v *Venue) queueModel() QueueModel {
	if v == nil || v.model == 0 {
		return QueuePessimistic
	}
	return v.model
}

func (v *Venue) marketableLimit(o Order) (core.Ticks, bool) {
	if v == nil || !v.hasQ {
		return 0, false
	}
	if o.Side == core.SideBid && o.Px >= v.quote.AskPx {
		return v.quote.AskPx, true
	}
	if o.Side == core.SideAsk && o.Px <= v.quote.BidPx {
		return v.quote.BidPx, true
	}
	return 0, false
}

func (v *Venue) rest(o Order) {
	if v.queueModel() == QueueL3 {
		v.restL3(o)
		return
	}
	r := resting{order: o, remaining: o.Qty}
	if v.hasQ {
		r.joined, r.ahead = joinAhead(o, v.quote)
	}
	v.working = append(v.working, r)
}

func joinAhead(o Order, q marketdata.Quote) (bool, core.Qty) {
	if o.Side == core.SideBid {
		if o.Px == q.BidPx {
			return true, q.BidQty
		}
		if o.Px > q.BidPx && o.Px < q.AskPx {
			return true, 0
		}
		return false, 0
	}
	if o.Px == q.AskPx {
		return true, q.AskQty
	}
	if o.Px < q.AskPx && o.Px > q.BidPx {
		return true, 0
	}
	return false, 0
}

func (v *Venue) applyQuote(q marketdata.Quote) {
	prev := v.quote
	had := v.hasQ
	v.SetQuote(q)
	if !had {
		for i := range v.working {
			v.tryJoin(&v.working[i], q)
		}
		return
	}
	prop := v.queueModel() == QueueProportional
	for i := range v.working {
		r := &v.working[i]
		if r.remaining == 0 {
			continue
		}
		if prop {
			shrinkAhead(r, prev, q)
		}
		v.tryJoin(r, q)
	}
}

func (v *Venue) tryJoin(r *resting, q marketdata.Quote) {
	if r.joined {
		return
	}
	r.joined, r.ahead = joinAhead(r.order, q)
}

func shrinkAhead(r *resting, prev, q marketdata.Quote) {
	if !r.joined || r.ahead <= 0 {
		return
	}
	var oldSz, newSz core.Qty
	if r.order.Side == core.SideBid && prev.BidPx == r.order.Px && q.BidPx == r.order.Px {
		oldSz, newSz = prev.BidQty, q.BidQty
	} else if r.order.Side == core.SideAsk && prev.AskPx == r.order.Px && q.AskPx == r.order.Px {
		oldSz, newSz = prev.AskQty, q.AskQty
	} else {
		return
	}
	if oldSz <= 0 || newSz >= oldSz {
		return
	}
	r.ahead = r.ahead * newSz / oldSz
}

func (v *Venue) onTrade(ev *marketdata.Event) []OrderEvent {
	if v.queueModel() == QueueL3 {
		return v.onTradeL3(ev)
	}
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
		if r.ahead > 0 {
			if left <= r.ahead {
				r.ahead -= left
				left = 0
				break
			}
			left -= r.ahead
			r.ahead = 0
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

func tradedThrough(o Order, px core.Ticks) bool {
	if o.Side == core.SideBid {
		return px < o.Px
	}
	return px > o.Px
}

func hitAtPrice(o Order, t marketdata.Trade) bool {
	if t.Px != o.Px {
		return false
	}
	if o.Side == core.SideBid {
		return t.Aggressor == core.SideAsk
	}
	return t.Aggressor == core.SideBid
}

func (v *Venue) take(r *resting, qty core.Qty, px core.Ticks, ts int64) OrderEvent {
	if qty > r.remaining {
		qty = r.remaining
	}
	r.remaining -= qty
	return v.filled(r.order, px, qty, ts)
}

func (v *Venue) filled(o Order, px core.Ticks, qty core.Qty, ts int64) OrderEvent {
	return OrderEvent{
		Order:  o,
		Status: StatusFilled,
		Fill: Fill{
			OrderID:  o.ID,
			Ts:       ts,
			Px:       px,
			Qty:      qty,
			Side:     o.Side,
			FeeCents: v.fees.perContract() * int64(qty),
		},
	}
}

func (v *Venue) dropFilled() {
	n := 0
	for _, r := range v.working {
		if r.remaining > 0 {
			v.working[n] = r
			n++
		}
	}
	v.working = v.working[:n]
}
