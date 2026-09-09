package chart

import (
	"encoding/xml"
	"fmt"
	"io"
	"math"

	"github.com/morteza-sakifard/market-data-lab/internal/core"
)

const (
	domWidth   = 80.0
	heatBidCol = "#2563eb"
	heatAskCol = "#dc2626"
	domLastCol = "#d97706"
)

// HeatmapView is resting size by bar index and grouped price. A nil
// or empty value leaves the step-8 candle layout unchanged.
// Columns is aligned with View.Bars; extra bars are skipped.
type HeatmapView struct {
	TicksPerRow int
	Columns     []HeatColumn
}

// HeatColumn is one time slot. Cells are already grouped and sorted.
type HeatColumn struct {
	Cells []HeatCell
}

// HeatCell is one grouped price. Bid/Ask are resting size, not trades.
type HeatCell struct {
	Price    core.Ticks
	Bid, Ask core.Qty
}

// DOMView is the live ladder. Levels are high price first.
type DOMView struct {
	Levels  []DOMLevel
	LastPx  core.Ticks
	LastQty core.Qty
}

// DOMLevel is one ladder row: bid size, ask size, last print at Price.
type DOMLevel struct {
	Price    core.Ticks
	Bid, Ask core.Qty
	Last     core.Qty
}

func hasHeatmap(v View) bool {
	if v.Heatmap == nil {
		return false
	}
	for _, col := range v.Heatmap.Columns {
		if len(col.Cells) > 0 {
			return true
		}
	}
	return false
}

func hasDOM(v View) bool {
	return v.DOM != nil && len(v.DOM.Levels) > 0
}

func writeHeatmap(w io.Writer, hm *HeatmapView, sc Scale) {
	if hm == nil {
		return
	}
	step := hm.TicksPerRow
	if step < 1 {
		step = 1
	}
	h := math.Abs(sc.Y(0) - sc.Y(core.Ticks(step)))
	if h < 1 {
		h = 1
	}
	var maxV core.Qty
	n := len(hm.Columns)
	if n > sc.N {
		n = sc.N
	}
	for i := 0; i < n; i++ {
		for _, c := range hm.Columns[i].Cells {
			if c.Bid > maxV {
				maxV = c.Bid
			}
			if c.Ask > maxV {
				maxV = c.Ask
			}
		}
	}
	if maxV <= 0 {
		return
	}
	half := sc.slotWidth() * 0.45
	if half < 1 {
		half = 1
	}
	for i := 0; i < n; i++ {
		x := sc.X(i)
		for _, c := range hm.Columns[i].Cells {
			y := sc.Y(c.Price) - h/2
			if c.Bid > 0 {
				writeHeatCell(w, x-half, y, half, h, heatBidCol, c.Bid, maxV, c.Price, "bid")
			}
			if c.Ask > 0 {
				writeHeatCell(w, x, y, half, h, heatAskCol, c.Ask, maxV, c.Price, "ask")
			}
		}
	}
}

func writeHeatCell(w io.Writer, x, y, width, height float64, color string, vol, max core.Qty, px core.Ticks, side string) {
	op := 0.12 + 0.70*float64(vol)/float64(max)
	fmt.Fprintf(w, "  <rect class=\"heat\" data-side=\"%s\" data-px=\"%d\" data-qty=\"%d\" x=\"%.1f\" y=\"%.1f\" width=\"%.1f\" height=\"%.1f\" fill=\"%s\" fill-opacity=\"%.2f\"/>\n",
		side, px, vol, x, y, width, height, color, op)
}

func writeDOM(w io.Writer, d *DOMView, inst core.Instrument, sc Scale, left, right float64) {
	if d == nil || len(d.Levels) == 0 {
		return
	}
	width := right - left
	if width < 1 {
		width = 1
	}
	tickH := math.Abs(sc.Y(0) - sc.Y(1))
	if tickH < 8 {
		tickH = 8
	}
	var maxV core.Qty
	for _, lv := range d.Levels {
		if lv.Bid > maxV {
			maxV = lv.Bid
		}
		if lv.Ask > maxV {
			maxV = lv.Ask
		}
	}
	mid := left + width*0.5
	barW := width * 0.22
	for _, lv := range d.Levels {
		y := sc.Y(lv.Price)
		if maxV > 0 {
			if lv.Bid > 0 {
				wbar := barW * float64(lv.Bid) / float64(maxV)
				if wbar < 1 {
					wbar = 1
				}
				fmt.Fprintf(w, "  <rect class=\"dom\" data-side=\"bid\" data-px=\"%d\" data-qty=\"%d\" x=\"%.1f\" y=\"%.1f\" width=\"%.1f\" height=\"%.1f\" fill=\"%s\" fill-opacity=\"0.45\"/>\n",
					lv.Price, lv.Bid, mid-4-wbar, y-tickH/2, wbar, tickH, heatBidCol)
			}
			if lv.Ask > 0 {
				wbar := barW * float64(lv.Ask) / float64(maxV)
				if wbar < 1 {
					wbar = 1
				}
				fmt.Fprintf(w, "  <rect class=\"dom\" data-side=\"ask\" data-px=\"%d\" data-qty=\"%d\" x=\"%.1f\" y=\"%.1f\" width=\"%.1f\" height=\"%.1f\" fill=\"%s\" fill-opacity=\"0.45\"/>\n",
					lv.Price, lv.Ask, mid+4, y-tickH/2, wbar, tickH, heatAskCol)
			}
		}
		if lv.Last > 0 {
			fmt.Fprintf(w, "  <circle class=\"dom\" data-side=\"last\" data-px=\"%d\" data-qty=\"%d\" cx=\"%.1f\" cy=\"%.1f\" r=\"2.5\" fill=\"%s\"/>\n",
				lv.Price, lv.Last, mid, y, domLastCol)
		}
		fmt.Fprintf(w, "  <text class=\"dom-px\" x=\"%.1f\" y=\"%.1f\" font-size=\"8\" fill=\"%s\" text-anchor=\"middle\">",
			mid, y+3, inkColor)
		_ = xml.EscapeText(w, []byte(inst.Text(lv.Price)))
		io.WriteString(w, "</text>\n")
	}
}
