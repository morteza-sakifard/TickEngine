package execution

import (
	"fmt"

	"github.com/morteza-sakifard/market-data-lab/internal/core"
	"github.com/morteza-sakifard/market-data-lab/internal/marketdata"
	"github.com/morteza-sakifard/market-data-lab/internal/orderbook"
)

// Venue is a simulated exchange. It does not import the strategy
// package: the runner feeds quotes in and pulls events out.
// Limits rest in working; see queue.go for how ahead is consumed.
type Venue struct {
	nextID  OrderID
	fees    Fees
	lat     Latency
	now     int64
	quote   marketdata.Quote
	hasQ    bool
	model   QueueModel
	inbox   []Order
	delayed []delayedOrder
	outbox  []delayedEvent
	working []resting
	book    *orderbook.L3
}

func NewVenue(fees Fees) *Venue {
	return &Venue{fees: fees}
}

func (v *Venue) SetFees(fees Fees) error {
	if v == nil {
		return fmt.Errorf("execution: nil venue")
	}
	if err := fees.validate(); err != nil {
		return err
	}
	v.fees = fees
	return nil
}

func (v *Venue) SetLatency(l Latency) error {
	if v == nil {
		return fmt.Errorf("execution: nil venue")
	}
	if err := l.validate(); err != nil {
		return err
	}
	v.lat = l
	return nil
}

func (v *Venue) Sync(ts int64) {
	if v == nil || ts <= v.now {
		return
	}
	v.now = ts
}

func (v *Venue) SetQuote(q marketdata.Quote) {
	if v == nil {
		return
	}
	v.quote = q
	v.hasQ = true
}

// Enqueue assigns an ID and holds the order until Settle. Filling
// inside Submit would change Position mid-OnEvent and skip the
// cascade the architecture requires.
func (v *Venue) Enqueue(o Order) (OrderID, error) {
	if v == nil {
		return 0, fmt.Errorf("execution: nil venue")
	}
	if err := o.validate(); err != nil {
		return 0, err
	}
	v.nextID++
	o.ID = v.nextID
	ready := v.now + v.lat.Entry
	if v.lat.Entry == 0 {
		v.inbox = append(v.inbox, o)
	} else {
		v.delayed = append(v.delayed, delayedOrder{order: o, ready: ready})
	}
	return o.ID, nil
}

// Cancel removes an order that has not been settled. A filled
// market order is already gone.
func (v *Venue) Cancel(id OrderID) error {
	if v == nil {
		return fmt.Errorf("execution: nil venue")
	}
	for i, d := range v.delayed {
		if d.order.ID == id {
			v.delayed = append(v.delayed[:i], v.delayed[i+1:]...)
			return nil
		}
	}
	for i, o := range v.inbox {
		if o.ID == id {
			v.inbox = append(v.inbox[:i], v.inbox[i+1:]...)
			return nil
		}
	}
	for i, r := range v.working {
		if r.order.ID == id {
			v.working = append(v.working[:i], v.working[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("execution: order %d not working", id)
}

// MatchResting is step 2 of the cascade: trades eat the queue,
// quotes update the book. See queue.go.
func (v *Venue) MatchResting(ev *marketdata.Event) []OrderEvent {
	if v == nil || ev == nil {
		return nil
	}
	ts := ev.TsRecv
	v.Sync(ts)
	var raw []OrderEvent
	switch ev.Kind {
	case marketdata.KindTrade:
		raw = v.onTrade(ev)
	case marketdata.KindQuote:
		v.applyQuote(ev.Quote)
	case marketdata.KindBook:
		v.applyBook(ev)
	}
	return v.holdOrEmit(raw, ts)
}

// Settle releases orders whose entry delay has elapsed, fills
// market and marketable-limit orders at the touch, and holds
// fills until response delay elapses. A non-marketable limit
// joins the queue. No quote rejects.
func (v *Venue) Settle(ts int64) []OrderEvent {
	if v == nil {
		return nil
	}
	v.Sync(ts)
	v.releaseDue(ts)
	return v.holdOrEmit(v.settleInbox(ts), ts)
}

// Flush fires every delayed submit and fill. The tape has ended,
// so there is no later print to wait for.
func (v *Venue) Flush() []OrderEvent {
	if v == nil {
		return nil
	}
	for _, d := range v.delayed {
		v.inbox = append(v.inbox, d.order)
	}
	v.delayed = nil
	out := v.settleInbox(v.now)
	for _, d := range v.outbox {
		out = append(out, d.ev)
	}
	v.outbox = nil
	if len(out) == 0 {
		return nil
	}
	return out
}

func (v *Venue) settleInbox(ts int64) []OrderEvent {
	if len(v.inbox) == 0 {
		return nil
	}
	inbox := v.inbox
	v.inbox = nil
	out := make([]OrderEvent, 0, len(inbox))
	for _, o := range inbox {
		if o.kind() == KindLimit {
			if px, ok := v.marketableLimit(o); ok {
				out = append(out, v.filled(o, px, o.Qty, ts))
				continue
			}
			if !v.hasQ {
				out = append(out, OrderEvent{Order: o, Status: StatusRejected})
				continue
			}
			v.rest(o)
			continue
		}
		px, ok := v.fillPx(o.Side)
		if !ok {
			out = append(out, OrderEvent{Order: o, Status: StatusRejected})
			continue
		}
		out = append(out, v.filled(o, px, o.Qty, ts))
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func (v *Venue) fillPx(side core.Side) (core.Ticks, bool) {
	if v == nil || !v.hasQ {
		return 0, false
	}
	if side == core.SideBid {
		return v.quote.AskPx, true
	}
	return v.quote.BidPx, true
}
