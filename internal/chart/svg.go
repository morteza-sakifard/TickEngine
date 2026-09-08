package chart

import (
	"encoding/xml"
	"fmt"
	"io"
	"time"

	"github.com/morteza-sakifard/market-data-lab/internal/aggregation"
	"github.com/morteza-sakifard/market-data-lab/internal/core"
	"github.com/morteza-sakifard/market-data-lab/internal/session"
)

const (
	defaultWidth  = 800
	defaultHeight = 480

	padL = 16.0
	padR = 72.0
	padT = 32.0
	padB = 28.0

	upColor   = "#1b7f3a"
	downColor = "#b42318"
	gridColor = "#e6e6e6"
	inkColor  = "#222222"
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

	left, right := padL, float64(width)-padR
	top, bottom := padT, float64(height)-padB
	if right <= left {
		right = left + 1
	}
	if bottom <= top {
		bottom = top + 1
	}
	sc := newScale(v.Bars, left, right, top, bottom)
	loc := labelLocation(o, v.Bars)

	if _, err := fmt.Fprintf(w, "<svg xmlns=\"http://www.w3.org/2000/svg\" width=\"%d\" height=\"%d\" viewBox=\"0 0 %d %d\">\n",
		width, height, width, height); err != nil {
		return err
	}
	fmt.Fprintf(w, "  <rect width=\"%d\" height=\"%d\" fill=\"#ffffff\"/>\n", width, height)

	writeHeader(w, v, loc)
	writePriceGrid(w, v.Instrument, sc)
	for i, b := range v.Bars {
		writeCandle(w, sc, i, b)
	}
	writeTimeAxis(w, v.Bars, sc, loc)
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

func writePriceGrid(w io.Writer, inst core.Instrument, sc Scale) {
	ticks := nicePriceTicks(sc.YMin, sc.YMax, 6)
	for _, px := range ticks {
		y := sc.Y(px)
		fmt.Fprintf(w, "  <line x1=\"%.1f\" y1=\"%.1f\" x2=\"%.1f\" y2=\"%.1f\" stroke=\"%s\" stroke-width=\"1\"/>\n",
			sc.Left, y, sc.Right, y, gridColor)
		label := formatPrice(inst, px)
		writeText(w, sc.Right+8, y+4, 11, label)
	}
}

func formatPrice(inst core.Instrument, px core.Ticks) string {
	if inst.TickSizeNano > 0 {
		return inst.Text(px)
	}
	return fmt.Sprintf("%d", px)
}

func writeCandle(w io.Writer, sc Scale, i int, b aggregation.Bar) {
	x := sc.X(i)
	yH, yL := sc.Y(b.High), sc.Y(b.Low)
	yO, yC := sc.Y(b.Open), sc.Y(b.Close)
	color := upColor
	if b.Close < b.Open {
		color = downColor
	}
	fmt.Fprintf(w, "  <line x1=\"%.1f\" y1=\"%.1f\" x2=\"%.1f\" y2=\"%.1f\" stroke=\"%s\" stroke-width=\"1\"/>\n",
		x, yH, x, yL, color)
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

func writeTimeAxis(w io.Writer, bars []aggregation.Bar, sc Scale, loc *time.Location) {
	if len(bars) == 0 {
		return
	}
	step := 1
	if n := len(bars); n > 12 {
		step = (n + 7) / 8
	}
	y := sc.Bottom + 16
	for i := 0; i < len(bars); i += step {
		label := bars[i].Start.In(loc).Format("15:04")
		writeText(w, sc.X(i)-14, y, 11, label)
	}
}

func writeText(w io.Writer, x, y, size float64, s string) {
	fmt.Fprintf(w, "  <text x=\"%.1f\" y=\"%.1f\" font-family=\"monospace\" font-size=\"%.0f\" fill=\"%s\">",
		x, y, size, inkColor)
	_ = xml.EscapeText(w, []byte(s))
	io.WriteString(w, "</text>\n")
}
