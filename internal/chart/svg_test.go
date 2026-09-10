package chart

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/morteza-sakifard/TickEngine/internal/aggregation"
	"github.com/morteza-sakifard/TickEngine/internal/core"
	"github.com/morteza-sakifard/TickEngine/internal/session"
)

var updateGolden = flag.Bool("update", false, "write data/test/golden/candles_basic.svg")

const goldenPath = "../../data/test/golden/candles_basic.svg"

func chicago(t *testing.T) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation("America/Chicago")
	if err != nil {
		t.Fatal(err)
	}
	return loc
}

func ct(t *testing.T, hour, min int) time.Time {
	t.Helper()
	return time.Date(2025, time.September, 23, hour, min, 0, 0, chicago(t))
}

func bar(t *testing.T, hour, min int, o, h, l, c int64) aggregation.Bar {
	t.Helper()
	start := ct(t, hour, min)
	return aggregation.Bar{
		Start:       start,
		End:         start.Add(30 * time.Minute),
		TradingDate: time.Date(2025, time.September, 23, 0, 0, 0, 0, chicago(t)),
		Session:     session.RTH,
		Open:        core.Ticks(o),
		High:        core.Ticks(h),
		Low:         core.Ticks(l),
		Close:       core.Ticks(c),
		Volume:      1,
	}
}

func basicView(t *testing.T) View {
	t.Helper()
	return View{
		Instrument: core.ESZ5(),
		Header: Header{
			Symbol:      "ESZ5",
			TradingDate: time.Date(2025, time.September, 23, 0, 0, 0, 0, chicago(t)),
			Session:     session.RTH,
		},
		Bars: []aggregation.Bar{
			bar(t, 8, 30, 26800, 26808, 26796, 26804),
			bar(t, 9, 0, 26804, 26810, 26800, 26802),
			bar(t, 9, 30, 26802, 26820, 26800, 26818),
			bar(t, 10, 0, 26818, 26818, 26806, 26808),
			bar(t, 10, 30, 26808, 26812, 26804, 26810),
			bar(t, 11, 0, 26810, 26814, 26798, 26800),
		},
	}
}

func render(t *testing.T, v View, o Options) string {
	t.Helper()
	var buf bytes.Buffer
	if err := RenderSVG(&buf, v, o); err != nil {
		t.Fatal(err)
	}
	return buf.String()
}

func TestRenderSVGDeterministic(t *testing.T) {
	v := basicView(t)
	o := Options{Width: 800, Height: 480, Location: chicago(t)}
	a, b := render(t, v, o), render(t, v, o)
	if a != b {
		t.Fatal("RenderSVG is not byte-identical across two calls")
	}
}

func TestRenderSVGGolden(t *testing.T) {
	got := render(t, basicView(t), Options{Width: 800, Height: 480, Location: chicago(t)})
	if *updateGolden {
		if err := os.MkdirAll(filepath.Dir(goldenPath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(goldenPath, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Logf("wrote %s — inspect it, then rerun without -update", goldenPath)
		return
	}
	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("%v (first run: go test ./internal/chart -run TestRenderSVGGolden -update)", err)
	}
	if got != string(want) {
		t.Errorf("SVG differs from %s; if this change is expected, rerun with -update", goldenPath)
	}
}

func TestTimeAxisUsesCalendarTimezone(t *testing.T) {
	// 13:30 UTC is 08:30 CDT. A UTC label would print 13:30 and fail
	// the acceptance criterion.
	start := time.Date(2025, time.September, 23, 13, 30, 0, 0, time.UTC)
	v := View{
		Instrument: core.ESZ5(),
		Bars: []aggregation.Bar{{
			Start: start, End: start.Add(30 * time.Minute),
			Open: 26800, High: 26804, Low: 26796, Close: 26802,
			Session: session.RTH,
		}},
	}
	svg := render(t, v, Options{Location: chicago(t)})
	if !strings.Contains(svg, "08:30") {
		t.Errorf("time axis missing 08:30 CT label\n%s", svg)
	}
	if strings.Contains(svg, "13:30") {
		t.Errorf("time axis used UTC 13:30 instead of calendar timezone\n%s", svg)
	}
}

func TestClosedGapUsesEqualBarSlots(t *testing.T) {
	// 15:30 and 17:00 are 90 minutes apart through the maintenance
	// break. On a wall-clock x-axis that hole is visible; on a bar-
	// index axis the two steps are the same width.
	v := View{
		Instrument: core.ESZ5(),
		Bars: []aggregation.Bar{
			bar(t, 14, 30, 26800, 26804, 26796, 26802),
			bar(t, 15, 30, 26802, 26806, 26800, 26804),
			bar(t, 17, 0, 26804, 26808, 26800, 26806),
		},
	}
	sc := newScale(v.Bars, 0, 300, 0, 100)
	d01 := sc.X(1) - sc.X(0)
	d12 := sc.X(2) - sc.X(1)
	if d01 != d12 {
		t.Fatalf("slot 0-1 = %v, slot 1-2 = %v; Closed gap must not stretch x", d01, d12)
	}
}

func TestNicePriceTicksAreTickMultiples(t *testing.T) {
	ticks := nicePriceTicks(26790, 26820, 6)
	if len(ticks) < 2 {
		t.Fatalf("got %d ticks, want at least 2", len(ticks))
	}
	step := ticks[1] - ticks[0]
	if step <= 0 {
		t.Fatalf("step = %d, want positive", step)
	}
	for _, px := range ticks {
		if px%step != 0 {
			t.Errorf("tick %d is not a multiple of step %d", px, step)
		}
	}
	// 1-2-5 grid: the step, divided out its 10^n, is 1, 2 or 5.
	n := step
	for n > 0 && n%10 == 0 {
		n /= 10
	}
	if n != 1 && n != 2 && n != 5 {
		t.Errorf("step %d is not on a 1-2-5 grid", step)
	}
}

func TestPriceLabelsUseInstrumentText(t *testing.T) {
	svg := render(t, basicView(t), Options{Location: chicago(t)})
	// 26800 ticks * 0.25 = 6700. Pad/nice grid should still emit a
	// 6700-ish label, not a raw tick integer.
	if strings.Contains(svg, ">26800<") {
		t.Errorf("price axis printed raw Ticks instead of Instrument.Text")
	}
	if !strings.Contains(svg, "6700") {
		t.Errorf("price axis missing a 6700-point label\n%s", svg)
	}
}

func TestRenderSVGEmptyBars(t *testing.T) {
	v := View{Instrument: core.ESZ5(), Header: Header{Symbol: "ESZ5", Session: session.RTH}}
	svg := render(t, v, Options{})
	if !strings.Contains(svg, "<svg") || !strings.HasSuffix(svg, "</svg>\n") {
		t.Fatalf("empty view is not a well-formed SVG")
	}
}

func TestRenderSVGRejectsNegativeSize(t *testing.T) {
	if err := RenderSVG(&bytes.Buffer{}, View{}, Options{Width: -1}); err == nil {
		t.Fatal("want error for negative width")
	}
}

func TestOverlayPolylineDrawn(t *testing.T) {
	v := basicView(t)
	v.Overlays = []Series{{
		Name:   "VWAP",
		Values: []core.Ticks{26800, 26802, 26804, 26810, 26808, 26806},
	}}
	svg := render(t, v, Options{Location: chicago(t)})
	if !strings.Contains(svg, "<polyline") {
		t.Fatal("overlay missing polyline")
	}
	if !strings.Contains(svg, overlayColor) {
		t.Fatalf("overlay missing stroke %s", overlayColor)
	}
}

func TestPanelDrawnWhenPresent(t *testing.T) {
	v := basicView(t)
	v.Panels = []Panel{{
		Name:   "CVD",
		Series: []Series{{Name: "CVD", Values: []core.Ticks{3, 1, 4, 2, -1, 0}}},
	}}
	svg := render(t, v, Options{Location: chicago(t)})
	if !strings.Contains(svg, "CVD") {
		t.Fatal("panel missing CVD label")
	}
	if !strings.Contains(svg, "<polyline") {
		t.Fatal("panel missing polyline")
	}
	if !strings.Contains(svg, zeroColor) {
		t.Fatal("CVD panel missing zero line")
	}
}

func TestProfileDrawnWhenPresent(t *testing.T) {
	v := basicView(t)
	v.Profile = &ProfileView{
		Levels: []ProfileLevel{
			{Price: 26800, Volume: 2},
			{Price: 26808, Volume: 8},
			{Price: 26816, Volume: 3},
		},
		POC: 26808, VAL: 26800, VAH: 26816,
	}
	svg := render(t, v, Options{Location: chicago(t)})
	if !strings.Contains(svg, pocColor) {
		t.Fatal("profile missing POC color")
	}
	if !strings.Contains(svg, "stroke-dasharray") {
		t.Fatal("profile missing POC line")
	}
}

func TestProfilePOCInSessionRange(t *testing.T) {
	v := basicView(t)
	lo, hi := v.Bars[0].Low, v.Bars[0].High
	for _, b := range v.Bars[1:] {
		if b.Low < lo {
			lo = b.Low
		}
		if b.High > hi {
			hi = b.High
		}
	}
	poc := core.Ticks(26808)
	if poc <= lo || poc >= hi {
		t.Fatalf("test POC %d is at the session edge [%d, %d]", poc, lo, hi)
	}
	v.Profile = &ProfileView{
		Levels: []ProfileLevel{
			{Price: lo, Volume: 1},
			{Price: poc, Volume: 9},
			{Price: hi, Volume: 1},
		},
		POC: poc, VAL: lo, VAH: hi,
	}
	sc := newScale(v.Bars, 0, 100, 0, 100)
	y := sc.Y(poc)
	yLo, yHi := sc.Y(hi), sc.Y(lo)
	if yLo > yHi {
		yLo, yHi = yHi, yLo
	}
	if y <= yLo || y >= yHi {
		t.Fatalf("POC y=%.1f at the edge of [%.1f, %.1f]", y, yLo, yHi)
	}
	mid := (yLo + yHi) / 2
	if abs(y-mid) > abs(y-yLo) || abs(y-mid) > abs(y-yHi) {
		t.Fatalf("POC y=%.1f is closer to a range edge than to mid %.1f", y, mid)
	}
}

func abs(f float64) float64 {
	if f < 0 {
		return -f
	}
	return f
}

func TestTPOLettersDrawn(t *testing.T) {
	v := basicView(t)
	v.TPO = &TPOView{
		Levels: []TPOViewLevel{
			{Price: 26800, Letters: "A", Single: true},
			{Price: 26808, Letters: "AB", Single: false},
			{Price: 26816, Letters: "B", Single: true},
		},
		POC: 26808, IBLow: 26800, IBHigh: 26816, HasIB: true, PeriodCount: 2,
	}
	svg := render(t, v, Options{Location: chicago(t)})
	if !strings.Contains(svg, ">AB<") {
		t.Fatal("TPO missing letter string")
	}
	if !strings.Contains(svg, tpoIBFill) {
		t.Fatal("TPO missing initial-balance band")
	}
	if !strings.Contains(svg, "tpo-clip") {
		t.Fatal("TPO missing clip path")
	}
}

func TestFootprintCellsDrawn(t *testing.T) {
	v := basicView(t)
	v.Footprint = &FootprintView{
		TicksPerRow: 1,
		Bars: []BarFootprint{{
			Levels: []FootprintLevel{
				{Price: 26800, Buy: 6, Sell: 2},
				{Price: 26804, Buy: 1, Sell: 3},
			},
			Imbs: []FootprintImb{{Price: 26800, Dir: core.SideAsk}},
		}},
	}
	svg := render(t, v, Options{Location: chicago(t)})
	if !strings.Contains(svg, "fill-opacity") {
		t.Fatal("footprint cells missing volume opacity")
	}
	if !strings.Contains(svg, imbColor) {
		t.Fatal("imbalance cell missing gold stroke")
	}
}

func TestVWAPOverlayYInsideSessionRange(t *testing.T) {
	v := basicView(t)
	lo, hi := v.Bars[0].Low, v.Bars[0].High
	for _, b := range v.Bars[1:] {
		if b.Low < lo {
			lo = b.Low
		}
		if b.High > hi {
			hi = b.High
		}
	}
	vals := []core.Ticks{26800, 26802, 26804, 26810, 26808, 26806}
	for _, px := range vals {
		if px < lo || px > hi {
			t.Fatalf("test VWAP %d outside session [%d, %d]", px, lo, hi)
		}
	}
	v.Overlays = []Series{{Name: "VWAP", Values: vals}}
	sc := newScale(v.Bars, 0, 100, 0, 100)
	yLo, yHi := sc.Y(hi), sc.Y(lo) // SVG y grows down
	if yLo > yHi {
		yLo, yHi = yHi, yLo
	}
	for _, px := range vals {
		y := sc.Y(px)
		if y < yLo || y > yHi {
			t.Fatalf("VWAP y=%.1f for %d outside session y [%.1f, %.1f]", y, px, yLo, yHi)
		}
	}
}
