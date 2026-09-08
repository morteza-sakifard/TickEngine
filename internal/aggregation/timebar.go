package aggregation

import (
	"time"

	"github.com/morteza-sakifard/market-data-lab/internal/marketdata"
	"github.com/morteza-sakifard/market-data-lab/internal/session"
)

type timeBuilder struct {
	spec BarSpec
	cal  session.Calendar
	sink
}

func (b *timeBuilder) Add(ev *marketdata.Event) {
	t, px, qty, side, ok := tradeOf(ev)
	if !ok {
		return
	}
	td, sess, ok := accept(b.cal, b.spec.Sessions, t)
	if !ok {
		return
	}
	start := bucketStart(b.spec, b.cal, t, td, sess)
	if b.open == nil || !b.open.Start.Equal(start) {
		b.Flush()
		labelTD, labelSess := b.cal.Classify(start)
		if labelTD.IsZero() {
			labelTD, labelSess = td, sess
		}
		b.open = &Bar{
			Start:       start,
			End:         start.Add(b.spec.Interval),
			TradingDate: labelTD,
			Session:     labelSess,
		}
	}
	b.open.apply(px, qty, side)
}

// floorDiv is integer division that floors toward -inf. Go's / truncates
// toward zero, which is wrong for ETH trades sitting before that day's
// RTH open: elapsed is negative and we still need the bucket that
// contains t, not the one toward zero.
func floorDiv(a, b int64) int64 {
	q := a / b
	if a%b != 0 && a < 0 {
		q--
	}
	return q
}

func addFloored(anchor, t time.Time, interval time.Duration) time.Time {
	n := floorDiv(t.UnixNano()-anchor.UnixNano(), int64(interval))
	return time.Unix(0, anchor.UnixNano()+n*int64(interval)).In(anchor.Location())
}

func epochFloor(t time.Time, interval time.Duration) time.Time {
	n := floorDiv(t.UnixNano(), int64(interval))
	return time.Unix(0, n*int64(interval)).UTC()
}

func bucketStart(spec BarSpec, cal session.Calendar, t time.Time, td time.Time, sess session.Session) time.Time {
	switch spec.Anchor {
	case AnchorUTCEpoch:
		return epochFloor(t, spec.Interval)
	case AnchorMidnightLocal:
		loc := spec.loc(cal)
		local := t.In(loc)
		y, m, d := local.Date()
		anchor := time.Date(y, m, d, 0, 0, 0, 0, loc)
		return addFloored(anchor, t, spec.Interval)
	case AnchorRTHOpen:
		h := cal.Schedule.HoursFor(td)
		// A 1d bar is one bar per trading date. Walking a negative
		// elapsed back from RTH open would put Sunday evening ETH on
		// Sunday's 08:30 — the wrong trading date.
		if spec.Interval >= 24*time.Hour {
			return h.RTHOpen
		}
		return addFloored(h.RTHOpen, t, spec.Interval)
	default: // AnchorSessionOpen
		h := cal.Schedule.HoursFor(td)
		anchor := h.ETHOpen
		if sess == session.RTH {
			anchor = h.RTHOpen
		}
		if spec.Interval >= 24*time.Hour {
			return anchor
		}
		return addFloored(anchor, t, spec.Interval)
	}
}
