package main

import (
	"fmt"
	"io"
	"os"

	"github.com/morteza-sakifard/market-data-lab/internal/core"
	"github.com/morteza-sakifard/market-data-lab/internal/feed/databento"
	"github.com/morteza-sakifard/market-data-lab/internal/feed/validate"
	"github.com/morteza-sakifard/market-data-lab/internal/marketdata"
)

// runAggressorCheck re-reads path through feed/databento -- the decoder
// step 4 built -- and feeds every event to a fresh validate.Aggressor.
// It is a second, independent pass over the file: Run/Census above
// predate core.Ticks and marketdata.Event (they are step 1's own
// parsing), and per docs/00-architecture.md 5.8 this project adds
// columnar storage or merges the two passes only after profiling says
// parsing is the bottleneck, not before. limit bounds the number of
// *events* read, a looser bound than Census's own record-count --limit
// since one CSV row can produce two events; that only matters for
// quick manual testing, not for the real run (limit 0), which is the
// one step 5's 95% acceptance criterion cares about.
func runAggressorCheck(path string, inst core.Instrument, limit int64, schema string) (*validate.Aggressor, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	sch, err := databento.ParseSchema(schema)
	if err != nil {
		return nil, err
	}
	d, err := databento.Open(f, inst, sch)
	if err != nil {
		return nil, err
	}
	defer d.Close()

	agg := validate.NewAggressor()
	var evt marketdata.Event
	var n int64
	for {
		if err := d.Next(&evt); err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		agg.Observe(&evt)
		n++
		if limit > 0 && n >= limit {
			break
		}
	}
	return agg, nil
}

// aggressorCategoryOrder fixes the report's print order: the two
// consistent outcomes first, then the three judged-but-not-consistent
// outcomes, then the three that Judged excludes entirely. Mirrors how
// actionOrder and sideOrder in census.go fix printCounts's order.
var aggressorCategoryOrder = []validate.Category{
	validate.CategoryBuyerAtAsk,
	validate.CategorySellerAtBid,
	validate.CategoryInsideSpread,
	validate.CategoryOutsideSpread,
	validate.CategoryContradictory,
	validate.CategoryNoAggressor,
	validate.CategoryNoQuote,
	validate.CategoryOneSidedBook,
}

// reportAggressor prints the step 5 aggressor-side breakdown: a count
// per category in aggressorCategoryOrder, then the consistency rate
// docs/01-roadmap.md step 5 requires to be above 95% before step 6.
func reportAggressor(w io.Writer, agg *validate.Aggressor) {
	fmt.Fprintln(w, "===========================================================")
	fmt.Fprintln(w, " aggressor-side validation (docs/01-roadmap.md step 5)")
	fmt.Fprintln(w, "===========================================================")
	fmt.Fprintf(w, " %-26s %d\n", "trades observed", agg.Total())
	for _, c := range aggressorCategoryOrder {
		fmt.Fprintf(w, "   %-28s %d\n", c, agg.Count(c))
	}
	rate := agg.ConsistentRate()
	fmt.Fprintf(w, " %-26s %.2f%% (of %d judged)\n", "consistency rate", rate*100, agg.Judged())
	if agg.Judged() > 0 && rate < 0.95 {
		fmt.Fprintln(w, " ^ below 95% -- stop before step 6; the aggressor-side assumption or the data is wrong")
	}
	fmt.Fprintln(w, "===========================================================")
}
