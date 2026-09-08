package portfolio

import (
	"github.com/morteza-sakifard/market-data-lab/internal/core"
	"github.com/morteza-sakifard/market-data-lab/internal/execution"
)

// Position is one instrument's signed size. Qty > 0 is long, Qty < 0
// is short. Realized is integer USD cents and includes fees from
// every fill. AvgPx is the volume-weighted open price; it is 0
// when flat.
type Position struct {
	Qty      core.Qty
	AvgPx    core.Ticks
	Realized int64
}

// Apply books a fill. A reversing order (long 1, sell 2) closes the
// long at the fill price, realizes that PnL, then opens a short of
// 1 at the same price — one ticket, two economic events.
func (p *Position) Apply(inst core.Instrument, f execution.Fill) {
	if p == nil || f.Qty <= 0 {
		return
	}
	signed := f.Qty
	if f.Side == core.SideAsk {
		signed = -f.Qty
	}
	p.Realized -= f.FeeCents
	if p.Qty == 0 || sameWay(p.Qty, signed) {
		absOld := absQty(p.Qty)
		sum := int64(p.AvgPx)*int64(absOld) + int64(f.Px)*int64(f.Qty)
		p.Qty += signed
		p.AvgPx = core.Ticks(sum / int64(absQty(p.Qty)))
		return
	}
	closed := f.Qty
	openAbs := absQty(p.Qty)
	if closed > openAbs {
		closed = openAbs
	}
	ticks := int64(f.Px - p.AvgPx)
	if p.Qty < 0 {
		ticks = int64(p.AvgPx - f.Px)
	}
	p.Realized += ticks * int64(closed) * CentsPerTick(inst)
	remain := f.Qty - closed
	if remain == 0 {
		p.Qty = 0
		p.AvgPx = 0
		return
	}
	if signed > 0 {
		p.Qty = remain
	} else {
		p.Qty = -remain
	}
	p.AvgPx = f.Px
}

func sameWay(pos, signed core.Qty) bool {
	return (pos > 0 && signed > 0) || (pos < 0 && signed < 0)
}

func absQty(q core.Qty) core.Qty {
	if q < 0 {
		return -q
	}
	return q
}
