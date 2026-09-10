package chart

import (
	"encoding/xml"
	"fmt"
	"io"
	"time"

	"github.com/morteza-sakifard/TickEngine/internal/aggregation"
	"github.com/morteza-sakifard/TickEngine/internal/core"
	"github.com/morteza-sakifard/TickEngine/internal/session"
)

const (
	defaultWidth  = 800
	defaultHeight = 480

	padL = 16.0
	padR = 72.0
	padT = 32.0
	padB = 28.0

	upColor      = "#1b7f3a"
	downColor    = "#b42318"
	gridColor    = "#e6e6e6"
	inkColor     = "#222222"
	overlayColor = "#1d4ed8"
	panelColor   = "#0f766e"
	zeroColor    = "#9ca3af"
)

// RenderSVG writes v as SVG. It is a pure function: no time.Now, no
// file I/O, no map ranging. Two calls with the same View and Options
// are byte-identical, which is what the golden file locks.
func RenderSVG(w io.Writer, v View, o Options) error {
	width, height := o.Width, o.Height
	if width == 0 {
		width = defaultWidth
	}
	if height == 0 {
		height = defaultHeight
	}
	if width < 0 || height < 0 {
		return fmt.Errorf("chart: width and height must be non-negative")
	}

	left, outerRight := padL, float64(width)-padR
	top, bottom := padT, float64(height)-padB
	if outerRight <= left {
		outerRight = left + 1
	}
	if bottom <= top {
		bottom = top + 1
	}
	priceRight := outerRight
	if hasProfile(v) {
		priceRight -= profileWidth
	}
	if hasTPO(v) {
		priceRight -= tpoWidth
	}
	if hasDOM(v) {
		priceRight -= domWidth
	}
	if priceRight <= left {
		priceRight = left + 1
	}
	priceBottom := bottom
	if n := len(v.Panels); n > 0 {
		priceBottom = bottom - (bottom-top)*0.24
		if priceBottom <= top {
			priceBottom = top + 1
		}
	}
	sc := newScale(v.Bars, left, priceRight, top, priceBottom)
	loc := labelLocation(o, v.Bars)

	if _, err := fmt.Fprintf(w, "<svg xmlns=\"http://www.w3.org/2000/svg\" width=\"%d\" height=\"%d\" viewBox=\"0 0 %d %d\">\n",
		width, height, width, height); err != nil {
		return err
	}
	fmt.Fprintf(w, "  <rect width=\"%d\" height=\"%d\" fill=\"#ffffff\"/>\n", width, height)

	writeHeader(w, v, loc)
	writePriceGrid(w, v.Instrument, sc, outerRight+8)
	if hasHeatmap(v) {
		writeHeatmap(w, v.Heatmap, sc)
	}
	if hasFootprint(v) {
		writeFootprint(w, v.Footprint, sc)
	}
	for i, b := range v.Bars {
		if hasFootprint(v) {
			writeWick(w, sc, i, b)
		} else {
			writeCandle(w, sc, i, b)
		}
	}
	for _, s := range v.Overlays {
		writePolyline(w, sc, s, overlayColor)
	}
	writeMarks(w, v.Marks, sc)
	side := priceRight
	if hasDOM(v) {
		writeDOM(w, v.DOM, v.Instrument, sc, side, side+domWidth)
		side += domWidth
	}
	if hasTPO(v) {
		writeTPO(w, v.TPO, sc, side, side+tpoWidth)
		side += tpoWidth
	}
	if hasProfile(v) {
		writeProfile(w, v.Profile, sc, side, outerRight)
	}
	axisY := sc.Bottom + 16
	if len(v.Panels) > 0 {
		writePanels(w, v.Panels, sc.N, left, priceRight, priceBottom, bottom)
		axisY = bottom + 16
	}
	writeTimeAxis(w, v.Bars, sc, loc, axisY)
	_, err := io.WriteString(w, "</svg>\n")
	return err
}

func labelLocation(o Options, bars []aggregation.Bar) *time.Location {
	if o.Location != nil {
		return o.Location
	}
	if len(bars) > 0 {
		return bars[0].Start.Location()
	}
	return time.UTC
}

func writeHeader(w io.Writer, v View, loc *time.Location) {
	sym := v.Header.Symbol
	if sym == "" {
		sym = v.Instrument.Symbol
	}
	td := v.Header.TradingDate
	if td.IsZero() && len(v.Bars) > 0 {
		td = v.Bars[0].TradingDate
	}
	date := ""
	if !td.IsZero() {
		date = td.In(loc).Format("2006-01-02")
	}
	sess := v.Header.Session
	if sess == session.Closed && len(v.Bars) > 0 {
		sess = v.Bars[0].Session
	}
	text := sym
	if date != "" {
		text += "  " + date
	}
	if sess != session.Closed {
		text += "  " + sess.String()
	}
	writeText(w, padL, 20, 13, text)
}

func writePriceGrid(w io.Writer, inst core.Instrument, sc Scale, labelX float64) {
	ticks := nicePriceTicks(sc.YMin, sc.YMax, 6)
	for _, px := range ticks {
		y := sc.Y(px)
		fmt.Fprintf(w, "  <line x1=\"%.1f\" y1=\"%.1f\" x2=\"%.1f\" y2=\"%.1f\" stroke=\"%s\" stroke-width=\"1\"/>\n",
			sc.Left, y, sc.Right, y, gridColor)
		label := formatPrice(inst, px)
		writeText(w, labelX, y+4, 11, label)
	}
}

func formatPrice(inst core.Instrument, px core.Ticks) string {
	if inst.TickSizeNano > 0 {
		return inst.Text(px)
	}
	return fmt.Sprintf("%d", px)
}

func writeCandle(w io.Writer, sc Scale, i int, b aggregation.Bar) {
	writeWick(w, sc, i, b)
	x := sc.X(i)
	yO, yC := sc.Y(b.Open), sc.Y(b.Close)
	color := upColor
	if b.Close < b.Open {
		color = downColor
	}
	bodyTop, bodyBot := yO, yC
	if bodyTop > bodyBot {
		bodyTop, bodyBot = bodyBot, bodyTop
	}
	h := bodyBot - bodyTop
	if h < 1 {
		h = 1
	}
	hw := sc.slotWidth() * 0.3
	fmt.Fprintf(w, "  <rect x=\"%.1f\" y=\"%.1f\" width=\"%.1f\" height=\"%.1f\" fill=\"%s\"/>\n",
		x-hw, bodyTop, hw*2, h, color)
}

func writeWick(w io.Writer, sc Scale, i int, b aggregation.Bar) {
	x := sc.X(i)
	color := upColor
	if b.Close < b.Open {
		color = downColor
	}
	fmt.Fprintf(w, "  <line x1=\"%.1f\" y1=\"%.1f\" x2=\"%.1f\" y2=\"%.1f\" stroke=\"%s\" stroke-width=\"1\"/>\n",
		x, sc.Y(b.High), x, sc.Y(b.Low), color)
}

func writeTimeAxis(w io.Writer, bars []aggregation.Bar, sc Scale, loc *time.Location, y float64) {
	if len(bars) == 0 {
		return
	}
	step := 1
	if n := len(bars); n > 12 {
		step = (n + 7) / 8
	}
	for i := 0; i < len(bars); i += step {
		label := bars[i].Start.In(loc).Format("15:04")
		writeText(w, sc.X(i)-14, y, 11, label)
	}
}

func writePolyline(w io.Writer, sc Scale, s Series, color string) {
	n := len(s.Values)
	if n == 0 || sc.N == 0 {
		return
	}
	if n > sc.N {
		n = sc.N
	}
	fmt.Fprintf(w, "  <polyline fill=\"none\" stroke=\"%s\" stroke-width=\"1.5\" points=\"", color)
	for i := 0; i < n; i++ {
		if i > 0 {
			io.WriteString(w, " ")
		}
		fmt.Fprintf(w, "%.1f,%.1f", sc.X(i), sc.Y(s.Values[i]))
	}
	io.WriteString(w, "\"/>\n")
}

func writePanels(w io.Writer, panels []Panel, nBars int, left, right, top, bottom float64) {
	n := len(panels)
	if n == 0 {
		return
	}
	gap := 10.0
	avail := bottom - top - gap
	if avail < 1 {
		avail = 1
	}
	each := avail / float64(n)
	y := top + gap
	for _, p := range panels {
		writePanel(w, p, nBars, left, right, y, y+each-gap)
		y += each
	}
}

func writePanel(w io.Writer, p Panel, nBars int, left, right, top, bottom float64) {
	fmt.Fprintf(w, "  <line x1=\"%.1f\" y1=\"%.1f\" x2=\"%.1f\" y2=\"%.1f\" stroke=\"%s\" stroke-width=\"1\"/>\n",
		left, top, right, top, gridColor)
	if p.Name != "" {
		writeText(w, left, top+12, 11, p.Name)
	}
	sc := Scale{
		YMin: panelLow(p.Series), YMax: panelHigh(p.Series),
		Left: left, Right: right,
		Top: top + 16, Bottom: bottom,
		N: nBars,
	}
	if sc.YMin < 0 && sc.YMax > 0 {
		y0 := sc.Y(0)
		fmt.Fprintf(w, "  <line x1=\"%.1f\" y1=\"%.1f\" x2=\"%.1f\" y2=\"%.1f\" stroke=\"%s\" stroke-width=\"1\"/>\n",
			left, y0, right, y0, zeroColor)
	}
	for _, s := range p.Series {
		writePolyline(w, sc, s, panelColor)
	}
}

func panelLow(series []Series) core.Ticks {
	lo, _, ok := panelMinMax(series)
	if !ok {
		return -1
	}
	if lo > 0 {
		return 0
	}
	return lo
}

func panelHigh(series []Series) core.Ticks {
	_, hi, ok := panelMinMax(series)
	if !ok {
		return 1
	}
	if hi < 0 {
		return 0
	}
	if hi == panelLow(series) {
		return hi + 1
	}
	return hi
}

func panelMinMax(series []Series) (lo, hi core.Ticks, ok bool) {
	for _, s := range series {
		for _, v := range s.Values {
			if !ok {
				lo, hi, ok = v, v, true
				continue
			}
			if v < lo {
				lo = v
			}
			if v > hi {
				hi = v
			}
		}
	}
	return lo, hi, ok
}

func writeText(w io.Writer, x, y, size float64, s string) {
	fmt.Fprintf(w, "  <text x=\"%.1f\" y=\"%.1f\" font-family=\"monospace\" font-size=\"%.0f\" fill=\"%s\">",
		x, y, size, inkColor)
	_ = xml.EscapeText(w, []byte(s))
	io.WriteString(w, "</text>\n")
}
