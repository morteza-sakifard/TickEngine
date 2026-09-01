package indicators

import (
	"sort"
	"time"

	"github.com/morteza-sakifard/market-data-lab/internal/trade"
)

// TPOPeriod is the length of one Time Price Opportunity period.
// Traditional market profile charts use 30 minutes and label periods as
// letters (period 0 = 'A', 1 = 'B', ...); this only tracks the index.
const TPOPeriod = 30 * time.Minute

// TPOLevel is one price row of a TPO chart: the distinct period indices
// during which that price traded.
type TPOLevel struct {
	Price   float64
	Periods []int
}

// Count returns the number of distinct periods that traded at this price —
// the height of this row in a market profile chart.
func (l TPOLevel) Count() int {
	return len(l.Periods)
}

// TPO builds a Time Price Opportunity chart: it buckets trades into
// fixed-length periods measured from the first trade seen, and for each
// price records which periods traded there.
type TPO struct {
	start time.Time
	seen  map[float64]map[int]bool
}

// Add incorporates one trade into the chart.
func (tpo *TPO) Add(t trade.Trade) {
	if tpo.seen == nil {
		tpo.seen = make(map[float64]map[int]bool)
		tpo.start = t.Time
	}
	period := int(t.Time.Sub(tpo.start) / TPOPeriod)
	periods, ok := tpo.seen[t.Price]
	if !ok {
		periods = make(map[int]bool)
		tpo.seen[t.Price] = periods
	}
	periods[period] = true
}

// Levels returns the chart as a slice sorted by ascending price, with each
// price's periods sorted ascending.
func (tpo *TPO) Levels() []TPOLevel {
	levels := make([]TPOLevel, 0, len(tpo.seen))
	for price, periods := range tpo.seen {
		list := make([]int, 0, len(periods))
		for p := range periods {
			list = append(list, p)
		}
		sort.Ints(list)
		levels = append(levels, TPOLevel{Price: price, Periods: list})
	}
	sort.Slice(levels, func(i, j int) bool { return levels[i].Price < levels[j].Price })
	return levels
}

// POC returns the price level with the highest TPO count (traded during
// the most distinct periods) — the approximate "fair value" center of a
// market profile.
func (tpo *TPO) POC() TPOLevel {
	var poc TPOLevel
	for _, lvl := range tpo.Levels() {
		if lvl.Count() > poc.Count() {
			poc = lvl
		}
	}
	return poc
}
