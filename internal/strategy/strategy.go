package strategy

import (
	"fmt"
	"io"

	"github.com/morteza-sakifard/market-data-lab/internal/execution"
	"github.com/morteza-sakifard/market-data-lab/internal/marketdata"
)

// maxCascade caps OnEvent re-entry after a fill at one timestamp.
// Without it a strategy that always submits never returns.
const maxCascade = 32

// Strategy is the only type a runner should depend on. It sees the
// market through Context, never through a file or a socket, so the
// same value can run on a CSV Source today and a live Source later
// without a Mode field to branch on. OnEvent must not retain ev:
// Run reuses one Event, the same way feed.Source.Next does.
//
// OnBar waits for a bar builder. Limit queueing is step 21.
type Strategy interface {
	OnStart(Context) error
	OnEvent(Context, *marketdata.Event) error
	OnOrder(Context, execution.OrderEvent) error
	OnStop(Context) error
}

// Source is the consumer-owned shape of feed.Source. The interface
// lives here so this package never imports feed: go list -deps
// ./internal/strategy must not show feed, because a strategy that
// can see the decoder can tell a file from a socket.
type Source interface {
	Next(dst *marketdata.Event) error
	Close() error
}

// Run reads src until EOF, advances rt on TsRecv, and calls s.
// It does not Close src; the caller owns the handle. A nil Pacer
// or Engine is not involved — importing replay would pull feed
// into this package's dependency graph.
func Run(src Source, s Strategy, rt *Runtime) error {
	if src == nil {
		return fmt.Errorf("strategy: nil source")
	}
	if s == nil {
		return fmt.Errorf("strategy: nil strategy")
	}
	if rt == nil {
		return fmt.Errorf("strategy: nil runtime")
	}
	if err := s.OnStart(rt); err != nil {
		return err
	}
	if err := rt.drain(s); err != nil {
		return err
	}
	var ev marketdata.Event
	for {
		err := src.Next(&ev)
		if err == io.EOF {
			if err := s.OnStop(rt); err != nil {
				return err
			}
			if err := rt.drain(s); err != nil {
				return err
			}
			return rt.apply(s, rt.venue.Flush())
		}
		if err != nil {
			return err
		}
		rt.Observe(&ev)
		if rest := rt.venue.MatchResting(&ev); len(rest) > 0 {
			if err := rt.apply(s, rest); err != nil {
				return err
			}
		}
		if err := rt.cascade(s, &ev); err != nil {
			return err
		}
	}
}

// cascade is the L8 loop: strategy sees the event, submits, venue
// settles, and a new fill re-enters OnEvent at the same clock.
func (rt *Runtime) cascade(s Strategy, ev *marketdata.Event) error {
	for i := 0; i < maxCascade; i++ {
		if err := s.OnEvent(rt, ev); err != nil {
			return err
		}
		events := rt.venue.Settle(rt.UnixNano())
		if len(events) == 0 {
			return nil
		}
		if err := rt.apply(s, events); err != nil {
			return err
		}
		if !anyFill(events) {
			return nil
		}
	}
	return fmt.Errorf("strategy: cascade exceeded %d at ts=%d", maxCascade, rt.UnixNano())
}

func (rt *Runtime) drain(s Strategy) error {
	for i := 0; i < maxCascade; i++ {
		events := rt.venue.Settle(rt.UnixNano())
		if len(events) == 0 {
			return nil
		}
		if err := rt.apply(s, events); err != nil {
			return err
		}
	}
	return fmt.Errorf("strategy: settle loop exceeded %d", maxCascade)
}

func (rt *Runtime) apply(s Strategy, events []execution.OrderEvent) error {
	for _, e := range events {
		if e.Status == execution.StatusFilled {
			rt.pos.Apply(rt.inst, e.Fill)
		}
		if err := s.OnOrder(rt, e); err != nil {
			return err
		}
	}
	return nil
}

func anyFill(events []execution.OrderEvent) bool {
	for _, e := range events {
		if e.Status == execution.StatusFilled {
			return true
		}
	}
	return false
}
