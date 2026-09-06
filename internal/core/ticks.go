package core

// Ticks is a price in the instrument's own tick units.
// ES (0.25): 6713.50 -> 26854.
type Ticks int64

// Qty is a signed contract count. Delta and position go negative;
// uint would force a cast at the first subtraction, which is where
// underflow hides.
type Qty int64

type Side uint8

const (
	SideNone Side = iota
	SideBid       // buyer aggressor — lifted the ask
	SideAsk       // seller aggressor — hit the bid
)

func (s Side) String() string {
	switch s {
	case SideBid:
		return "bid"
	case SideAsk:
		return "ask"
	default:
		return "none"
	}
}
