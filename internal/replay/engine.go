package replay

import (
	"context"
	"fmt"
	"io"

	"github.com/morteza-sakifard/TickEngine/internal/feed"
	"github.com/morteza-sakifard/TickEngine/internal/marketdata"
)

// Handler is invoked synchronously, in Subscribe order, for every
// event. OnEvent must not retain ev: Next reuses one Event. Copy
// fields you need to keep. Two Runs on the same source bytes are
// the same log.
type Handler interface {
	OnEvent(ev *marketdata.Event)
}

// Engine is the deterministic replay loop. It owns the ReplayClock
// and a list of handlers. Fan-out is synchronous. Context is polled
// with ctx.Err() so this file stays free of goroutine primitives.
type Engine struct {
	src      feed.Source
	clock    ReplayClock
	handlers []Handler
	pacer    *Pacer
}

// HandlerFunc is a function adapter for Handler.
type HandlerFunc func(*marketdata.Event)

func (f HandlerFunc) OnEvent(ev *marketdata.Event) { f(ev) }

func New(src feed.Source) *Engine {
	return &Engine{src: src}
}

func (e *Engine) Subscribe(h Handler) {
	if e == nil || h == nil {
		return
	}
	e.handlers = append(e.handlers, h)
}

func (e *Engine) Clock() Clock {
	if e == nil {
		return (*ReplayClock)(nil)
	}
	return &e.clock
}

func (e *Engine) SetPacer(p *Pacer) {
	if e == nil {
		return
	}
	e.pacer = p
}

// Run reads src until EOF or ctx cancellation. For each event it
// advances the clock on TsRecv unless FlagBadTsRecv is set, then
// calls every handler. A Pacer, if set, waits after a successful
// Next for every event except the first (so the last print does
// not wait for an extra Enter) and sleeps after each delivery
// using the clock delta. A nil Pacer is the same loop as step 14.
// The source is not Closed; the caller owns it.
func (e *Engine) Run(ctx context.Context) error {
	if e == nil || e.src == nil {
		return fmt.Errorf("replay: nil engine or source")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	var ev marketdata.Event
	first := true
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		err := e.src.Next(&ev)
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		if !first && e.pacer != nil {
			e.pacer.Await()
		}
		// FlagBadTsRecv: ts_recv is not a usable observation time.
		// Do not Advance. Still deliver — dropping the print would
		// invent a hole that the file does not have.
		before := e.clock.UnixNano()
		if ev.Flags&marketdata.FlagBadTsRecv == 0 {
			e.clock.Advance(ev.TsRecv)
		}
		for _, h := range e.handlers {
			h.OnEvent(&ev)
		}
		if e.pacer != nil {
			e.pacer.Between(e.clock.UnixNano() - before)
		}
		first = false
	}
}
