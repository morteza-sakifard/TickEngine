package orderflow

import (
	"github.com/morteza-sakifard/market-data-lab/internal/marketdata"
	"github.com/morteza-sakifard/market-data-lab/internal/session"
)

// Orchestrator fans each event to a list of Accumulator values and
// calls Reset on every one when the event time crosses a
// session.Boundary. Accumulators stay session-blind; this type is
// the only place that knows an edge happened. Events must be in
// TsEvent order — the same order feed.Source already yields.
type Orchestrator struct {
	bounds []session.Boundary
	next   int
	accs   []Accumulator
}

func New(bounds []session.Boundary, accs ...Accumulator) *Orchestrator {
	return &Orchestrator{bounds: bounds, accs: accs}
}

func (o *Orchestrator) OnEvent(ev *marketdata.Event) {
	if o == nil || ev == nil {
		return
	}
	t := ev.EventTime()
	for o.next < len(o.bounds) && !t.Before(o.bounds[o.next].At) {
		for _, a := range o.accs {
			a.Reset()
		}
		o.next++
	}
	if ev.Kind != marketdata.KindTrade {
		return
	}
	for _, a := range o.accs {
		a.OnTrade(ev)
	}
}
