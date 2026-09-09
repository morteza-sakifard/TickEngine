package main

import (
	"fmt"
	"io"
	"sort"
	"time"
)

func (c *Census) Report(w io.Writer) {
	line := func() {
		fmt.Fprintln(w, "-----------------------------------------------------------")
	}

	fmt.Fprintln(w, "===========================================================")
	if c.Schema == "" {
		fmt.Fprintln(w, " dataset census")
	} else {
		fmt.Fprintf(w, " %s dataset census\n", c.Schema)
	}
	fmt.Fprintln(w, "===========================================================")
	fmt.Fprintf(w, " %-26s %d\n", "records", c.Records)
	if c.LimitReached {
		fmt.Fprintf(w, " %-26s %s\n", "limit", "REACHED - numbers below are partial")
	}

	line()
	fmt.Fprintln(w, " columns")
	for i, name := range c.Header {
		fmt.Fprintf(w, "   %2d  %s\n", i, name)
	}

	line()
	fmt.Fprintln(w, " columns expected to be constant")
	consts := []struct {
		name string
		m    map[string]int64
	}{
		{"rtype", c.RType},
		{"publisher_id", c.Publisher},
	}
	if c.Schema != "mbo" {
		consts = append(consts, struct {
			name string
			m    map[string]int64
		}{"depth", c.Depth})
	}
	for _, e := range consts {
		fmt.Fprintf(w, "   %s\n", e.name)
		printCounts(w, e.m, nil)
		if len(e.m) > 1 {
			fmt.Fprintf(w, "     ^ %d distinct values - an assumption is wrong\n", len(e.m))
		}
	}

	line()
	fmt.Fprintln(w, " symbols")
	syms := make([]string, 0, len(c.Symbols))
	for k := range c.Symbols {
		syms = append(syms, k)
	}
	sort.Strings(syms)
	for _, k := range syms {
		s := c.Symbols[k]
		fmt.Fprintf(w, "   %-10s instrument_id=%-8s records=%-12d %s .. %s\n",
			s.Symbol, s.InstrumentID, s.Records, ts(s.First), ts(s.Last))
	}
	if len(syms) > 1 {
		fmt.Fprintln(w, "   ^ more than one symbol: this dataset spans a contract roll")
	}

	line()
	fmt.Fprintln(w, " timestamps (UTC)")
	reportTs(w, "ts_recv", c.TsRecv)
	reportTs(w, "ts_event", c.TsEvent)

	line()
	fmt.Fprintln(w, " latency")
	reportHist(w, "ts_recv - ts_event", c.RecvMinusEvent, asDuration)
	fmt.Fprintf(w, "     %-22s %d\n", "recv before event", c.RecvBeforeEvent)
	reportHist(w, "ts_in_delta", c.InDelta, asDuration)
	fmt.Fprintf(w, "     %-22s %d\n", "ts_in_delta == 0", c.InDeltaZero)
	fmt.Fprintf(w, "     %-22s %d\n", "parse failures", c.InDeltaFail)

	line()
	fmt.Fprintln(w, " action")
	printCounts(w, c.Actions, actionOrder)
	fmt.Fprintln(w, " side")
	printCounts(w, c.Sides, sideOrder)
	fmt.Fprintln(w, " action x side")
	printCounts(w, c.ActionSide, nil)

	line()
	fmt.Fprintln(w, " flags - per bit")
	names := make([]string, 0, len(flagTable))
	for _, f := range flagTable {
		names = append(names, f.name)
	}
	printCounts(w, c.FlagBits, names)
	fmt.Fprintf(w, "   %-24s %d\n", "unknown bits set", c.FlagUnknown)
	fmt.Fprintf(w, "   %-24s %d\n", "parse failures", c.FlagFail)

	fmt.Fprintln(w, " flags - raw combinations")
	raws := make([]int, 0, len(c.FlagRaw))
	for k := range c.FlagRaw {
		raws = append(raws, int(k))
	}
	sort.Ints(raws)
	for _, r := range raws {
		raw := uint8(r)
		fmt.Fprintf(w, "     %3d  0x%02X  %-42v %d\n",
			raw, raw, flagNames(raw), c.FlagRaw[raw])
	}

	line()
	fmt.Fprintln(w, " price - all records")
	reportPrice(w, c.Price)
	fmt.Fprintln(w, " price - trades only")
	reportPrice(w, c.TradePrice)

	line()
	fmt.Fprintln(w, " quote (top of book)")
	for _, e := range []struct {
		name string
		n    int64
	}{
		{"both sides present", c.QuoteBoth},
		{"bid only", c.QuoteBidOnly},
		{"ask only", c.QuoteAskOnly},
		{"neither", c.QuoteNeither},
		{"bid_px undefined", c.UndefBidPx},
		{"ask_px undefined", c.UndefAskPx},
		{"locked (bid == ask)", c.LockedBook},
		{"crossed (bid > ask)", c.CrossedBook},
	} {
		fmt.Fprintf(w, "   %-24s %d\n", e.name, e.n)
	}
	reportHist(w, "spread in ticks", c.SpreadTicks, asTicks)

	line()
	fmt.Fprintln(w, " trades (action = T)")
	fmt.Fprintf(w, "   %-24s %d\n", "count", c.Trades)
	for _, s := range sideOrder {
		fmt.Fprintf(w, "     side=%-3s trades=%-12d volume=%d\n",
			s, c.TradeSideCount[s], c.TradeSideVolume[s])
	}
	for _, e := range []struct {
		name string
		n    int64
	}{
		{"min size", c.TradeSizeMin},
		{"max size", c.TradeSizeMax},
		{"size undefined", c.SizeUndef},
		{"size parse failures", c.SizeFail},
	} {
		fmt.Fprintf(w, "   %-24s %d\n", e.name, e.n)
	}

	line()
	fmt.Fprintf(w, " calendar days in %s\n", c.opt.Loc)
	c.reportDays(w)
	fmt.Fprintln(w, "===========================================================")
}

func (c *Census) reportDays(w io.Writer) {
	days := c.SortedDays()
	if len(days) == 0 {
		fmt.Fprintln(w, "   (none)")
		return
	}

	median := medianVolume(c.Days)
	prev, _ := time.Parse("2006-01-02", days[0])

	for i, key := range days {
		d, _ := time.Parse("2006-01-02", key)

		// Report gaps: holidays and weekends live here. This list is the
		// input to the calendar work in step 6.
		if i > 0 {
			for g := prev.AddDate(0, 0, 1); g.Before(d); g = g.AddDate(0, 0, 1) {
				note := "MISSING - candidate holiday"
				if wd := g.Weekday(); wd == time.Saturday || wd == time.Sunday {
					note = "missing (weekend)"
				}
				fmt.Fprintf(w, "   %s  %s\n", g.Format("2006-01-02"), note)
			}
		}
		prev = d

		s := c.Days[key]
		note := ""
		if median > 0 && s.Volume*10 < median*4 {
			note = "  <- low volume, check for an early close"
		}
		fmt.Fprintf(w, "   %s  records=%-10d trades=%-9d volume=%-10d %s..%s%s\n",
			s.Date, s.Records, s.Trades, s.Volume,
			s.First.In(c.opt.Loc).Format("15:04:05"),
			s.Last.In(c.opt.Loc).Format("15:04:05"),
			note)
	}
}

func medianVolume(days map[string]*dayStat) int64 {
	if len(days) == 0 {
		return 0
	}
	vols := make([]int64, 0, len(days))
	for _, d := range days {
		vols = append(vols, d.Volume)
	}
	sort.Slice(vols, func(i, j int) bool { return vols[i] < vols[j] })
	return vols[len(vols)/2]
}

func reportTs(w io.Writer, name string, s tsStat) {
	fmt.Fprintf(w, "   %s\n", name)
	fmt.Fprintf(w, "     %-22s %s\n", "min", ts(s.Min))
	fmt.Fprintf(w, "     %-22s %s\n", "max", ts(s.Max))
	fmt.Fprintf(w, "     %-22s %s\n", "span", s.Max.Sub(s.Min))
	fmt.Fprintf(w, "     %-22s %d\n", "backward jumps", s.Decreases)
	fmt.Fprintf(w, "     %-22s %s\n", "largest jump back", time.Duration(s.MaxBackNs))
	fmt.Fprintf(w, "     %-22s %d\n", "parse failures", s.Fail)
}

func reportPrice(w io.Writer, p priceStat) {
	fmt.Fprintf(w, "   %-24s %s\n", "min", formatNano(p.MinNano))
	fmt.Fprintf(w, "   %-24s %s\n", "max", formatNano(p.MaxNano))
	fmt.Fprintf(w, "   %-24s %d\n", "off-tick", p.OffTick)
	fmt.Fprintf(w, "   %-24s %d\n", "undefined (sentinel)", p.Undef)
	fmt.Fprintf(w, "   %-24s %d\n", "parse failures", p.Fail)
}

func reportHist(w io.Writer, name string, h *Hist, render func(int64) string) {
	fmt.Fprintf(w, "   %s  (n=%d)\n", name, h.Count())
	if h.Count() == 0 {
		return
	}
	for _, q := range []struct {
		label string
		p     float64
	}{
		{"p50", 0.50}, {"p90", 0.90}, {"p99", 0.99}, {"p99.9", 0.999},
	} {
		fmt.Fprintf(w, "     %-22s %s\n", q.label, render(h.Percentile(q.p)))
	}
	fmt.Fprintf(w, "     %-22s %s\n", "min (exact)", render(h.Min()))
	fmt.Fprintf(w, "     %-22s %s\n", "max (exact)", render(h.Max()))
	fmt.Fprintf(w, "     %-22s %d\n", "negative", h.Under())
	fmt.Fprintf(w, "     %-22s %d\n", "overflow", h.Overflow())
	if h.Overflow()*10 > h.Count() {
		fmt.Fprintln(w, "     ^ over 10% overflow: the bucket width is too small,")
		fmt.Fprintln(w, "       so the percentiles above are not meaningful")
	}
}

// printCounts prints a string-keyed counter map. Keys in order come first,
// in that order; anything else is sorted and marked, so an unexpected
// value cannot hide.
func printCounts(w io.Writer, m map[string]int64, order []string) {
	seen := make(map[string]bool, len(order))
	for _, k := range order {
		if n, ok := m[k]; ok {
			fmt.Fprintf(w, "     %-16s %d\n", k, n)
		}
		seen[k] = true
	}
	rest := make([]string, 0, len(m))
	for k := range m {
		if !seen[k] {
			rest = append(rest, k)
		}
	}
	sort.Strings(rest)
	for _, k := range rest {
		mark := ""
		if len(order) > 0 {
			mark = "   <- unexpected value"
		}
		fmt.Fprintf(w, "     %-16s %d%s\n", k, m[k], mark)
	}
}

func ts(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	return t.UTC().Format("2006-01-02T15:04:05.000000000Z")
}

func asDuration(ns int64) string { return time.Duration(ns).String() }
func asTicks(t int64) string     { return fmt.Sprintf("%d", t) }
