package orderflow

import (
	"sort"

	"github.com/morteza-sakifard/TickEngine/internal/core"
	"github.com/morteza-sakifard/TickEngine/internal/marketdata"
)

// PriceLevel is one row of a volume profile. Price is Ticks, never
// float64: adjacent means px-1, and a float key would silently merge
// 6700.00 and 6700.0000001.
type PriceLevel struct {
	Price  core.Ticks
	Volume core.Qty
}

// VolumeProfile is traded volume by price. POC is the mode; the
// value area is the contiguous block around POC that first covers
// 70% of total volume. The type does not know what a session is:
// Reset is called from outside. See docs/00-architecture.md L5.
type VolumeProfile struct {
	vol map[core.Ticks]core.Qty
	sum core.Qty
}

func (vp *VolumeProfile) OnTrade(ev *marketdata.Event) {
	if ev == nil || ev.Kind != marketdata.KindTrade {
		return
	}
	qty := ev.Trade.Qty
	if qty <= 0 {
		return
	}
	if vp.vol == nil {
		vp.vol = make(map[core.Ticks]core.Qty)
	}
	vp.vol[ev.Trade.Px] += qty
	vp.sum += qty
}

func (vp *VolumeProfile) Reset() { *vp = VolumeProfile{} }

func (vp *VolumeProfile) Total() core.Qty { return vp.sum }

// Levels returns rows sorted by ascending price. Empty if nothing
// was added. The sort is what keeps RenderSVG deterministic: the
// map itself has no order.
func (vp *VolumeProfile) Levels() []PriceLevel {
	if len(vp.vol) == 0 {
		return nil
	}
	out := make([]PriceLevel, 0, len(vp.vol))
	for px, q := range vp.vol {
		out = append(out, PriceLevel{Price: px, Volume: q})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Price < out[j].Price })
	return out
}

func (vp *VolumeProfile) POC() core.Ticks {
	_, poc, _ := vp.ValueArea()
	return poc
}

func (vp *VolumeProfile) VAL() core.Ticks {
	val, _, _ := vp.ValueArea()
	return val
}

func (vp *VolumeProfile) VAH() core.Ticks {
	_, _, vah := vp.ValueArea()
	return vah
}

// ValueArea returns VAL, POC, VAH. Empty input is (0, 0, 0).
//
// From the POC row, each step adds the unused neighbour (next
// traded price above or below) with the larger volume until the
// running total reaches ceil(70% of Σ). A tie expands toward the
// lower price so the walk is deterministic. The last row may push
// the area over 70%; that overshoot is at most one level — the
// roadmap acceptance bound.
func (vp *VolumeProfile) ValueArea() (val, poc, vah core.Ticks) {
	levels := vp.Levels()
	if len(levels) == 0 {
		return 0, 0, 0
	}
	pocI := 0
	for i := 1; i < len(levels); i++ {
		if levels[i].Volume > levels[pocI].Volume {
			pocI = i
		}
	}
	poc = levels[pocI].Price
	if vp.sum <= 0 {
		return poc, poc, poc
	}
	target := (int64(vp.sum)*70 + 99) / 100
	acc := int64(levels[pocI].Volume)
	lo, hi := pocI, pocI
	for acc < target && (lo > 0 || hi < len(levels)-1) {
		var down, up core.Qty
		if lo > 0 {
			down = levels[lo-1].Volume
		}
		if hi < len(levels)-1 {
			up = levels[hi+1].Volume
		}
		switch {
		case lo == 0:
			hi++
			acc += int64(levels[hi].Volume)
		case hi == len(levels)-1:
			lo--
			acc += int64(levels[lo].Volume)
		case down > up:
			lo--
			acc += int64(levels[lo].Volume)
		case up > down:
			hi++
			acc += int64(levels[hi].Volume)
		default:
			lo--
			acc += int64(levels[lo].Volume)
		}
	}
	return levels[lo].Price, poc, levels[hi].Price
}
