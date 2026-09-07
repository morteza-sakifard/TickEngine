package session

import (
	"sort"
	"time"
)

// Boundary is one session transition. At is the instant the session
// changes; From is Classify(At - 1ns) and To is Classify(At).
// TradingDate is the trading date of the session being entered (To),
// or the zero Time when To is Closed — the same convention Classify
// already uses. See docs/00-architecture.md L3.
//
// The orchestrator built in step 10 resets accumulators on these
// edges: a VWAP does not care whether the edge was "RTH ended" or
// "ETH began", only that the regime changed.
type Boundary struct {
	At          time.Time
	TradingDate time.Time
	From, To    Session
}

// Boundaries returns every session transition in [from, to) in time
// order, with no duplicates. An empty or inverted range returns nil.
//
// Cut points are the open/close/halt instants of every nearby trading
// date — not a minute-by-minute walk. Classify is half-open
// ([RTHOpen, RTHClose), [ETHOpen, ETHClose), halt first), so the
// instant of a cut is already the new session; one nanosecond before
// it is the old one. DST is handled by time.Date in c.Location: the
// cut clock-times (17:00, 08:30, 12:15, 15:00, 15:15, 15:30, 16:00)
// never land inside the skipped or repeated 02:00 hour.
func (c Calendar) Boundaries(from, to time.Time) []Boundary {
	if !from.Before(to) {
		return nil
	}
	from = from.In(c.Location)
	to = to.In(c.Location)

	start := civilMidnight(from).AddDate(0, 0, -2)
	end := civilMidnight(to).AddDate(0, 0, 2)

	var cuts []time.Time
	for day := start; !day.After(end); day = day.AddDate(0, 0, 1) {
		h := c.Schedule.HoursFor(day)
		if h.Closed {
			continue
		}
		cuts = append(cuts, h.ETHOpen, h.RTHOpen, h.RTHClose, h.ETHClose)
		if !h.HaltOpen.IsZero() {
			cuts = append(cuts, h.HaltOpen, h.HaltClose)
		}
	}
	sort.Slice(cuts, func(i, j int) bool { return cuts[i].Before(cuts[j]) })

	var out []Boundary
	var prev time.Time
	for i, at := range cuts {
		if i > 0 && at.Equal(prev) {
			continue
		}
		prev = at
		if at.Before(from) || !at.Before(to) {
			continue
		}
		prevTD, prevSess := c.Classify(at.Add(-time.Nanosecond))
		nextTD, nextSess := c.Classify(at)
		if prevSess == nextSess && prevTD.Equal(nextTD) {
			continue
		}
		out = append(out, Boundary{
			At:          at,
			TradingDate: nextTD,
			From:        prevSess,
			To:          nextSess,
		})
	}
	return out
}
