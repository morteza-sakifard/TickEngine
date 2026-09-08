package execution

import (
	"fmt"

	"github.com/morteza-sakifard/market-data-lab/internal/core"
	"github.com/morteza-sakifard/market-data-lab/internal/marketdata"
)

// Venue is a simulated exchange for market orders. It does not
// import strategy: the runner feeds quotes in and pulls events
// out. Resting limit matching is step 21; MatchResting exists so
// the cascade loop already has a place to call.
type Venue struct {
	nextID OrderID
	fees   Fees
	quote  marketdata.Quote
	hasQ   bool
	inbox  []Order
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
	return fmt.Errorf("execution: order %d not working", id)
}

// MatchResting is step 2 of the cascade. No limits live here yet,
// so it only records a quote from the event and returns nothing.
func (v *Venue) MatchResting(ev *marketdata.Event) []OrderEvent {
	if v == nil || ev == nil {
		return nil
	}
	if ev.Kind == marketdata.KindQuote {
		v.SetQuote(ev.Quote)
	}
	return nil
}

// Settle fills every queued market order at the current quote:
// buy at AskPx, sell at BidPx. No quote means rejected, not a
// fill at last trade.
func (v *Venue) Settle(ts int64) []OrderEvent {
	if v == nil || len(v.inbox) == 0 {
		return nil
	}
	inbox := v.inbox
	v.inbox = nil
	out := make([]OrderEvent, 0, len(inbox))
	for _, o := range inbox {
		px, ok := v.fillPx(o.Side)
		if !ok {
			out = append(out, OrderEvent{Order: o, Status: StatusRejected})
			continue
		}
		out = append(out, OrderEvent{
			Order:  o,
			Status: StatusFilled,
			Fill: Fill{
				OrderID:  o.ID,
				Ts:       ts,
				Px:       px,
				Qty:      o.Qty,
				Side:     o.Side,
				FeeCents: v.fees.perContract() * int64(o.Qty),
			},
		})
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
