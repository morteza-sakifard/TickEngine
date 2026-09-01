package indicators

import (
	"sort"
	"time"

	"github.com/morteza-sakifard/market-data-lab/internal/trade"
)

const TPOPeriod = 30 * time.Minute

type TPOLevel struct {
	Price   float64
	Periods []int
}

func (l TPOLevel) Count() int {
	return len(l.Periods)
}

type TPO struct {
	start time.Time
	seen  map[float64]map[int]bool
}

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

func (tpo *TPO) POC() TPOLevel {
	var poc TPOLevel
	for _, lvl := range tpo.Levels() {
		if lvl.Count() > poc.Count() {
			poc = lvl
		}
	}
	return poc
}
