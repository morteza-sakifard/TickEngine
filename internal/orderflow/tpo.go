package orderflow

import (
	"sort"
	"time"

	"github.com/morteza-sakifard/TickEngine/internal/core"
	"github.com/morteza-sakifard/TickEngine/internal/marketdata"
)

// TPOPeriod is one market-profile letter. RTH 08:30–15:00 is 13 of
// these (A–M). The clock is the session open, not the first trade:
// a file that starts at 10:37 still labels that print E, not A.
const TPOPeriod = 30 * time.Minute

// TPOLevel is one price row: the distinct period indices that traded
// there, sorted. Count is len(Periods).
type TPOLevel struct {
	Price   core.Ticks
	Periods []int
}

func (l TPOLevel) Count() int { return len(l.Periods) }

// TPO counts time-at-price. The anchor is injected; Reset clears
// prints but not the open. A zero anchor ignores every trade — it
// must not fall back to the first print (the tpo.go:40 bug).
type TPO struct {
	anchor time.Time
	period time.Duration
	seen   map[core.Ticks]map[int]struct{}
}

func NewTPO(anchor time.Time) *TPO {
	return &TPO{anchor: anchor, period: TPOPeriod}
}

func (t *TPO) OnTrade(ev *marketdata.Event) {
	if t == nil || ev == nil || ev.Kind != marketdata.KindTrade {
		return
	}
	if t.anchor.IsZero() || t.period <= 0 {
		return
	}
	if ev.Trade.Qty <= 0 {
		return
	}
	p := periodIndex(ev.EventTime(), t.anchor, t.period)
	if p < 0 {
		return
	}
	if t.seen == nil {
		t.seen = make(map[core.Ticks]map[int]struct{})
	}
	row := t.seen[ev.Trade.Px]
	if row == nil {
		row = make(map[int]struct{})
		t.seen[ev.Trade.Px] = row
	}
	row[p] = struct{}{}
}

func (t *TPO) Reset() {
	if t == nil {
		return
	}
	t.seen = nil
}

func periodIndex(at, anchor time.Time, d time.Duration) int {
	if at.Before(anchor) {
		return -1
	}
	return int(at.Sub(anchor) / d)
}

// PeriodLetter is period 0 = A … 25 = Z, 26 = a. RTH never needs
// more than M (12).
func PeriodLetter(i int) string {
	switch {
	case i < 0:
		return ""
	case i < 26:
		return string(rune('A' + i))
	case i < 52:
		return string(rune('a' + (i - 26)))
	default:
		return "?"
	}
}

func (t *TPO) Levels() []TPOLevel {
	if t == nil || len(t.seen) == 0 {
		return nil
	}
	out := make([]TPOLevel, 0, len(t.seen))
	for px, periods := range t.seen {
		list := make([]int, 0, len(periods))
		for p := range periods {
			list = append(list, p)
		}
		sort.Ints(list)
		out = append(out, TPOLevel{Price: px, Periods: list})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Price < out[j].Price })
	return out
}

func (t *TPO) POC() core.Ticks {
	levels := t.Levels()
	if len(levels) == 0 {
		return 0
	}
	best := 0
	for i := 1; i < len(levels); i++ {
		if levels[i].Count() > levels[best].Count() {
			best = i
		}
	}
	return levels[best].Price
}

// InitialBalance is the high/low of periods 0 and 1 (A and B).
func (t *TPO) InitialBalance() (low, high core.Ticks, ok bool) {
	for _, lv := range t.Levels() {
		for _, p := range lv.Periods {
			if p != 0 && p != 1 {
				continue
			}
			if !ok || lv.Price < low {
				low = lv.Price
			}
			if !ok || lv.Price > high {
				high = lv.Price
			}
			ok = true
			break
		}
	}
	return low, high, ok
}

// SinglePrints returns rows that traded in exactly one period.
func (t *TPO) SinglePrints() []TPOLevel {
	var out []TPOLevel
	for _, lv := range t.Levels() {
		if lv.Count() == 1 {
			out = append(out, lv)
		}
	}
	return out
}

// PeriodCount is how many distinct 30-minute letters printed.
// A full ES RTH day is 13 (A–M). 12 or 14 means the anchor is wrong.
func (t *TPO) PeriodCount() int {
	seen := make(map[int]struct{})
	for _, lv := range t.Levels() {
		for _, p := range lv.Periods {
			seen[p] = struct{}{}
		}
	}
	return len(seen)
}
