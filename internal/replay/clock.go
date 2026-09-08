package replay

import "time"

// Clock is the only time a handler is allowed to see. Now and
// UnixNano are the same instant; the hot path uses UnixNano (int64
// compare, no Location pointer). See docs/00-architecture.md 5.2.
type Clock interface {
	Now() time.Time
	UnixNano() int64
}

// ReplayClock is a virtual clock driven by TsRecv. TsRecv is what a
// live observer could have known at that moment; advancing on TsEvent
// would be look-ahead — the matching engine's time is not visible
// until the gateway stamps ts_recv.
//
// The clock never moves backward. An earlier or equal TsRecv is
// ignored. FlagBadTsRecv is handled in Engine: those records do not
// call Advance, because the stamp is unreliable. The event is still
// delivered; a bad clock is not a missing trade.
type ReplayClock struct {
	ns int64
}

func (c *ReplayClock) Now() time.Time {
	if c == nil {
		return time.Unix(0, 0).UTC()
	}
	return time.Unix(0, c.ns).UTC()
}

func (c *ReplayClock) UnixNano() int64 {
	if c == nil {
		return 0
	}
	return c.ns
}

// Advance sets the clock to ts if ts is strictly later. It is the
// only mutation; handlers cannot rewind the market.
func (c *ReplayClock) Advance(ts int64) {
	if c == nil || ts <= c.ns {
		return
	}
	c.ns = ts
}
