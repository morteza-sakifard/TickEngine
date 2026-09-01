package indicators

import "github.com/morteza-sakifard/market-data-lab/internal/trade"

type Delta struct {
	buyVol  int64
	sellVol int64
}

func (d *Delta) Add(t trade.Trade) {
	switch t.Side {
	case trade.Buy:
		d.buyVol += int64(t.Size)
	case trade.Sell:
		d.sellVol += int64(t.Size)
	}
}

// Value returns the current Delta. Positive means buy-dominated,
// negative means sell-dominated.
func (d *Delta) Value() int64 {
	return d.buyVol - d.sellVol
}
