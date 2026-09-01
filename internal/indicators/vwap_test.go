package indicators

import (
	"testing"

	"github.com/morteza-sakifard/market-data-lab/internal/trade"
)

func TestVWAP(t *testing.T) {
	var v VWAP
	if got := v.Value(); got != 0 {
		t.Errorf("Value() with no trades = %v, want 0", got)
	}

	v.Add(trade.Trade{Price: 100, Size: 2})
	v.Add(trade.Trade{Price: 101, Size: 1})

	want := (100.0*2 + 101.0*1) / 3.0
	if got := v.Value(); got != want {
		t.Errorf("Value() = %v, want %v", got, want)
	}
}
