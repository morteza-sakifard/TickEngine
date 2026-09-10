package main

import (
	"strings"
	"testing"

	"github.com/morteza-sakifard/TickEngine/internal/core"
	"github.com/morteza-sakifard/TickEngine/internal/feed/validate"
)

func TestRunAggressorCheckOnFixture(t *testing.T) {
	agg, err := runAggressorCheck(fixturePath, core.ESZ5(), 0, "mbp-1")
	if err != nil {
		t.Fatal(err)
	}

	// TestCensusOnFixture already pins the fixture at 3 action=T rows:
	// row 3 (side=B @ ask, consistent), row 4 (side=A @ bid,
	// consistent), row 7 (side=N, not judged).
	if got, want := agg.Total(), int64(3); got != want {
		t.Errorf("Total() = %d, want %d", got, want)
	}
	if got, want := agg.Judged(), int64(2); got != want {
		t.Errorf("Judged() = %d, want %d", got, want)
	}
	if got, want := agg.ConsistentRate(), 1.0; got != want {
		t.Errorf("ConsistentRate() = %v, want %v", got, want)
	}
	if got := agg.Count(validate.CategoryBuyerAtAsk); got != 1 {
		t.Errorf("CategoryBuyerAtAsk = %d, want 1", got)
	}
	if got := agg.Count(validate.CategorySellerAtBid); got != 1 {
		t.Errorf("CategorySellerAtBid = %d, want 1", got)
	}
	if got := agg.Count(validate.CategoryNoAggressor); got != 1 {
		t.Errorf("CategoryNoAggressor = %d, want 1 (row 7 is side=N)", got)
	}
}

func TestReportAggressorDoesNotPanic(t *testing.T) {
	agg, err := runAggressorCheck(fixturePath, core.ESZ5(), 0, "mbp-1")
	if err != nil {
		t.Fatal(err)
	}
	var buf strings.Builder
	reportAggressor(&buf, agg)
	for _, want := range []string{"aggressor-side validation", "consistency rate", "100.00%"} {
		if !strings.Contains(buf.String(), want) {
			t.Errorf("report is missing %q", want)
		}
	}
}
