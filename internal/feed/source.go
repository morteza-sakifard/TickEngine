package feed

import "github.com/morteza-sakifard/market-data-lab/internal/marketdata"

// Source produces normalized market-data events, one at a time, in the
// order they should be observed: for a single row that implies both a
// trade and a quote, the Trade comes before the Quote it produced. It
// is the single seam between "where data comes from" — a CSV file
// today (feed/databento), a live socket in step 27 — and everything
// built on marketdata.Event: aggregation, orderflow, chart, strategy.
// None of those layers import feed/databento directly; they only see
// Source. See docs/00-architecture.md L2.
//
// Next writes the next event into dst in place and returns nil. It
// returns io.EOF at a clean end of input, or any other error on a
// decode failure; *dst is unspecified in both of those cases. The
// in-place signature exists so a caller — typically the replay engine
// built in step 14 — can decode millions of events into one reused
// Event without allocating per call. See
// docs/steps/04-feed-databento.reference.md for how feed/databento
// meets that bar and how to verify it yourself.
//
// Close releases whatever Next was reading from (a file handle today,
// a live connection later).
type Source interface {
	Next(dst *marketdata.Event) error
	Close() error
}
