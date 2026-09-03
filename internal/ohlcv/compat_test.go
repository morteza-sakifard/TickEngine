package ohlcv_test

import (
	"testing"
	"time"

	"github.com/morteza-sakifard/market-data-lab/internal/indicators"
	"github.com/morteza-sakifard/market-data-lab/internal/ohlcv"
	"github.com/morteza-sakifard/market-data-lab/internal/session"
	"github.com/morteza-sakifard/market-data-lab/internal/trade"
)

// Confirms the existing indicators.Pipeline still works unchanged now that
// the same trades also feed the new ohlcv.Builder: VWAP, Delta, CVD, Volume
// Profile, Footprint, and TPO all match their pre-existing expectations.
func TestExistingAnalyticsStillWork(t *testing.T) {
	loc, err := time.LoadLocation("America/Chicago")
	if err != nil {
		t.Fatalf("LoadLocation: %v", err)
	}
	cal := session.Calendar{Location: loc, Schedule: session.ESRegularSchedule()}
	builder := ohlcv.Builder{Symbol: "ESZ5", Interval: 5 * time.Minute, Calendar: cal}

	trades := []trade.Trade{
		{Time: mustUTC(t, "2025-09-23T15:31:17Z"), Price: 6714.75, Size: 1, Side: trade.Buy},
		{Time: mustUTC(t, "2025-09-23T15:31:18Z"), Price: 6714.50, Size: 3, Side: trade.Sell},
	}

	var pipeline indicators.Pipeline
	for _, tr := range trades {
		pipeline.Add(tr)
		builder.Add(tr)
	}

	if got := pipeline.VWAP.Value(); got == 0 {
		t.Errorf("VWAP.Value() = %v, want nonzero", got)
	}
	if got := pipeline.Delta.Value(); got != -2 {
		t.Errorf("Delta.Value() = %d, want -2", got)
	}
	if got := pipeline.CVD.Value(); got != -2 {
		t.Errorf("CVD.Value() = %d, want -2", got)
	}
	if got := len(pipeline.VolumeProfile.Levels()); got != 2 {
		t.Errorf("len(VolumeProfile.Levels()) = %d, want 2", got)
	}
	if got := len(pipeline.Footprint.Levels()); got != 2 {
		t.Errorf("len(Footprint.Levels()) = %d, want 2", got)
	}
	if got := len(pipeline.TPO.Levels()); got != 2 {
		t.Errorf("len(TPO.Levels()) = %d, want 2", got)
	}
	if got := len(builder.Bars()); got != 1 {
		t.Errorf("len(builder.Bars()) = %d, want 1", got)
	}
}

func mustUTC(t *testing.T, s string) time.Time {
	t.Helper()
	parsed, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t.Fatalf("time.Parse(%q): %v", s, err)
	}
	return parsed
}
