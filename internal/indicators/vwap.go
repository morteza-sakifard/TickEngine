package indicators

import "github.com/morteza-sakifard/market-data-lab/internal/trade"

// VWAP computes the volume-weighted average price:
// VWAP = Σ(Price × Size) / Σ(Size)
type VWAP struct {
	sumPV float64
	sumV  float64
}

// Add incorporates one trade into the running VWAP.
func (v *VWAP) Add(t trade.Trade) {
	v.sumPV += t.Price * float64(t.Size)
	v.sumV += float64(t.Size)
}

// Value returns the current VWAP, or 0 if no trades have been added.
func (v *VWAP) Value() float64 {
	if v.sumV == 0 {
		return 0
	}
	return v.sumPV / v.sumV
}
