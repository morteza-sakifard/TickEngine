package chart

import (
	"math"

	"github.com/morteza-sakifard/TickEngine/internal/aggregation"
	"github.com/morteza-sakifard/TickEngine/internal/core"
)

// Scale maps Ticks → y and bar index → x. X is the bar index, not
// wall-clock time: a 15:30 bar and the 17:00 bar after the
// maintenance break sit one slot apart, the same as 14:30 and 15:00.
// Plotting Unix time would leave a Closed hole — the thing every
// real platform removes and the step-8 acceptance criterion forbids.
type Scale struct {
	YMin, YMax  core.Ticks
	Left, Right float64
	Top, Bottom float64
	N           int
}

func newScale(bars []aggregation.Bar, left, right, top, bottom float64) Scale {
	lo, hi := priceBounds(bars)
	step := niceStep(hi-lo, 6)
	lo -= step
	hi += step
	if lo == hi {
		hi = lo + 1
	}
	return Scale{
		YMin: lo, YMax: hi,
		Left: left, Right: right,
		Top: top, Bottom: bottom,
		N: len(bars),
	}
}

// X is the horizontal center of bar i.
func (s Scale) X(i int) float64 {
	if s.N <= 0 {
		return s.Left
	}
	slot := (s.Right - s.Left) / float64(s.N)
	return s.Left + (float64(i)+0.5)*slot
}

func (s Scale) slotWidth() float64 {
	if s.N <= 0 {
		return 0
	}
	return (s.Right - s.Left) / float64(s.N)
}

// Y maps a price onto SVG y (downward). YMin is at Bottom, YMax at Top.
func (s Scale) Y(px core.Ticks) float64 {
	if s.YMax == s.YMin {
		return (s.Top + s.Bottom) / 2
	}
	frac := float64(s.YMax-px) / float64(s.YMax-s.YMin)
	return s.Top + frac*(s.Bottom-s.Top)
}

func priceBounds(bars []aggregation.Bar) (lo, hi core.Ticks) {
	if len(bars) == 0 {
		return 0, 1
	}
	lo, hi = bars[0].Low, bars[0].High
	for i := 1; i < len(bars); i++ {
		if bars[i].Low < lo {
			lo = bars[i].Low
		}
		if bars[i].High > hi {
			hi = bars[i].High
		}
	}
	if lo == hi {
		hi = lo + 1
	}
	return lo, hi
}

// nicePriceTicks returns label prices on a 1-2-5 * 10^n grid in
// Ticks, so ES labels land on 6700 / 6702 / 6705, not 6701.37.
// The step is a whole number of ticks; Instrument.Text turns each
// value into display text.
func nicePriceTicks(lo, hi core.Ticks, target int) []core.Ticks {
	if hi < lo {
		lo, hi = hi, lo
	}
	step := niceStep(hi-lo, target)
	start := lo
	if rem := start % step; rem != 0 {
		if start >= 0 {
			start = start - rem + step
		} else {
			start = start - rem
		}
	}
	n := int((hi-start)/step) + 1
	if n < 1 {
		return []core.Ticks{lo}
	}
	out := make([]core.Ticks, 0, n)
	for t := start; t <= hi; t += step {
		out = append(out, t)
	}
	return out
}

// niceStep is the usual 1-2-5 * 10^n choice that keeps labels both
// few enough to read and on a multiple of one tick.
func niceStep(rng core.Ticks, target int) core.Ticks {
	if rng <= 0 {
		return 1
	}
	if target < 1 {
		target = 5
	}
	raw := float64(rng) / float64(target)
	if raw <= 1 {
		return 1
	}
	exp := math.Pow(10, math.Floor(math.Log10(raw)))
	frac := raw / exp
	var nf float64
	switch {
	case frac <= 1:
		nf = 1
	case frac <= 2:
		nf = 2
	case frac <= 5:
		nf = 5
	default:
		nf = 10
	}
	step := core.Ticks(math.Round(nf * exp))
	if step < 1 {
		return 1
	}
	return step
}
