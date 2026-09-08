// Command render turns a Databento MBP-1 CSV into an SVG chart for
// one trading date and session. It wires feed → aggregation →
// orderflow → chart and does not live in internal/: cmd is allowed
// to import every layer. See docs/01-roadmap.md steps 9 and 10.
package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	// tzdata embeds IANA zones so America/Chicago works on Windows
	// binaries that have left this GOROOT.
	_ "time/tzdata"

	"github.com/morteza-sakifard/market-data-lab/internal/aggregation"
	"github.com/morteza-sakifard/market-data-lab/internal/chart"
	"github.com/morteza-sakifard/market-data-lab/internal/core"
	"github.com/morteza-sakifard/market-data-lab/internal/feed"
	"github.com/morteza-sakifard/market-data-lab/internal/feed/databento"
	"github.com/morteza-sakifard/market-data-lab/internal/marketdata"
	"github.com/morteza-sakifard/market-data-lab/internal/orderflow"
	"github.com/morteza-sakifard/market-data-lab/internal/session"
)

func main() {
	log.SetFlags(0)

	var (
		data     = flag.String("data", "", "path to the Databento MBP-1 CSV (required)")
		symbol   = flag.String("symbol", "", "contract symbol, e.g. ESZ5 (required)")
		date     = flag.String("date", "", "trading date YYYY-MM-DD in the product calendar (required)")
		sessName = flag.String("session", "", "RTH or ETH (required)")
		interval = flag.String("interval", "", "time-bar width: 5m, 15m, 30m, 1h, 4h, 1d (required)")
		out      = flag.String("out", "", "output SVG path (required)")
	)
	flag.Parse()

	if *data == "" || *symbol == "" || *date == "" || *sessName == "" || *interval == "" || *out == "" {
		flag.Usage()
		log.Fatal("--data, --symbol, --date, --session, --interval, and --out are required")
	}

	inst, err := lookupInstrument(*symbol)
	if err != nil {
		log.Fatal(err)
	}
	cal, err := session.ForProduct(inst.Product)
	if err != nil {
		log.Fatal(err)
	}
	day, err := parseDate(*date, cal.Location)
	if err != nil {
		log.Fatal(err)
	}
	sess, set, anchor, err := parseSession(*sessName)
	if err != nil {
		log.Fatal(err)
	}
	iv, err := parseInterval(*interval)
	if err != nil {
		log.Fatal(err)
	}

	f, err := os.Open(*data)
	if err != nil {
		log.Fatal(err)
	}
	dec, err := databento.NewDecoder(f, inst)
	if err != nil {
		f.Close()
		log.Fatal(err)
	}
	defer dec.Close()

	outf, err := os.Create(*out)
	if err != nil {
		log.Fatal(err)
	}
	defer outf.Close()

	start := time.Now()
	n, err := render(dec, outf, inst, cal, spec{
		Date:     day,
		Session:  sess,
		Sessions: set,
		Anchor:   anchor,
		Interval: iv,
	})
	if err != nil {
		log.Fatal(err)
	}
	elapsed := time.Since(start).Round(time.Millisecond)
	fmt.Printf("wrote %s (%d bars) in %s\n", *out, n, elapsed)
}

type spec struct {
	Date     time.Time
	Session  session.Session
	Sessions session.Set
	Anchor   aggregation.Anchor
	Interval time.Duration
}

func lookupInstrument(symbol string) (core.Instrument, error) {
	inst := core.ESZ5()
	if symbol != inst.Symbol {
		return core.Instrument{}, fmt.Errorf("render: unknown symbol %q (only %s is configured)", symbol, inst.Symbol)
	}
	return inst, nil
}

func parseDate(s string, loc *time.Location) (time.Time, error) {
	t, err := time.ParseInLocation("2006-01-02", s, loc)
	if err != nil {
		return time.Time{}, fmt.Errorf("render: --date %q: want YYYY-MM-DD", s)
	}
	return t, nil
}

func parseSession(s string) (session.Session, session.Set, aggregation.Anchor, error) {
	switch s {
	case "RTH":
		return session.RTH, session.SetRTH, aggregation.AnchorRTHOpen, nil
	case "ETH":
		return session.ETH, session.SetETH, aggregation.AnchorSessionOpen, nil
	default:
		return session.Closed, 0, 0, fmt.Errorf("render: --session %q: want RTH or ETH", s)
	}
}

func parseInterval(s string) (time.Duration, error) {
	if s == "1d" {
		return 24 * time.Hour, nil
	}
	d, err := time.ParseDuration(s)
	if err != nil || d <= 0 {
		return 0, fmt.Errorf("render: --interval %q: want 5m, 15m, 30m, 1h, 4h, or 1d", s)
	}
	return d, nil
}

func sameCivil(a, b time.Time) bool {
	y1, m1, d1 := a.Date()
	y2, m2, d2 := b.Date()
	return y1 == y2 && m1 == m2 && d1 == d2
}

// render reads src, keeps trades whose Classify trading date and
// session match the spec, aggregates them, samples VWAP/CVD at each
// bar, builds the session volume profile and per-bar footprints, and
// writes an SVG to w. Quotes are ignored: a bar is
// executions, not book updates. Zero matching trades is an error —
// a blank chart would hide a wrong --date or --session rather than
// say so. The decoder reuses one Event, so each kept trade is copied
// before it is stored.
func render(src feed.Source, w io.Writer, inst core.Instrument, cal session.Calendar, s spec) (int, error) {
	agg, err := aggregation.New(aggregation.BarSpec{
		Kind:     aggregation.KindTime,
		Interval: s.Interval,
		Anchor:   s.Anchor,
		Location: cal.Location,
		Sessions: s.Sessions,
	}, cal)
	if err != nil {
		return 0, err
	}

	var trades []marketdata.Event
	var ev marketdata.Event
	for {
		err := src.Next(&ev)
		if err == io.EOF {
			break
		}
		if err != nil {
			return 0, err
		}
		if ev.Kind != marketdata.KindTrade {
			continue
		}
		td, sess := cal.Classify(ev.EventTime())
		if !sameCivil(td, s.Date) || !s.Sessions.Contains(sess) {
			continue
		}
		kept := ev
		trades = append(trades, kept)
		agg.Add(&kept)
	}
	agg.Flush()
	bars := agg.Bars()
	if len(bars) == 0 {
		return 0, fmt.Errorf("render: no trades for %s %s %s",
			inst.Symbol, s.Date.Format("2006-01-02"), s.Session)
	}

	vwap, cvd := sampleFlow(trades, bars, cal)
	var vp orderflow.VolumeProfile
	for i := range trades {
		vp.OnTrade(&trades[i])
	}
	view := chart.View{
		Instrument: inst,
		Header: chart.Header{
			Symbol:      inst.Symbol,
			TradingDate: s.Date,
			Session:     s.Session,
		},
		Bars:     bars,
		Overlays: []chart.Series{{Name: "VWAP", Values: vwap}},
		Panels: []chart.Panel{{
			Name:   "CVD",
			Series: []chart.Series{{Name: "CVD", Values: cvd}},
		}},
		Profile:   snapshotProfile(&vp),
		Footprint: snapshotFootprints(trades, bars),
	}
	if err := chart.RenderSVG(w, view, chart.Options{Location: cal.Location}); err != nil {
		return 0, err
	}
	return len(bars), nil
}

// sampleFlow walks trades in TsEvent order, resets on session
// boundaries, and snapshots VWAP and CVD after the last trade of
// each bar. Sampling lives here because orderflow must not import
// aggregation.
func sampleFlow(trades []marketdata.Event, bars []aggregation.Bar, cal session.Calendar) (vwap, cvd []core.Ticks) {
	vwap = make([]core.Ticks, len(bars))
	cvd = make([]core.Ticks, len(bars))
	if len(bars) == 0 {
		return vwap, cvd
	}
	from, to := bars[0].Start, bars[len(bars)-1].End
	if len(trades) > 0 {
		if t0 := trades[0].EventTime(); t0.Before(from) {
			from = t0
		}
		if t1 := trades[len(trades)-1].EventTime(); !t1.Before(to) {
			to = t1.Add(time.Nanosecond)
		}
	}
	var accVWAP orderflow.VWAP
	var accCVD orderflow.CVD
	o := orderflow.New(cal.Boundaries(from, to), &accVWAP, &accCVD)
	j := 0
	for i := range bars {
		end := bars[i].End
		for j < len(trades) && trades[j].EventTime().Before(end) {
			o.OnEvent(&trades[j])
			j++
		}
		vwap[i] = accVWAP.Value()
		cvd[i] = core.Ticks(accCVD.Value())
	}
	return vwap, cvd
}

// snapshotProfile copies a session profile into the chart DTO.
// Chart does not import orderflow: Levels is already sorted, so
// RenderSVG never ranges a map.
func snapshotProfile(vp *orderflow.VolumeProfile) *chart.ProfileView {
	levels := vp.Levels()
	if len(levels) == 0 {
		return nil
	}
	val, poc, vah := vp.ValueArea()
	out := make([]chart.ProfileLevel, len(levels))
	for i, lv := range levels {
		out[i] = chart.ProfileLevel{Price: lv.Price, Volume: lv.Volume}
	}
	return &chart.ProfileView{Levels: out, POC: poc, VAL: val, VAH: vah}
}

const (
	footprintTicksPerRow = 4
	footprintRatio       = 3
	footprintMinStack    = 3
)

// snapshotFootprints builds one grouped footprint per bar from the
// same trades that made the bar. Sampling lives here because
// orderflow must not import aggregation.
func snapshotFootprints(trades []marketdata.Event, bars []aggregation.Bar) *chart.FootprintView {
	out := make([]chart.BarFootprint, len(bars))
	j := 0
	for i := range bars {
		var fp orderflow.Footprint
		end := bars[i].End
		for j < len(trades) && trades[j].EventTime().Before(end) {
			fp.OnTrade(&trades[j])
			j++
		}
		out[i] = snapshotBarFootprint(&fp)
	}
	return &chart.FootprintView{TicksPerRow: footprintTicksPerRow, Bars: out}
}

func snapshotBarFootprint(fp *orderflow.Footprint) chart.BarFootprint {
	levels := fp.Grouped(footprintTicksPerRow)
	cells := make([]chart.FootprintLevel, len(levels))
	for i, lv := range levels {
		cells[i] = chart.FootprintLevel{Price: lv.Price, Buy: lv.Buy, Sell: lv.Sell}
	}
	stacks := fp.StackedImbalances(footprintMinStack, footprintRatio, footprintTicksPerRow)
	imbs := fp.Imbalances(footprintRatio, footprintTicksPerRow)
	marks := make([]chart.FootprintImb, len(imbs))
	for i, im := range imbs {
		marks[i] = chart.FootprintImb{
			Price:   im.Price,
			Dir:     im.Dir,
			Stacked: inStack(stacks, im.Price, im.Dir, footprintTicksPerRow),
		}
	}
	return chart.BarFootprint{Levels: cells, Imbs: marks}
}

func inStack(stacks []orderflow.Stack, px core.Ticks, dir core.Side, n int) bool {
	step := core.Ticks(n)
	for _, s := range stacks {
		if s.Dir != dir || px < s.From || px > s.To {
			continue
		}
		if step > 0 && (px-s.From)%step == 0 {
			return true
		}
	}
	return false
}
