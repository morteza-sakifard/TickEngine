package chart

import (
	"encoding/xml"
	"fmt"
	"io"
)

const (
	tpoWidth  = 100.0
	tpoIBFill = "#fef3c7"
	tpoSingle = "#dc2626"
)

func hasTPO(v View) bool {
	return v.TPO != nil && len(v.TPO.Levels) > 0
}

func writeTPO(w io.Writer, p *TPOView, sc Scale, left, right float64) {
	if p == nil || len(p.Levels) == 0 {
		return
	}
	width := right - left
	if width < 1 {
		width = 1
	}
	fmt.Fprintf(w, "  <clipPath id=\"tpo-clip\"><rect x=\"%.1f\" y=\"%.1f\" width=\"%.1f\" height=\"%.1f\"/></clipPath>\n",
		left, sc.Top, width, sc.Bottom-sc.Top)
	if p.HasIB {
		y1, y2 := sc.Y(p.IBHigh), sc.Y(p.IBLow)
		if y1 > y2 {
			y1, y2 = y2, y1
		}
		h := y2 - y1
		if h < 1 {
			h = 1
		}
		fmt.Fprintf(w, "  <rect x=\"%.1f\" y=\"%.1f\" width=\"%.1f\" height=\"%.1f\" fill=\"%s\"/>\n",
			left, y1, width, h, tpoIBFill)
	}
	io.WriteString(w, "  <g clip-path=\"url(#tpo-clip)\">\n")
	for _, lv := range p.Levels {
		color := inkColor
		if lv.Price == p.POC {
			color = pocColor
		} else if lv.Single {
			color = tpoSingle
		}
		fmt.Fprintf(w, "    <text x=\"%.1f\" y=\"%.1f\" font-family=\"monospace\" font-size=\"8\" fill=\"%s\">",
			left+2, sc.Y(lv.Price)+3, color)
		_ = xml.EscapeText(w, []byte(lv.Letters))
		io.WriteString(w, "</text>\n")
	}
	io.WriteString(w, "  </g>\n")
}
