package execution

import (
	"fmt"

	"github.com/morteza-sakifard/market-data-lab/internal/core"
)

var (
	ErrRiskKilled   = fmt.Errorf("execution: risk kill switch")
	ErrRiskQty      = fmt.Errorf("execution: risk max order qty")
	ErrRiskPosition = fmt.Errorf("execution: risk max position")
	ErrRiskDayLoss  = fmt.Errorf("execution: risk max day loss")
	ErrRiskRate     = fmt.Errorf("execution: risk order rate")
)

// Limits are the hard caps in front of the venue. A zero field is
// off. Checks run on the intended order, not after a fill — a fill
// that already happened cannot be unread.
type Limits struct {
	MaxQty          core.Qty // per order
	MaxAbsPosition  core.Qty // |pos + signed qty|
	MaxDayLossCents int64    // freeze new risk when dayPnL <= -this
	MaxOrders       int      // accepted submits in WindowNs
	WindowNs        int64
}

func (l Limits) validate() error {
	if l.MaxQty < 0 || l.MaxAbsPosition < 0 {
		return fmt.Errorf("execution: risk qty limits must be >= 0")
	}
	if l.MaxDayLossCents < 0 {
		return fmt.Errorf("execution: risk max day loss must be >= 0")
	}
	if l.MaxOrders < 0 || l.WindowNs < 0 {
		return fmt.Errorf("execution: risk rate limits must be >= 0")
	}
	if l.MaxOrders > 0 && l.WindowNs <= 0 {
		return fmt.Errorf("execution: risk MaxOrders requires WindowNs")
	}
	return nil
}

// Risk is the last gate before Enqueue. now is the runner's clock
// (TsRecv), never the wall. Kill stays set until the runner calls Arm.
type Risk struct {
	lim    Limits
	killed bool
	stamps []int64
}

func NewRisk(lim Limits) (*Risk, error) {
	if err := lim.validate(); err != nil {
		return nil, err
	}
	return &Risk{lim: lim}, nil
}

func (r *Risk) Kill() {
	if r != nil {
		r.killed = true
	}
}

// Arm is the operator reset of the kill switch. The strategy has
// no method for this — only the runner flips it back on.
func (r *Risk) Arm() {
	if r != nil {
		r.killed = false
	}
}

func (r *Risk) Killed() bool {
	return r != nil && r.killed
}

func (r *Risk) Limits() Limits {
	if r == nil {
		return Limits{}
	}
	return r.lim
}

// Allow is the gate. pos is signed qty already on the book. dayPnL
// is realized cents so far this session (fees included). now is the
// virtual clock. On success the submit is counted for the rate cap.
func (r *Risk) Allow(o Order, pos core.Qty, dayPnL, now int64) error {
	if r == nil {
		return nil
	}
	if r.killed {
		return ErrRiskKilled
	}
	if r.lim.MaxQty > 0 && o.Qty > r.lim.MaxQty {
		return ErrRiskQty
	}
	if r.lim.MaxAbsPosition > 0 {
		next := pos + signedQty(o)
		if absQty(next) > r.lim.MaxAbsPosition {
			return ErrRiskPosition
		}
	}
	if r.lim.MaxDayLossCents > 0 && dayPnL <= -r.lim.MaxDayLossCents {
		if !reducesTowardFlat(o, pos) {
			return ErrRiskDayLoss
		}
	}
	if r.lim.MaxOrders > 0 {
		r.prune(now)
		if len(r.stamps) >= r.lim.MaxOrders {
			return ErrRiskRate
		}
	}
	if r.lim.MaxOrders > 0 {
		r.stamps = append(r.stamps, now)
	}
	return nil
}

func (r *Risk) prune(now int64) {
	cut := now - r.lim.WindowNs
	n := 0
	for _, ts := range r.stamps {
		if ts >= cut {
			r.stamps[n] = ts
			n++
		}
	}
	r.stamps = r.stamps[:n]
}

func signedQty(o Order) core.Qty {
	if o.Side == core.SideAsk {
		return -o.Qty
	}
	return o.Qty
}

// reducesTowardFlat is true only for a cut that cannot flip the
// sign. A sell of 6 against a long of 5 is new short risk.
func reducesTowardFlat(o Order, pos core.Qty) bool {
	if pos == 0 {
		return false
	}
	if pos > 0 {
		return o.Side == core.SideAsk && o.Qty <= pos
	}
	return o.Side == core.SideBid && o.Qty <= -pos
}

func absQty(q core.Qty) core.Qty {
	if q < 0 {
		return -q
	}
	return q
}
