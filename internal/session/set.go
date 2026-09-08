package session

// Set is a bitset of Session values. Closed is never a member: a
// trade in Closed hours is a calendar bug, not a bar you asked for.
//
// The zero value means every open session (RTH and ETH). A BarSpec
// that never sets Sessions therefore sees every trade Classify
// assigns to RTH or ETH — the same as ohlcv.Builder, which
// classified every trade and never filtered.
type Set uint8

const (
	SetRTH Set = 1 << RTH
	SetETH Set = 1 << ETH
)

func (s Set) Contains(sess Session) bool {
	if sess == Closed {
		return false
	}
	if s == 0 {
		return true
	}
	return s&(1<<sess) != 0
}
