// Command backtest runs a strategy through a Databento MBP-1 CSV
// with a simulated venue. Market buys fill at AskPx, sells at BidPx.
// See docs/01-roadmap.md step 20.
package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	"github.com/morteza-sakifard/market-data-lab/internal/core"
	"github.com/morteza-sakifard/market-data-lab/internal/execution"
	"github.com/morteza-sakifard/market-data-lab/internal/feed"
	"github.com/morteza-sakifard/market-data-lab/internal/feed/databento"
	"github.com/morteza-sakifard/market-data-lab/internal/marketdata"
	"github.com/morteza-sakifard/market-data-lab/internal/portfolio"
	"github.com/morteza-sakifard/market-data-lab/internal/strategy"
)

func main() {
	log.SetFlags(0)

	var (
		data       = flag.String("data", "", "path to the Databento MBP-1 CSV (required)")
		symbol     = flag.String("symbol", "", "contract symbol, e.g. ESZ5 (required)")
		commission = flag.Int64("commission", 0, "commission per contract per fill, USD cents")
		fee        = flag.Int64("fee", 0, "exchange fee per contract per fill, USD cents")
		entry      = flag.Int64("entry-ns", 0, "order entry latency, nanoseconds")
		response   = flag.Int64("response-ns", 0, "fill response latency, nanoseconds")
	)
	flag.Parse()

	if *data == "" || *symbol == "" {
		flag.Usage()
		log.Fatal("--data and --symbol are required")
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
	pos, unreal, err := runBacktest(dec, inst, execution.Fees{
		CommissionCents: *commission,
		FeeCents:        *fee,
	}, execution.Latency{Entry: *entry, Response: *response}, os.Stdout)
	if err != nil {
		log.Fatal(err)
	}
	elapsed := time.Since(start).Round(time.Millisecond)
	fmt.Fprintf(os.Stderr, "realized=%d unrealized=%d qty=%d in %s\n",
		pos.Realized, unreal, pos.Qty, elapsed)
}

func lookupInstrument(symbol string) (core.Instrument, error) {
	inst := core.ESZ5()
	if symbol != inst.Symbol {
		return core.Instrument{}, fmt.Errorf("backtest: unknown symbol %q (only %s is configured)", symbol, inst.Symbol)
	}
	return inst, nil
}

func runBacktest(src feed.Source, inst core.Instrument, fees execution.Fees, lat execution.Latency, w io.Writer) (portfolio.Position, int64, error) {
	rt := strategy.NewRuntime(inst, w)
	if err := rt.SetFees(fees); err != nil {
		return portfolio.Position{}, 0, err
	}
	if err := rt.SetLatency(lat); err != nil {
		return portfolio.Position{}, 0, err
	}
	if err := strategy.Run(src, &buyHold{}, rt); err != nil {
		return portfolio.Position{}, 0, err
	}
	pos := rt.Position()
	var unreal int64
	if q, ok := rt.Quote(); ok {
		unreal = portfolio.Unrealized(pos, q, inst)
	}
	fmt.Fprintf(w, "realized=%d unrealized=%d qty=%d\n", pos.Realized, unreal, pos.Qty)
	return pos, unreal, nil
}

// buyHold buys one contract on the first quote and flattens on stop.
// It exists so the CLI has a strategy; it is not an edge.
type buyHold struct{ bought bool }

func (s *buyHold) OnStart(strategy.Context) error { return nil }

func (s *buyHold) OnEvent(ctx strategy.Context, ev *marketdata.Event) error {
	if s.bought || ev == nil || ev.Kind != marketdata.KindQuote {
		return nil
	}
	_, err := ctx.Submit(execution.Order{Side: core.SideBid, Qty: 1})
	if err == nil {
		s.bought = true
	}
	return err
}

func (s *buyHold) OnOrder(ctx strategy.Context, ev execution.OrderEvent) error {
	if ev.Status != execution.StatusFilled {
		return nil
	}
	ctx.Logf("fill id=%d side=%s px=%d qty=%d fee=%d\n",
		ev.Fill.OrderID, ev.Fill.Side, ev.Fill.Px, ev.Fill.Qty, ev.Fill.FeeCents)
	return nil
}

func (s *buyHold) OnStop(ctx strategy.Context) error {
	q := ctx.Position().Qty
	if q > 0 {
		_, err := ctx.Submit(execution.Order{Side: core.SideAsk, Qty: q})
		return err
	}
	if q < 0 {
		_, err := ctx.Submit(execution.Order{Side: core.SideBid, Qty: -q})
		return err
	}
	return nil
}
