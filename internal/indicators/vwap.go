package indicators

import "github.com/morteza-sakifard/market-data-lab/internal/trade"

type VWAP struct {
	sumPV float64
	sumV  float64
}

func (v *VWAP) Add(t trade.Trade) {
	v.sumPV += t.Price * float64(t.Size)
	v.sumV += float64(t.Size)
}

func (v *VWAP) Value() float64 {
	if v.sumV == 0 {
		return 0
	}
	return v.sumPV / v.sumV
}
