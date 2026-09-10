package execution

import (
	"fmt"
	"sort"

	"github.com/morteza-sakifard/TickEngine/internal/marketdata"
)

// Latency is the two delays you control. Feed delay is already in
// the record (TsInDelta, TsRecv − TsEvent) and on the replay clock
// (TsRecv). Adding it here again would double-count: the strategy
// would see a print later than the gateway did.
//
// Entry is submit → matching engine. Response is fill → strategy.
// Both are constant nanoseconds, not draws from a random source,
// so two runs with the same config match. Zero is step 20: the
// order sits in the inbox and the fill comes back on the same tick.
type Latency struct {
	Entry    int64
	Response int64
}

func (l Latency) validate() error {
	if l.Entry < 0 || l.Response < 0 {
		return fmt.Errorf("execution: latency must be >= 0")
	}
	return nil
}

// FeedNs is ts_in_delta: matching-engine send to the capture
// gateway. That is the feed delay the file actually measured.
func FeedNs(ev marketdata.Event) int64 {
	return int64(ev.TsInDelta)
}

// RecvLag is TsRecv − TsEvent, the other view of the same hop
// when the gateway stamp is usable.
func RecvLag(ev marketdata.Event) int64 {
	return ev.TsRecv - ev.TsEvent
}

// FeedDist is an empirical feed-latency sample. Census already
// histograms ts_in_delta on the full file; this is the same
// quantity so a backtest can pin a number to the data, not a guess.
type FeedDist struct {
	samples []int64
}

func (d *FeedDist) Add(ev marketdata.Event) {
	if d == nil {
		return
	}
	d.samples = append(d.samples, FeedNs(ev))
}

func (d FeedDist) N() int { return len(d.samples) }

func (d FeedDist) Min() int64 {
	if len(d.samples) == 0 {
		return 0
	}
	m := d.samples[0]
	for _, v := range d.samples[1:] {
		if v < m {
			m = v
		}
	}
	return m
}

func (d FeedDist) Max() int64 {
	if len(d.samples) == 0 {
		return 0
	}
	m := d.samples[0]
	for _, v := range d.samples[1:] {
		if v > m {
			m = v
		}
	}
	return m
}

func (d FeedDist) Mean() int64 {
	if len(d.samples) == 0 {
		return 0
	}
	var sum int64
	for _, v := range d.samples {
		sum += v
	}
	return sum / int64(len(d.samples))
}

// Percentile is nearest-rank on a sorted copy, p in 0..100.
func (d FeedDist) Percentile(p int) int64 {
	n := len(d.samples)
	if n == 0 {
		return 0
	}
	if p < 0 {
		p = 0
	}
	if p > 100 {
		p = 100
	}
	cp := append([]int64(nil), d.samples...)
	sort.Slice(cp, func(i, j int) bool { return cp[i] < cp[j] })
	if p == 0 {
		return cp[0]
	}
	i := (p*n + 99) / 100
	if i < 1 {
		i = 1
	}
	if i > n {
		i = n
	}
	return cp[i-1]
}

type delayedOrder struct {
	order Order
	ready int64
}

type delayedEvent struct {
	ev    OrderEvent
	ready int64
}

func (v *Venue) releaseDue(ts int64) {
	if v == nil {
		return
	}
	n := 0
	for _, d := range v.delayed {
		if d.ready <= ts {
			v.inbox = append(v.inbox, d.order)
			continue
		}
		v.delayed[n] = d
		n++
	}
	v.delayed = v.delayed[:n]
}

func (v *Venue) holdOrEmit(raw []OrderEvent, ts int64) []OrderEvent {
	if v == nil {
		return raw
	}
	if v.lat.Response > 0 {
		for _, e := range raw {
			v.outbox = append(v.outbox, delayedEvent{ev: e, ready: ts + v.lat.Response})
		}
		raw = nil
	}
	due := v.deliverDue(ts)
	if len(raw) == 0 {
		return due
	}
	if len(due) == 0 {
		return raw
	}
	return append(raw, due...)
}

func (v *Venue) deliverDue(ts int64) []OrderEvent {
	if v == nil || len(v.outbox) == 0 {
		return nil
	}
	n := 0
	var out []OrderEvent
	for _, d := range v.outbox {
		if d.ready <= ts {
			out = append(out, d.ev)
			continue
		}
		v.outbox[n] = d
		n++
	}
	v.outbox = v.outbox[:n]
	return out
}
