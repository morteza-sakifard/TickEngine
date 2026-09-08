package execution

import (
	"fmt"

	"github.com/morteza-sakifard/market-data-lab/internal/core"
)

// OrderID is assigned by the venue. Zero means "not yet accepted".
type OrderID uint64

// Kind is how the order meets the book. Only market is implemented
// in this step; a limit would sit on the book and is step 21.
type Kind uint8

const (
	KindMarket Kind = iota + 1
)

func (k Kind) String() string {
	switch k {
	case KindMarket:
		return "market"
	default:
		return "unknown"
	}
}

// Order is what a strategy submits. SideBid is a buy (lifts the
// ask). SideAsk is a sell (hits the bid). Qty must be positive;
// direction lives on Side, not on the sign of Qty.
type Order struct {
	ID         OrderID
	Instrument core.InstrumentID
	Side       core.Side
	Qty        core.Qty
	Kind       Kind
}

func (o Order) kind() Kind {
	if o.Kind == 0 {
		return KindMarket
	}
	return o.Kind
}

func (o Order) validate() error {
	if o.Qty <= 0 {
		return fmt.Errorf("execution: qty must be positive, got %d", o.Qty)
	}
	if o.Side != core.SideBid && o.Side != core.SideAsk {
		return fmt.Errorf("execution: side %s is not bid or ask", o.Side)
	}
	if o.kind() != KindMarket {
		return fmt.Errorf("execution: %s orders are not implemented", o.kind())
	}
	return nil
}

// Status is the outcome delivered on OrderEvent.
type Status uint8

const (
	StatusRejected Status = iota + 1
	StatusFilled
	StatusCanceled
)

func (s Status) String() string {
	switch s {
	case StatusRejected:
		return "rejected"
	case StatusFilled:
		return "filled"
	case StatusCanceled:
		return "canceled"
	default:
		return "unknown"
	}
}

// OrderEvent is what the strategy sees after the venue settles.
// Fill is meaningful only when Status is StatusFilled.
type OrderEvent struct {
	Order  Order
	Status Status
	Fill   Fill
}
