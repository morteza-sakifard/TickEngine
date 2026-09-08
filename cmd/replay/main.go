// Command replay prints a Databento MBP-1 CSV through replay.Engine
// with optional speed and step. The pacer changes only how long the
// process waits; the event log is the same at every speed. See
// docs/01-roadmap.md step 15.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"strconv"
	"time"

	"github.com/morteza-sakifard/market-data-lab/internal/core"
	"github.com/morteza-sakifard/market-data-lab/internal/feed"
	"github.com/morteza-sakifard/market-data-lab/internal/feed/databento"
	"github.com/morteza-sakifard/market-data-lab/internal/marketdata"
	"github.com/morteza-sakifard/market-data-lab/internal/replay"
)

func main() {
	log.SetFlags(0)

	var (
		data   = flag.String("data", "", "path to the Databento MBP-1 CSV (required)")
		symbol = flag.String("symbol", "", "contract symbol, e.g. ESZ5 (required)")
		speed  = flag.String("speed", "0", "0 = as fast as possible, 1 = real time, 100 = 100x")
		step   = flag.Bool("step", false, "wait for Enter before each event after the first")
	)
	flag.Parse()

	if *data == "" || *symbol == "" {
		flag.Usage()
		log.Fatal("--data and --symbol are required")
	}
	rate, err := parseSpeed(*speed)
	if err != nil {
		log.Fatal(err)
	}
	inst, err := lookupInstrument(*symbol)
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

	start := time.Now()
	n, err := replaySource(dec, os.Stdout, &replay.Pacer{Speed: rate, Step: *step})
	if err != nil {
		log.Fatal(err)
	}
	elapsed := time.Since(start).Round(time.Millisecond)
	fmt.Fprintf(os.Stderr, "replayed %d events in %s (speed=%s)\n", n, elapsed, *speed)
}

func lookupInstrument(symbol string) (core.Instrument, error) {
	inst := core.ESZ5()
	if symbol != inst.Symbol {
		return core.Instrument{}, fmt.Errorf("replay: unknown symbol %q (only %s is configured)", symbol, inst.Symbol)
	}
	return inst, nil
}

func parseSpeed(s string) (float64, error) {
	v, err := strconv.ParseFloat(s, 64)
	if err != nil || v < 0 || math.IsNaN(v) || math.IsInf(v, 0) {
		return 0, fmt.Errorf("replay: --speed %q: want a number >= 0", s)
	}
	return v, nil
}

func replaySource(src feed.Source, w io.Writer, p *replay.Pacer) (int, error) {
	var n int
	e := replay.New(src)
	e.SetPacer(p)
	e.Subscribe(replay.HandlerFunc(func(ev *marketdata.Event) {
		n++
		fmt.Fprintln(w, formatEvent(ev))
	}))
	return n, e.Run(context.Background())
}

func formatEvent(ev *marketdata.Event) string {
	switch ev.Kind {
	case marketdata.KindTrade:
		return fmt.Sprintf("trade recv=%d seq=%d px=%d qty=%d side=%s%s",
			ev.TsRecv, ev.Sequence, ev.Trade.Px, ev.Trade.Qty, ev.Trade.Aggressor, flagSuffix(ev.Flags))
	case marketdata.KindQuote:
		return fmt.Sprintf("quote recv=%d seq=%d bid=%d ask=%d bq=%d aq=%d%s",
			ev.TsRecv, ev.Sequence, ev.Quote.BidPx, ev.Quote.AskPx, ev.Quote.BidQty, ev.Quote.AskQty, flagSuffix(ev.Flags))
	default:
		return fmt.Sprintf("%s recv=%d seq=%d%s", ev.Kind, ev.TsRecv, ev.Sequence, flagSuffix(ev.Flags))
	}
}

func flagSuffix(f marketdata.Flags) string {
	if f&marketdata.FlagBadTsRecv != 0 {
		return " bad_ts_recv"
	}
	return ""
}
