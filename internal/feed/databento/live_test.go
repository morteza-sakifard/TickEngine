package databento

import (
	"bufio"
	"bytes"
	"io"
	"net"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/morteza-sakifard/TickEngine/internal/core"
	"github.com/morteza-sakifard/TickEngine/internal/execution"
	"github.com/morteza-sakifard/TickEngine/internal/marketdata"
	"github.com/morteza-sakifard/TickEngine/internal/orderbook"
	"github.com/morteza-sakifard/TickEngine/internal/strategy"
)

func mbp1QuoteRow(flags, seq, bid, ask string) string {
	return "2025-09-22T00:00:00.000000000Z,2025-09-22T00:00:00.000000000Z,1,1,294973," +
		"A,N,0,6700.000000000,1," + flags + ",0," + seq + "," +
		bid + "," + ask + ",5,4,2,2,ESZ5\n"
}

type testGW struct {
	ln     net.Listener
	bodies []string
	auths  []string
	mu     sync.Mutex
	n      int
}

func startGW(t *testing.T, bodies ...string) *testGW {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	g := &testGW{ln: ln, bodies: bodies}
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go g.serve(c)
		}
	}()
	t.Cleanup(func() { _ = ln.Close() })
	return g
}

func (g *testGW) serve(c net.Conn) {
	defer c.Close()
	br := bufio.NewReader(c)
	var lines [3]string
	for i := 0; i < 3; i++ {
		s, err := br.ReadString('\n')
		if err != nil {
			return
		}
		lines[i] = strings.TrimSpace(s)
	}
	g.mu.Lock()
	g.auths = append(g.auths, lines[0])
	i := g.n
	g.n++
	body := ""
	if i < len(g.bodies) {
		body = g.bodies[i]
	}
	g.mu.Unlock()
	_, _ = io.WriteString(c, body)
}

func (g *testGW) dial() func() (io.ReadWriteCloser, error) {
	addr := g.ln.Addr().String()
	return func() (io.ReadWriteCloser, error) {
		return net.Dial("tcp", addr)
	}
}

func readAll(t *testing.T, src interface{ Next(*marketdata.Event) error }) []marketdata.Event {
	t.Helper()
	var evs []marketdata.Event
	var ev marketdata.Event
	for {
		err := src.Next(&ev)
		if err == io.EOF {
			return evs
		}
		if err != nil {
			t.Fatal(err)
		}
		evs = append(evs, ev)
	}
}

func TestLiveDecodesLikeHistorical(t *testing.T) {
	row := mbp1QuoteRow("128", "7", "6700.000000000", "6700.250000000")
	body := expectedHeader + "\n" + row
	g := startGW(t, body)
	live, err := NewLive(LiveConfig{
		Dial:             g.dial(),
		Inst:             core.ESZ5(),
		Key:              "k",
		DisableReconnect: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = live.Close() })

	got := readAll(t, live)
	if len(got) != 1 || got[0].Kind != marketdata.KindQuote || got[0].Quote.BidPx != 26800 {
		t.Fatalf("live = %+v", got)
	}
	if got[0].Flags&marketdata.FlagSnapshot == 0 {
		t.Fatal("first quote after connect must carry SNAPSHOT so the book resets")
	}

	hist, err := NewDecoder(io.NopCloser(strings.NewReader(body)), core.ESZ5())
	if err != nil {
		t.Fatal(err)
	}
	defer hist.Close()
	var want marketdata.Event
	if err := hist.Next(&want); err != nil {
		t.Fatal(err)
	}
	if got[0].Quote != want.Quote || got[0].Sequence != want.Sequence {
		t.Fatalf("live quote %+v hist %+v", got[0].Quote, want.Quote)
	}
	g.mu.Lock()
	if len(g.auths) == 0 || g.auths[0] != "AUTH k" {
		t.Fatalf("handshake AUTH = %v", g.auths)
	}
	g.mu.Unlock()
}

func TestLiveReconnectSendsSnapshot(t *testing.T) {
	s1 := expectedHeader + "\n" + mbp1QuoteRow("128", "1", "6700.000000000", "6700.250000000")
	s2 := expectedHeader + "\n" + mbp1QuoteRow("32", "9", "6699.000000000", "6699.250000000")
	g := startGW(t, s1, s2)
	live, err := NewLive(LiveConfig{Dial: g.dial(), Inst: core.ESZ5()})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = live.Close() })

	var ev marketdata.Event
	if err := live.Next(&ev); err != nil || ev.Quote.BidPx != 26800 {
		t.Fatalf("session1: %v %+v", err, ev.Quote)
	}
	if err := live.Next(&ev); err != nil || ev.Quote.BidPx != 26796 {
		t.Fatalf("session2: %v %+v", err, ev.Quote)
	}
	if ev.Flags&marketdata.FlagSnapshot == 0 {
		t.Fatal("first quote after reconnect must be a snapshot")
	}
	_ = live.Close()
}

func TestLiveReconnectResetsL3(t *testing.T) {
	s1 := mboHeader + "\n" +
		mboRow("A", "B", "6700.000000000", "5", "1", "128", "1") + "\n" +
		mboRow("A", "B", "6699.750000000", "3", "2", "128", "2") + "\n"
	s2 := mboHeader + "\n" +
		mboRow("R", "N", "0.000000000", "0", "0", "32", "0") + "\n" +
		mboRow("A", "B", "6701.000000000", "1", "3", "128", "10") + "\n"
	g := startGW(t, s1, s2)
	live, err := NewLive(LiveConfig{Dial: g.dial(), Inst: core.ESZ5(), Schema: SchemaMBO})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = live.Close() })

	var book orderbook.L3
	var ev marketdata.Event
	for i := 0; i < 2; i++ {
		if err := live.Next(&ev); err != nil {
			t.Fatal(err)
		}
		book.Apply(&ev)
	}
	if book.BestBid().Qty != 5 || len(book.FIFO(core.SideBid, 26799)) != 1 {
		t.Fatalf("pre-reconnect book %+v", book.BestBid())
	}
	for i := 0; i < 2; i++ {
		if err := live.Next(&ev); err != nil {
			t.Fatal(err)
		}
		book.Apply(&ev)
	}
	if book.BestBid().Px != 26804 || book.BestBid().Qty != 1 {
		t.Fatalf("snapshot must replace, got %+v", book.BestBid())
	}
	if len(book.FIFO(core.SideBid, 26800)) != 0 || len(book.FIFO(core.SideBid, 26799)) != 0 {
		t.Fatal("stale levels survived reconnect")
	}
	_ = live.Close()
}

func TestLiveCloseUnblocksNext(t *testing.T) {
	g := startGW(t, expectedHeader+"\n") // header only; Next waits
	live, err := NewLive(LiveConfig{Dial: g.dial(), Inst: core.ESZ5()})
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		var ev marketdata.Event
		done <- live.Next(&ev)
	}()
	if err := live.Close(); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err == nil {
		t.Fatal("Close must unblock Next")
	}
}

type buyOnce struct {
	sent bool
}

func (s *buyOnce) OnStart(strategy.Context) error { return nil }
func (s *buyOnce) OnStop(strategy.Context) error  { return nil }
func (s *buyOnce) OnOrder(strategy.Context, execution.OrderEvent) error {
	return nil
}
func (s *buyOnce) OnEvent(ctx strategy.Context, ev *marketdata.Event) error {
	if s.sent || ev.Kind != marketdata.KindQuote {
		return nil
	}
	s.sent = true
	_, err := ctx.Submit(execution.Order{Side: core.SideBid, Qty: 1})
	return err
}

func TestLivePaperSimulatedExecution(t *testing.T) {
	body := expectedHeader + "\n" + mbp1QuoteRow("128", "1", "6700.000000000", "6700.250000000")
	g := startGW(t, body)
	live, err := NewLive(LiveConfig{
		Dial:             g.dial(),
		Inst:             core.ESZ5(),
		DisableReconnect: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = live.Close() })
	paper, err := execution.NewPaper(execution.Fees{}, execution.Limits{MaxQty: 1, MaxAbsPosition: 1})
	if err != nil {
		t.Fatal(err)
	}
	rt := strategy.NewRuntime(core.ESZ5(), nil)
	if err := rt.SetPaper(paper); err != nil {
		t.Fatal(err)
	}
	if err := strategy.Run(live, &buyOnce{}, rt); err != nil {
		t.Fatal(err)
	}
	pos := rt.Position()
	if pos.Qty != 1 || pos.AvgPx != 26801 {
		t.Fatalf("live data + paper venue: qty=%d avg=%d, want 1 @ 26801 (ask), not an exchange order", pos.Qty, pos.AvgPx)
	}
}

func TestStrategyRunsOnLiveWithoutChanges(t *testing.T) {
	body := expectedHeader + "\n" + mbp1QuoteRow("128", "1", "6700.000000000", "6700.250000000")
	g := startGW(t, body)
	live, err := NewLive(LiveConfig{
		Dial:             g.dial(),
		Inst:             core.ESZ5(),
		DisableReconnect: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = live.Close() })
	var buf bytes.Buffer
	rt := strategy.NewRuntime(core.ESZ5(), &buf)
	if err := strategy.Run(live, strategy.LogStrategy{}, rt); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "start ESZ5") || !strings.Contains(buf.String(), "stop") {
		t.Fatalf("LogStrategy on Live:\n%s", buf.String())
	}
}

func TestLiveRequiresDial(t *testing.T) {
	if _, err := NewLive(LiveConfig{Inst: core.ESZ5()}); err == nil {
		t.Fatal("nil Dial must fail")
	}
}

func TestOnlyLiveHasSocketPrimitives(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") || name == "live.go" {
			continue
		}
		b, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		s := string(b)
		for _, bad := range []string{"\ngo ", " chan ", "\tchan ", "\nselect ", " select ", "time.Now("} {
			if strings.Contains(s, bad) {
				t.Errorf("%s contains %q; only live.go may talk to a socket", name, strings.TrimSpace(bad))
			}
		}
	}
	b, err := os.ReadFile("live.go")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "time.Now(") {
		t.Fatal("live.go must not call time.Now; Dial and Close own time")
	}
}
