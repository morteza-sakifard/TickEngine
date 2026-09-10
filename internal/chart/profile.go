package chart

import (
	"fmt"
	"io"
	"math"

	"github.com/morteza-sakifard/TickEngine/internal/core"
)

const (
	profileWidth = 88.0
	vaColor      = "#93c5fd"
	pocColor     = "#d97706"
	outerColor   = "#d1d5db"
)

func hasProfile(v View) bool {
	return v.Profile != nil && len(v.Profile.Levels) > 0
}

func writeProfile(w io.Writer, p *ProfileView, sc Scale, left, right float64) {
	if p == nil || len(p.Levels) == 0 {
		return
	}
	var maxV core.Qty
	for _, lv := range p.Levels {
		if lv.Volume > maxV {
			maxV = lv.Volume
		}
	}
	if maxV <= 0 {
		return
	}
	width := right - left
	if width < 1 {
		width = 1
	}
	tickH := math.Abs(sc.Y(0) - sc.Y(1))
	if tickH < 1 {
		tickH = 1
	}
	for _, lv := range p.Levels {
		barW := width * float64(lv.Volume) / float64(maxV)
		if barW < 1 {
			barW = 1
		}
		color := outerColor
		if lv.Price >= p.VAL && lv.Price <= p.VAH {
			color = vaColor
		}
		if lv.Price == p.POC {
			color = pocColor
		}
		y := sc.Y(lv.Price) - tickH/2
		fmt.Fprintf(w, "  <rect x=\"%.1f\" y=\"%.1f\" width=\"%.1f\" height=\"%.1f\" fill=\"%s\"/>\n",
			right-barW, y, barW, tickH, color)
	}
	yPOC := sc.Y(p.POC)
	fmt.Fprintf(w, "  <line x1=\"%.1f\" y1=\"%.1f\" x2=\"%.1f\" y2=\"%.1f\" stroke=\"%s\" stroke-width=\"1\" stroke-dasharray=\"4 3\"/>\n",
		sc.Left, yPOC, right, yPOC, pocColor)
}
