package chart

import (
	"fmt"
	"io"

	"github.com/morteza-sakifard/TickEngine/internal/aggregation"
	"github.com/morteza-sakifard/TickEngine/internal/core"
)

// Mark is a fill (or signal) on the price pane. Index is a bar
// slot, the same x as candles — not a Unix time. A wall-clock x
// would drop the mark into the Closed hole this project forbids.
type Mark struct {
	Index int
	Px    core.Ticks
	Qty   core.Qty
	Side  core.Side
}

// IndexByTime puts ts on the bar whose [Start, End) contains it.
// Before the first bar → 0; after the last → last. Empty bars → 0.
func IndexByTime(ts int64, bars []aggregation.Bar) int {
	if len(bars) == 0 {
		return 0
	}
	if ts < bars[0].Start.UnixNano() {
		return 0
	}
	for i, b := range bars {
		if ts >= b.Start.UnixNano() && ts < b.End.UnixNano() {
			return i
		}
	}
	return len(bars) - 1
}

func writeMarks(w io.Writer, marks []Mark, sc Scale) {
	for _, m := range marks {
		x, y := sc.X(m.Index), sc.Y(m.Px)
		color := upColor
		if m.Side == core.SideAsk {
			color = downColor
		}
		const r = 5.0
		var points string
		if m.Side == core.SideAsk {
			points = fmt.Sprintf("%.1f,%.1f %.1f,%.1f %.1f,%.1f", x, y, x-r, y-r, x+r, y-r)
		} else {
			points = fmt.Sprintf("%.1f,%.1f %.1f,%.1f %.1f,%.1f", x, y, x-r, y+r, x+r, y+r)
		}
		fmt.Fprintf(w, "  <polygon class=\"mark\" data-side=\"%s\" data-px=\"%d\" data-qty=\"%d\" points=\"%s\" fill=\"%s\"/>\n",
			m.Side, m.Px, m.Qty, points, color)
	}
}
