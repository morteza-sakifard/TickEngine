package indicators

import (
	"testing"

	"github.com/morteza-sakifard/market-data-lab/internal/trade"
)

func TestPipeline(t *testing.T) {
	var p Pipeline
	p.Add(trade.Trade{Price: 100.00, Size: 5, Side: trade.Buy})
	p.Add(trade.Trade{Price: 100.00, Size: 2, Side: trade.Sell})

	if got := p.VWAP.Value(); got != 100.00 {
		t.Errorf("VWAP.Value() = %v, want 100.00", got)
	}
	if got := p.Delta.Value(); got != 3 {
		t.Errorf("Delta.Value() = %d, want 3", got)
	}
	if got := p.CVD.Value(); got != 3 {
		t.Errorf("CVD.Value() = %d, want 3", got)
	}
	if got := len(p.VolumeProfile.Levels()); got != 1 {
		t.Errorf("len(VolumeProfile.Levels()) = %d, want 1", got)
	}
	if got := len(p.Footprint.Levels()); got != 1 {
		t.Errorf("len(Footprint.Levels()) = %d, want 1", got)
	}
	if got := len(p.TPO.Levels()); got != 1 {
		t.Errorf("len(TPO.Levels()) = %d, want 1", got)
	}
}
