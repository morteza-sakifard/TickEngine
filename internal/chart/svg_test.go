package chart

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/morteza-sakifard/market-data-lab/internal/aggregation"
	"github.com/morteza-sakifard/market-data-lab/internal/core"
	"github.com/morteza-sakifard/market-data-lab/internal/session"
)

var updateGolden = flag.Bool("update", false, "write testdata/golden/candles_basic.svg")

const goldenPath = "../../testdata/golden/candles_basic.svg"

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
