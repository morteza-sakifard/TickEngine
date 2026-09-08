// Command serve hosts the same chart.View as cmd/render: JSON for
// the canvas, SVG for the proof that the model is renderer-agnostic.
// Web files are embedded. See docs/01-roadmap.md step 16.
package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	_ "time/tzdata"

	"github.com/morteza-sakifard/market-data-lab/internal/aggregation"
	"github.com/morteza-sakifard/market-data-lab/internal/chart"
	"github.com/morteza-sakifard/market-data-lab/internal/compose"
	"github.com/morteza-sakifard/market-data-lab/internal/core"
	"github.com/morteza-sakifard/market-data-lab/internal/feed/databento"
	"github.com/morteza-sakifard/market-data-lab/internal/marketdata"
	"github.com/morteza-sakifard/market-data-lab/internal/session"
	"github.com/morteza-sakifard/market-data-lab/web"
)

func main() {
	log.SetFlags(0)

	var (
		data     = flag.String("data", "", "path to the Databento MBP-1 CSV (required)")
		symbol   = flag.String("symbol", "", "contract symbol, e.g. ESZ5 (required)")
		date     = flag.String("date", "", "trading date YYYY-MM-DD in the product calendar (required)")
		sessName = flag.String("session", "RTH", "default session: RTH or ETH")
		interval = flag.String("interval", "5m", "time-bar width: 5m, 15m, 30m, 1h, 4h, 1d")
		addr     = flag.String("addr", ":8080", "HTTP listen address")
	)
	flag.Parse()

	if *data == "" || *symbol == "" || *date == "" {
		flag.Usage()
		log.Fatal("--data, --symbol, and --date are required")
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
	if _, _, _, err := parseSession(*sessName); err != nil {
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
	start := time.Now()
	trades, err := compose.Collect(dec, cal, day, 0)
	dec.Close()
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("loaded %d trades for %s in %s", len(trades), day.Format("2006-01-02"), time.Since(start).Round(time.Millisecond))

	srv := &server{
		inst:     inst,
		cal:      cal,
		date:     day,
		interval: iv,
		defSess:  *sessName,
		trades:   trades,
		views:    map[session.Session]chart.View{},
	}
	log.Printf("listening on %s", *addr)
	if err := http.ListenAndServe(*addr, newMux(srv)); err != nil {
		log.Fatal(err)
	}
}

type server struct {
	inst     core.Instrument
	cal      session.Calendar
	date     time.Time
	interval time.Duration
	defSess  string
	trades   []marketdata.Event

	mu    sync.Mutex
	views map[session.Session]chart.View
}

func newMux(s *server) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/view", s.serveJSON)
	mux.HandleFunc("GET /api/view.svg", s.serveSVG)
	mux.Handle("/", http.FileServer(http.FS(web.FS)))
	return mux
}

func (s *server) serveJSON(w http.ResponseWriter, r *http.Request) {
	v, err := s.view(r.URL.Query().Get("session"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if err := chart.WriteJSON(w, v); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *server) serveSVG(w http.ResponseWriter, r *http.Request) {
	v, err := s.view(r.URL.Query().Get("session"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "image/svg+xml; charset=utf-8")
	if err := chart.RenderSVG(w, v, chart.Options{Location: s.cal.Location}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *server) view(name string) (chart.View, error) {
	if name == "" {
		name = s.defSess
	}
	sess, set, anchor, err := parseSession(name)
	if err != nil {
		return chart.View{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if v, ok := s.views[sess]; ok {
		return v, nil
	}
	v, err := compose.FromTrades(s.trades, s.inst, s.cal, compose.Spec{
		Date:     s.date,
		Session:  sess,
		Sessions: set,
		Anchor:   anchor,
		Interval: s.interval,
	})
	if err != nil {
		return chart.View{}, err
	}
	s.views[sess] = v
	return v, nil
}

func lookupInstrument(symbol string) (core.Instrument, error) {
	inst := core.ESZ5()
	if symbol != inst.Symbol {
		return core.Instrument{}, fmt.Errorf("serve: unknown symbol %q (only %s is configured)", symbol, inst.Symbol)
	}
	return inst, nil
}

func parseDate(s string, loc *time.Location) (time.Time, error) {
	t, err := time.ParseInLocation("2006-01-02", s, loc)
	if err != nil {
		return time.Time{}, fmt.Errorf("serve: --date %q: want YYYY-MM-DD", s)
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
		return session.Closed, 0, 0, fmt.Errorf("serve: session %q: want RTH or ETH", s)
	}
}

func parseInterval(s string) (time.Duration, error) {
	if s == "1d" {
		return 24 * time.Hour, nil
	}
	d, err := time.ParseDuration(s)
	if err != nil || d <= 0 {
		return 0, fmt.Errorf("serve: --interval %q: want 5m, 15m, 30m, 1h, 4h, or 1d", s)
	}
	return d, nil
}
