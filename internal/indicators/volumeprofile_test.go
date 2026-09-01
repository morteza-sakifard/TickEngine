package indicators

import (
	"testing"

	"github.com/morteza-sakifard/market-data-lab/internal/trade"
)

func TestVolumeProfile(t *testing.T) {
	var vp VolumeProfile
	if got := vp.Levels(); len(got) != 0 {
		t.Errorf("Levels() with no trades = %v, want empty", got)
	}
	if got := vp.POC(); got != (PriceLevel{}) {
		t.Errorf("POC() with no trades = %+v, want zero value", got)
	}

	vp.Add(trade.Trade{Price: 100.00, Size: 5, Side: trade.Buy})
	vp.Add(trade.Trade{Price: 100.25, Size: 3, Side: trade.Sell})
	vp.Add(trade.Trade{Price: 100.00, Size: 2, Side: trade.Sell})

	want := []PriceLevel{
		{Price: 100.00, Volume: 7},
		{Price: 100.25, Volume: 3},
	}

	got := vp.Levels()
	if len(got) != len(want) {
		t.Fatalf("len(Levels()) = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("Levels()[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}

	if poc := vp.POC(); poc != want[0] {
		t.Errorf("POC() = %+v, want %+v", poc, want[0])
	}
}
