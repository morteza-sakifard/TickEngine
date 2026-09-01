package indicators

import (
	"testing"

	"github.com/morteza-sakifard/market-data-lab/internal/trade"
)

func TestDelta(t *testing.T) {
	var d Delta
	if got := d.Value(); got != 0 {
		t.Errorf("Value() with no trades = %v, want 0", got)
	}

	d.Add(trade.Trade{Side: trade.Buy, Size: 5})
	d.Add(trade.Trade{Side: trade.Sell, Size: 2})
	d.Add(trade.Trade{Side: trade.Buy, Size: 1})

	want := int64(5 + 1 - 2)
	if got := d.Value(); got != want {
		t.Errorf("Value() = %v, want %v", got, want)
	}
}
