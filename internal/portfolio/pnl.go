package portfolio

import (
	"github.com/morteza-sakifard/market-data-lab/internal/core"
	"github.com/morteza-sakifard/market-data-lab/internal/marketdata"
)

// CentsPerTick is the dollar value of one tick, in integer cents.
// ES: $50 per point * 0.25 point * 100 cents = 1250. The acceptance
// test "buy X, sell X+1 tick" is this number minus fees. TickSizeNano
// must divide 10_000_000 or the cents truncate; ES does.
func CentsPerTick(inst core.Instrument) int64 {
	if inst.TickSizeNano == 0 {
		return 0
	}
	return inst.Multiplier * inst.TickSizeNano / 10_000_000
}

// Unrealized marks a long at BidPx and a short at AskPx — the
// prices a market exit would get. Mid would hide the spread the
// venue just charged you.
func Unrealized(p Position, q marketdata.Quote, inst core.Instrument) int64 {
	if p.Qty == 0 {
		return 0
	}
	cpt := CentsPerTick(inst)
	if p.Qty > 0 {
		return int64(q.BidPx-p.AvgPx) * int64(p.Qty) * cpt
	}
	return int64(p.AvgPx-q.AskPx) * int64(-p.Qty) * cpt
}

func Total(p Position, q marketdata.Quote, inst core.Instrument) int64 {
	return p.Realized + Unrealized(p, q, inst)
}
