package execution

import (
	"fmt"

	"github.com/morteza-sakifard/market-data-lab/internal/core"
)

// Fill is an execution against the top of the book. A market buy
// is priced at AskPx, a market sell at BidPx — never the last
// trade. The last trade is someone else's print; your market order
// crosses the spread. On ES that gift is $12.50 per contract if
// you fill at the trade instead. FeeCents is commission plus
// exchange fee for this fill, already multiplied by Qty.
type Fill struct {
	OrderID  OrderID
	Ts       int64
	Px       core.Ticks
	Qty      core.Qty
	Side     core.Side
	FeeCents int64
}

// Fees are integer USD cents per contract per fill. A $2.25 ES
// round-turn is about 112 + 113 or 100 + 12 twice; pick numbers
// that sum to the cost you want. Negative values are rejected.
type Fees struct {
	CommissionCents int64
	FeeCents        int64
}

func (f Fees) perContract() int64 {
	return f.CommissionCents + f.FeeCents
}

func (f Fees) validate() error {
	if f.CommissionCents < 0 || f.FeeCents < 0 {
		return fmt.Errorf("execution: fees must be >= 0")
	}
	return nil
}
