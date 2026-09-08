// Command render turns a Databento MBP-1 CSV into an SVG chart for
// one trading date and session. It wires feed → aggregation → chart
// and does not live in internal/: cmd is allowed to import every
// layer. See docs/01-roadmap.md step 9.
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

// render reads src, keeps trades whose Classify trading date matches
// spec.Date, aggregates them, and writes an SVG to w. Quotes are
// ignored: a bar is executions, not book updates. Zero matching
// trades is an error — a blank chart would hide a wrong --date or
// --session rather than say so.
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
		td, _ := cal.Classify(ev.EventTime())
		if !sameCivil(td, s.Date) {
			continue
		}
		agg.Add(&ev)
	}
	agg.Flush()
	bars := agg.Bars()
	if len(bars) == 0 {
		return 0, fmt.Errorf("render: no trades for %s %s %s",
			inst.Symbol, s.Date.Format("2006-01-02"), s.Session)
	}

	view := chart.View{
		Instrument: inst,
		Header: chart.Header{
			Symbol:      inst.Symbol,
			TradingDate: s.Date,
			Session:     s.Session,
		},
		Bars: bars,
	}
	if err := chart.RenderSVG(w, view, chart.Options{Location: cal.Location}); err != nil {
		return 0, err
	}
	return len(bars), nil
}
