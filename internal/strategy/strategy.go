package strategy

import (
	"fmt"
	"io"

	"github.com/morteza-sakifard/market-data-lab/internal/marketdata"
)

// Strategy is the only type a runner should depend on. It sees the
// market through Context, never through a file or a socket, so the
// same value can run on a CSV Source today and a live Source later
// without a Mode field to branch on. OnEvent must not retain ev:
// Run reuses one Event, the same way feed.Source.Next does.
//
// Submit, OnBar, and OnOrder wait for the venue in step 20. Adding
// them here would import packages that do not exist yet, or return
// zeros that look like real fills.
type Strategy interface {
	OnStart(Context) error
	OnEvent(Context, *marketdata.Event) error
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
	var ev marketdata.Event
	for {
		err := src.Next(&ev)
		if err == io.EOF {
			return s.OnStop(rt)
		}
		if err != nil {
			return err
		}
		rt.Observe(&ev)
		if err := s.OnEvent(rt, &ev); err != nil {
			return err
		}
	}
}
