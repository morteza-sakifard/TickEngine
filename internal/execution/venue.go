package execution

import (
	"fmt"

	"github.com/morteza-sakifard/market-data-lab/internal/core"
	"github.com/morteza-sakifard/market-data-lab/internal/marketdata"
)

// Venue is a simulated exchange. It does not import the strategy
// package: the runner feeds quotes in and pulls events out.
// Limits rest in working; see queue.go for how ahead is consumed.
type Venue struct {
	nextID  OrderID
	fees    Fees
	quote   marketdata.Quote
	hasQ    bool
	model   QueueModel
	inbox   []Order
	working []resting
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
	v.inbox = append(v.inbox, o)
	return o.ID, nil
}

// Cancel removes an order that has not been settled. A filled
// market order is already gone.
func (v *Venue) Cancel(id OrderID) error {
	if v == nil {
		return fmt.Errorf("execution: nil venue")
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
	switch ev.Kind {
	case marketdata.KindTrade:
		return v.onTrade(ev)
	case marketdata.KindQuote:
		v.applyQuote(ev.Quote)
		return nil
	default:
		return nil
	}
}

// Settle fills market and marketable-limit orders at the touch.
// A non-marketable limit joins the queue and emits nothing until
// MatchResting fills it or Cancel removes it. No quote rejects.
func (v *Venue) Settle(ts int64) []OrderEvent {
	if v == nil || len(v.inbox) == 0 {
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
