package strategy

import (
	"github.com/morteza-sakifard/TickEngine/internal/execution"
	"github.com/morteza-sakifard/TickEngine/internal/marketdata"
)

// LogStrategy writes one line per callback and does nothing else.
// It has no fields, so the same value runs on any Source without
// a branch on where the events came from.
type LogStrategy struct{}

func (LogStrategy) OnStart(ctx Context) error {
	if ctx != nil {
		ctx.Logf("start %s\n", ctx.Instrument().Symbol)
	}
	return nil
}

func (LogStrategy) OnEvent(ctx Context, ev *marketdata.Event) error {
	if ctx == nil || ev == nil {
		return nil
	}
	// Numbers only — no pointer stored, no wall clock.
	ctx.Logf("%d %s %d %d %d %d %d %d %d\n",
		ctx.UnixNano(), ev.Kind, ev.Sequence, ev.Flags,
		ev.Trade.Px, ev.Trade.Qty, ev.Trade.Aggressor,
		ev.Quote.BidPx, ev.Quote.AskPx)
	return nil
}

func (LogStrategy) OnOrder(Context, execution.OrderEvent) error { return nil }

func (LogStrategy) OnStop(ctx Context) error {
	if ctx != nil {
		ctx.Logf("stop\n")
	}
	return nil
}
