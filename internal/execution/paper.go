package execution

import (
	"fmt"

	"github.com/morteza-sakifard/market-data-lab/internal/core"
)

// Paper is market data (file or socket) plus a simulated venue.
// Orders stay in this process: there is no broker. Risk.Allow runs
// before Enqueue so a bug in the strategy cannot size or spam the
// book. The runner still owns the Source and the clock.
type Paper struct {
	venue *Venue
	risk  *Risk
}

func NewPaper(fees Fees, lim Limits) (*Paper, error) {
	r, err := NewRisk(lim)
	if err != nil {
		return nil, err
	}
	if err := fees.validate(); err != nil {
		return nil, err
	}
	return &Paper{venue: NewVenue(fees), risk: r}, nil
}

func (p *Paper) Venue() *Venue {
	if p == nil {
		return nil
	}
	return p.venue
}

func (p *Paper) Risk() *Risk {
	if p == nil {
		return nil
	}
	return p.risk
}

func (p *Paper) Kill() {
	if p != nil && p.risk != nil {
		p.risk.Kill()
	}
}

func (p *Paper) Arm() {
	if p != nil && p.risk != nil {
		p.risk.Arm()
	}
}

// Enqueue is Strategy → Risk → Venue. pos and dayPnL are the
// runner's books, not this type's — one source of truth.
func (p *Paper) Enqueue(o Order, pos core.Qty, dayPnL, now int64) (OrderID, error) {
	if p == nil || p.venue == nil {
		return 0, fmt.Errorf("execution: nil paper")
	}
	if p.risk != nil {
		if err := p.risk.Allow(o, pos, dayPnL, now); err != nil {
			return 0, err
		}
	}
	return p.venue.Enqueue(o)
}
