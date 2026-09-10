package chart

import (
	"fmt"
	"io"
	"math"

	"github.com/morteza-sakifard/TickEngine/internal/core"
)

const imbColor = "#eab308"

func hasFootprint(v View) bool {
	return v.Footprint != nil && len(v.Footprint.Bars) > 0
}

func writeFootprint(w io.Writer, fp *FootprintView, sc Scale) {
	if fp == nil {
		return
	}
	step := fp.TicksPerRow
	if step < 1 {
		step = 1
	}
	h := math.Abs(sc.Y(0) - sc.Y(core.Ticks(step)))
	if h < 1 {
		h = 1
	}
	n := len(fp.Bars)
	if n > sc.N {
		n = sc.N
	}
	half := sc.slotWidth() * 0.4
	if half < 1 {
		half = 1
	}
	for i := 0; i < n; i++ {
		bar := fp.Bars[i]
		var maxV core.Qty
		for _, lv := range bar.Levels {
			if lv.Buy > maxV {
				maxV = lv.Buy
			}
			if lv.Sell > maxV {
				maxV = lv.Sell
			}
		}
		if maxV <= 0 {
			continue
		}
		x := sc.X(i)
		for _, lv := range bar.Levels {
			y := sc.Y(lv.Price) - h/2
			if lv.Sell > 0 {
				st, imb := cellMark(bar.Imbs, lv.Price, core.SideBid)
				writeFootCell(w, x-half, y, half, h, downColor, lv.Sell, maxV, st, imb)
			}
			if lv.Buy > 0 {
				st, imb := cellMark(bar.Imbs, lv.Price, core.SideAsk)
				writeFootCell(w, x, y, half, h, upColor, lv.Buy, maxV, st, imb)
			}
		}
	}
}

func writeFootCell(w io.Writer, x, y, width, height float64, color string, vol, max core.Qty, stacked bool, imb bool) {
	op := 0.20 + 0.70*float64(vol)/float64(max)
	fmt.Fprintf(w, "  <rect x=\"%.1f\" y=\"%.1f\" width=\"%.1f\" height=\"%.1f\" fill=\"%s\" fill-opacity=\"%.2f\"",
		x, y, width, height, color, op)
	if imb {
		sw := 1.5
		if stacked {
			sw = 2.5
		}
		fmt.Fprintf(w, " stroke=\"%s\" stroke-width=\"%.1f\"", imbColor, sw)
	}
	io.WriteString(w, "/>\n")
}

func cellMark(imbs []FootprintImb, px core.Ticks, dir core.Side) (stacked bool, imb bool) {
	for _, im := range imbs {
		if im.Price == px && im.Dir == dir {
			return im.Stacked, true
		}
	}
	return false, false
}
