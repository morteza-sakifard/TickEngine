package indicators

import (
	"testing"

	"github.com/morteza-sakifard/market-data-lab/internal/trade"
)

func TestFootprint(t *testing.T) {
	var fp Footprint
	if got := fp.Levels(); len(got) != 0 {
		t.Errorf("Levels() with no trades = %v, want empty", got)
	}

	fp.Add(trade.Trade{Price: 100.00, Size: 5, Side: trade.Buy})
	fp.Add(trade.Trade{Price: 100.25, Size: 3, Side: trade.Sell})
	fp.Add(trade.Trade{Price: 100.00, Size: 2, Side: trade.Sell})
	fp.Add(trade.Trade{Price: 100.00, Size: 1, Side: trade.Buy})

	want := []FootprintLevel{
		{Price: 100.00, BuyVolume: 6, SellVolume: 2},
		{Price: 100.25, BuyVolume: 0, SellVolume: 3},
	}
	got := fp.Levels()
	if len(got) != len(want) {
		t.Fatalf("len(Levels()) = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("Levels()[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}
	if got := got[0].Delta(); got != 4 {
		t.Errorf("Delta() = %d, want 4", got)
	}
}
