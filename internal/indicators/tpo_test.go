package indicators

import (
	"testing"
	"time"

	"github.com/morteza-sakifard/market-data-lab/internal/trade"
)

func TestTPO(t *testing.T) {
	var tpo TPO
	base := time.Date(2025, 9, 22, 0, 0, 0, 0, time.UTC)
	tpo.Add(trade.Trade{Time: base, Price: 100.00, Side: trade.Buy, Size: 1})                       // period 0
	tpo.Add(trade.Trade{Time: base.Add(35 * time.Minute), Price: 100.00, Side: trade.Buy, Size: 1}) // period 1
	tpo.Add(trade.Trade{Time: base.Add(65 * time.Minute), Price: 100.00, Side: trade.Buy, Size: 1}) // period 2
	tpo.Add(trade.Trade{Time: base, Price: 100.25, Side: trade.Sell, Size: 1})                      // period 0

	want := []TPOLevel{
		{Price: 100.00, Periods: []int{0, 1, 2}},
		{Price: 100.25, Periods: []int{0}},
	}
	got := tpo.Levels()
	if len(got) != len(want) {
		t.Fatalf("len(Levels()) = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i].Price != want[i].Price || !equalInts(got[i].Periods, want[i].Periods) {
			t.Errorf("Levels()[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}
	if poc := tpo.POC(); poc.Price != 100.00 || poc.Count() != 3 {
		t.Errorf("POC() = %+v, want price 100.00 count 3", poc)
	}
}
func equalInts(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
