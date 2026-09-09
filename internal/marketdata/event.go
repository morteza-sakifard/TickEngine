package marketdata

import (
	"time"

	"github.com/morteza-sakifard/market-data-lab/internal/core"
)

// Kind discriminates the payloads Event can carry. See
// docs/00-architecture.md 5.1 for why Event is a struct with a Kind
// field instead of an interface.
type Kind uint8

const (
	KindTrade Kind = iota + 1
	KindQuote
	KindStatus
)

// String exists for the same reason core.Side has one: Kind shows up
// in test failures and future logging, and a small enum should not
// print as a bare integer.
func (k Kind) String() string {
	switch k {
	case KindTrade:
		return "trade"
	case KindQuote:
		return "quote"
	case KindStatus:
		return "status"
	default:
		return "unknown"
	}
}

// Event is every normalized market-data record in the system: the
// common language every layer above feed speaks. It is a struct with a
// Kind discriminant, not an interface, and its timestamps are int64
// Unix nanoseconds, not time.Time, for the allocation and heap-pointer
// reasons in docs/00-architecture.md 5.1 and 5.2. Trade is valid only
// when Kind == KindTrade, Quote only when Kind == KindQuote. KindStatus
// carries no payload yet: nothing in the roadmap needs an
// instrument-status event before the order-book steps, so a Status
// struct here would be a field nobody can fill in or test today.
type Event struct {
	Kind       Kind
	Instrument core.InstrumentID

	TsEvent   int64 // Unix nanoseconds, exchange matching engine
	TsRecv    int64 // Unix nanoseconds, Databento capture gateway
	TsInDelta int32 // nanoseconds, ts_recv minus matching-engine send time

	Sequence uint32
	Flags    Flags

	Trade Trade
	Quote Quote
	Depth Depth // KindQuote: L2 after the event; MBP-1 fills slot 0
}

// EventTime is TsEvent as a time.Time, for display and for code outside
// the hot loop. Inside a hot loop, compare TsEvent directly as int64;
// do not compare two EventTime() results with ==, because time.Time
// carries a monotonic reading that makes == behave unintuitively (use
// Equal instead).
func (e *Event) EventTime() time.Time { return time.Unix(0, e.TsEvent).UTC() }

// RecvTime is TsRecv as a time.Time. The replay clock built in step 14
// runs on TsRecv, not TsEvent: TsRecv is what you could actually have
// observed at the time, so running the clock on TsEvent would be
// look-ahead bias.
func (e *Event) RecvTime() time.Time { return time.Unix(0, e.TsRecv).UTC() }
