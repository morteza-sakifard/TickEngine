package orderflow

import (
	"github.com/morteza-sakifard/market-data-lab/internal/core"
	"github.com/morteza-sakifard/market-data-lab/internal/marketdata"
)

// VWAP is Σ(price×qty) / Σ(qty) in Ticks. Integer division truncates
// toward zero: a price, not a statistical metric, so it stays off
// float64 (docs/02-conventions.md 2.5). Empty input is 0.
type VWAP struct {
	sumPV int64
	sumV  int64
}

func (v *VWAP) OnTrade(ev *marketdata.Event) {
	if ev == nil || ev.Kind != marketdata.KindTrade {
		return
	}
	qty := int64(ev.Trade.Qty)
	if qty <= 0 {
		return
	}
	v.sumPV += int64(ev.Trade.Px) * qty
	v.sumV += qty
}

func (v *VWAP) Value() core.Ticks {
	if v.sumV == 0 {
		return 0
	}
	return core.Ticks(v.sumPV / v.sumV)
}

func (v *VWAP) Reset() { *v = VWAP{} }
