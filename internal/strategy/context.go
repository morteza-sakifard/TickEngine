package strategy

import (
	"fmt"
	"io"
	"time"

	"github.com/morteza-sakifard/market-data-lab/internal/core"
	"github.com/morteza-sakifard/market-data-lab/internal/marketdata"
)

// Context is the strategy's only window to the outside world.
// Everything a strategy is allowed to know is a method here: clock,
// instrument, last print, last quote. There is no Mode, no Live
// flag, and no way to open a file. If a strategy can tell it is
// in a backtest, it will eventually behave differently there than
// it does live. See docs/00-architecture.md L8.
type Context interface {
	Now() time.Time
	UnixNano() int64
	Instrument() core.Instrument
	LastTrade() (marketdata.Trade, bool)
	Quote() (marketdata.Quote, bool)
	Last() (marketdata.Event, bool)
	Logf(format string, args ...any)
}

// Runtime is the backtest-side Context. A live runner in a later
// step will be a different type with the same methods. The clock
// is virtual and moves only on Observe: wall-clock reads are not
// used, because they are neither replayable nor testable.
type Runtime struct {
	inst  core.Instrument
	ns    int64
	cache Cache
	log   io.Writer
}

func NewRuntime(inst core.Instrument, log io.Writer) *Runtime {
	return &Runtime{inst: inst, log: log}
}

func (rt *Runtime) Now() time.Time {
	if rt == nil {
		return time.Unix(0, 0).UTC()
	}
	return time.Unix(0, rt.ns).UTC()
}

func (rt *Runtime) UnixNano() int64 {
	if rt == nil {
		return 0
	}
	return rt.ns
}

func (rt *Runtime) Instrument() core.Instrument {
	if rt == nil {
		return core.Instrument{}
	}
	return rt.inst
}

func (rt *Runtime) LastTrade() (marketdata.Trade, bool) {
	if rt == nil {
		return marketdata.Trade{}, false
	}
	return rt.cache.LastTrade()
}

func (rt *Runtime) Quote() (marketdata.Quote, bool) {
	if rt == nil {
		return marketdata.Quote{}, false
	}
	return rt.cache.Quote()
}

func (rt *Runtime) Last() (marketdata.Event, bool) {
	if rt == nil {
		return marketdata.Event{}, false
	}
	return rt.cache.Last()
}

func (rt *Runtime) Cache() *Cache {
	if rt == nil {
		return nil
	}
	return &rt.cache
}

func (rt *Runtime) Logf(format string, args ...any) {
	if rt == nil || rt.log == nil {
		return
	}
	fmt.Fprintf(rt.log, format, args...)
}

// Observe copies ev into the cache and advances the clock on
// TsRecv. FlagBadTsRecv skips Advance: the stamp is unusable, but
// the print still happened, so the cache still updates. The clock
// never moves backward. Same rules as replay.Engine, kept here so
// this package does not import replay.
func (rt *Runtime) Observe(ev *marketdata.Event) {
	if rt == nil || ev == nil {
		return
	}
	if ev.Flags&marketdata.FlagBadTsRecv == 0 && ev.TsRecv > rt.ns {
		rt.ns = ev.TsRecv
	}
	rt.cache.onEvent(ev)
}
