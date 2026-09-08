package replay

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/morteza-sakifard/market-data-lab/internal/core"
	"github.com/morteza-sakifard/market-data-lab/internal/feed/databento"
	"github.com/morteza-sakifard/market-data-lab/internal/marketdata"
)

type sliceSrc struct {
	evs []marketdata.Event
	i   int
}

func (s *sliceSrc) Next(dst *marketdata.Event) error {
	if s.i >= len(s.evs) {
		return io.EOF
	}
	*dst = s.evs[s.i]
	s.i++
	return nil
}

func (s *sliceSrc) Close() error { return nil }

type logHandler struct{ buf bytes.Buffer }

func (h *logHandler) OnEvent(ev *marketdata.Event) {
	fmt.Fprintf(&h.buf, "%d %d %d %d %d %d %d %d %d %d %d %d %d %d\n",
		ev.Kind, ev.Instrument, ev.TsEvent, ev.TsRecv, ev.TsInDelta, ev.Sequence, ev.Flags,
		ev.Trade.Px, ev.Trade.Qty, ev.Trade.Aggressor,
		ev.Quote.BidPx, ev.Quote.AskPx, ev.Quote.BidQty, ev.Quote.AskQty)
}

type clockProbe struct {
	clk  Clock
	nows []int64
}

func (p *clockProbe) OnEvent(*marketdata.Event) {
	p.nows = append(p.nows, p.clk.UnixNano())
}

type named struct {
	name string
	dst  *[]string
}

func (n named) OnEvent(*marketdata.Event) { *n.dst = append(*n.dst, n.name) }

func TestClockUsesTsRecvNotTsEvent(t *testing.T) {
	src := &sliceSrc{evs: []marketdata.Event{{
		Kind: marketdata.KindTrade, TsEvent: 500, TsRecv: 100,
		Trade: marketdata.Trade{Px: 1, Qty: 1, Aggressor: core.SideBid},
	}}}
	e := New(src)
	p := &clockProbe{clk: e.Clock()}
	e.Subscribe(p)
	if err := e.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(p.nows) != 1 || p.nows[0] != 100 {
		t.Fatalf("clock = %v, want [100] (TsRecv, not TsEvent 500)", p.nows)
	}
}

func TestClockNeverGoesBackward(t *testing.T) {
	src := &sliceSrc{evs: []marketdata.Event{
		{Kind: marketdata.KindTrade, TsRecv: 100},
		{Kind: marketdata.KindTrade, TsRecv: 50},
		{Kind: marketdata.KindTrade, TsRecv: 150},
	}}
	e := New(src)
	p := &clockProbe{clk: e.Clock()}
	e.Subscribe(p)
	if err := e.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	want := []int64{100, 100, 150}
	if len(p.nows) != 3 || p.nows[0] != want[0] || p.nows[1] != want[1] || p.nows[2] != want[2] {
		t.Fatalf("clock = %v, want %v", p.nows, want)
	}
}

func TestBadTsRecvDoesNotAdvance(t *testing.T) {
	src := &sliceSrc{evs: []marketdata.Event{
		{Kind: marketdata.KindTrade, TsRecv: 100},
		{Kind: marketdata.KindTrade, TsRecv: 999, Flags: marketdata.FlagBadTsRecv},
		{Kind: marketdata.KindTrade, TsRecv: 200},
	}}
	e := New(src)
	p := &clockProbe{clk: e.Clock()}
	var n int
	e.Subscribe(p)
	e.Subscribe(HandlerFunc(func(*marketdata.Event) { n++ }))
	if err := e.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if n != 3 {
		t.Fatalf("delivered %d events, want 3 (bad stamp is still a print)", n)
	}
	want := []int64{100, 100, 200}
	if len(p.nows) != 3 || p.nows[0] != want[0] || p.nows[1] != want[1] || p.nows[2] != want[2] {
		t.Fatalf("clock = %v, want %v", p.nows, want)
	}
}

func TestHandlerOrder(t *testing.T) {
	var got []string
	src := &sliceSrc{evs: []marketdata.Event{{TsRecv: 1}, {TsRecv: 2}}}
	e := New(src)
	e.Subscribe(named{name: "a", dst: &got})
	e.Subscribe(named{name: "b", dst: &got})
	if err := e.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	want := "a b a b"
	if strings.Join(got, " ") != want {
		t.Fatalf("order = %q, want %q", got, want)
	}
}

func TestDeterminismSameHash(t *testing.T) {
	evs := []marketdata.Event{
		{Kind: marketdata.KindTrade, TsEvent: 10, TsRecv: 11, Sequence: 1, Trade: marketdata.Trade{Px: 26800, Qty: 2, Aggressor: core.SideBid}},
		{Kind: marketdata.KindQuote, TsEvent: 12, TsRecv: 13, Sequence: 2, Quote: marketdata.Quote{BidPx: 26799, AskPx: 26800}},
		{Kind: marketdata.KindTrade, TsRecv: 5, Flags: marketdata.FlagBadTsRecv},
	}
	hashOnce := func() [32]byte {
		e := New(&sliceSrc{evs: evs})
		var h logHandler
		e.Subscribe(&h)
		if err := e.Run(context.Background()); err != nil {
			t.Fatal(err)
		}
		return sha256.Sum256(h.buf.Bytes())
	}
	if a, b := hashOnce(), hashOnce(); a != b {
		t.Fatal("two Runs on the same events produced different hashes")
	}
}

func TestDeterminismFixture(t *testing.T) {
	hashFile := func() [32]byte {
		f, err := os.Open("../../testdata/mbp1_sample.csv")
		if err != nil {
			t.Skip(err)
		}
		dec, err := databento.NewDecoder(f, core.ESZ5())
		if err != nil {
			f.Close()
			t.Fatal(err)
		}
		defer dec.Close()
		e := New(dec)
		var h logHandler
		e.Subscribe(&h)
		if err := e.Run(context.Background()); err != nil {
			t.Fatal(err)
		}
		return sha256.Sum256(h.buf.Bytes())
	}
	if a, b := hashFile(), hashFile(); a != b {
		t.Fatal("two Runs on mbp1_sample.csv produced different hashes")
	}
}

func TestRunNilSource(t *testing.T) {
	if err := New(nil).Run(context.Background()); err == nil {
		t.Fatal("want error for nil source")
	}
}

func TestRunCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	e := New(&sliceSrc{evs: []marketdata.Event{{TsRecv: 1}}})
	if err := e.Run(ctx); err == nil {
		t.Fatal("want ctx error")
	}
}

func TestNoConcurrencyPrimitives(t *testing.T) {
	for _, name := range []string{"clock.go", "engine.go", "pacer.go"} {
		b, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		s := string(b)
		for _, bad := range []string{"\ngo ", " chan ", "\tchan ", "\nselect ", " select ", "time.Now("} {
			if strings.Contains(s, bad) {
				t.Errorf("%s contains %q", name, strings.TrimSpace(bad))
			}
		}
	}
}

func BenchmarkEngineRun(b *testing.B) {
	evs := make([]marketdata.Event, 10_000)
	for i := range evs {
		evs[i] = marketdata.Event{
			Kind:   marketdata.KindTrade,
			TsRecv: int64(i + 1),
			Trade:  marketdata.Trade{Px: 26800, Qty: 1, Aggressor: core.SideBid},
		}
	}
	nop := HandlerFunc(func(*marketdata.Event) {})
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		e := New(&sliceSrc{evs: evs})
		e.Subscribe(nop)
		if err := e.Run(context.Background()); err != nil {
			b.Fatal(err)
		}
	}
	b.ReportMetric(float64(len(evs)*b.N)/b.Elapsed().Seconds(), "events/s")
}
