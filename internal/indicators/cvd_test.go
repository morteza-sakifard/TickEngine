package indicators

import (
	"testing"
	"time"

	"github.com/morteza-sakifard/market-data-lab/internal/trade"
)

func TestCVD(t *testing.T) {
	var c CVD
	if got := c.Value(); got != 0 {
		t.Errorf("Value() with no trades = %v, want 0", got)
	}
	if got := c.Series(); len(got) != 0 {
		t.Errorf("Series() with no trades = %v, want empty", got)
	}

	t1 := time.Date(2025, 9, 22, 0, 0, 0, 0, time.UTC)
	t2 := t1.Add(time.Second)
	t3 := t2.Add(time.Second)
	c.Add(trade.Trade{Time: t1, Side: trade.Buy, Size: 5})
	c.Add(trade.Trade{Time: t2, Side: trade.Sell, Size: 2})
	c.Add(trade.Trade{Time: t3, Side: trade.Buy, Size: 1})
	want := []CVDPoint{
		{Time: t1, Value: 5},
		{Time: t2, Value: 3},
		{Time: t3, Value: 4},
	}
	got := c.Series()
	if len(got) != len(want) {
		t.Fatalf("len(Series()) = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if !got[i].Time.Equal(want[i].Time) || got[i].Value != want[i].Value {
			t.Errorf("Series()[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}
	if got := c.Value(); got != 4 {
		t.Errorf("Value() = %d, want 4", got)
	}
}
