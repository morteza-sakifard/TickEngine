package orderflow

import "github.com/morteza-sakifard/market-data-lab/internal/marketdata"

// Accumulator is the contract every order-flow stat implements.
// None of them know what a session is: Reset is called from
// outside, by the orchestrator, when a session.Boundary is
// crossed. Daily VWAP, session VWAP, and RTH VWAP are the same
// math; only the reset schedule changes. See
// docs/00-architecture.md L5.
type Accumulator interface {
	OnTrade(ev *marketdata.Event)
	Reset()
}
