package strategy

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/morteza-sakifard/TickEngine/internal/core"
	"github.com/morteza-sakifard/TickEngine/internal/execution"
	"github.com/morteza-sakifard/TickEngine/internal/feed/databento"
	"github.com/morteza-sakifard/TickEngine/internal/marketdata"
	"github.com/morteza-sakifard/TickEngine/internal/portfolio"
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

// copySrc is a second Source type over the same bytes. The strategy
// must not care which concrete type produced the event.
type copySrc struct{ inner sliceSrc }

func (s *copySrc) Next(dst *marketdata.Event) error { return s.inner.Next(dst) }
func (s *copySrc) Close() error                     { return s.inner.Close() }

type closeSrc struct {
	sliceSrc
	closed int
}

func (s *closeSrc) Close() error { s.closed++; return nil }

type clockProbe struct{ nows []int64 }

func (p *clockProbe) OnStart(Context) error { return nil }
func (p *clockProbe) OnStop(Context) error  { return nil }
func (p *clockProbe) OnOrder(Context, execution.OrderEvent) error {
	return nil
}
func (p *clockProbe) OnEvent(ctx Context, _ *marketdata.Event) error {
	p.nows = append(p.nows, ctx.UnixNano())
	return nil
}

func sampleEvents() []marketdata.Event {
	return []marketdata.Event{
		{
			Kind:     marketdata.KindTrade,
			TsEvent:  500,
			TsRecv:   100,
			Sequence: 1,
			Trade:    marketdata.Trade{Px: 26800, Qty: 2, Aggressor: core.SideBid},
		},
		{
			Kind:     marketdata.KindQuote,
			TsRecv:   150,
			Sequence: 2,
			Quote:    marketdata.Quote{BidPx: 26799, AskPx: 26800, BidQty: 3, AskQty: 1},
		},
	}
}

func TestClockUsesTsRecvNotTsEvent(t *testing.T) {
	p := &clockProbe{}
	if err := Run(&sliceSrc{evs: sampleEvents()[:1]}, p, NewRuntime(core.ESZ5(), nil)); err != nil {
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
	p := &clockProbe{}
	if err := Run(src, p, NewRuntime(core.ESZ5(), nil)); err != nil {
		t.Fatal(err)
	}
	want := []int64{100, 100, 150}
	if len(p.nows) != 3 || p.nows[0] != want[0] || p.nows[1] != want[1] || p.nows[2] != want[2] {
		t.Fatalf("clock = %v, want %v", p.nows, want)
	}
}

func TestFlagBadTsRecvDoesNotAdvance(t *testing.T) {
	src := &sliceSrc{evs: []marketdata.Event{
		{Kind: marketdata.KindTrade, TsRecv: 100, Sequence: 1},
		{Kind: marketdata.KindTrade, TsRecv: 999, Sequence: 2, Flags: marketdata.FlagBadTsRecv},
		{Kind: marketdata.KindTrade, TsRecv: 200, Sequence: 3},
	}}
	p := &clockProbe{}
	if err := Run(src, p, NewRuntime(core.ESZ5(), nil)); err != nil {
		t.Fatal(err)
	}
	want := []int64{100, 100, 200}
	if len(p.nows) != 3 || p.nows[0] != want[0] || p.nows[1] != want[1] || p.nows[2] != want[2] {
		t.Fatalf("clock = %v, want %v (bad stamp delivered, clock stayed)", p.nows, want)
	}
}

func TestCacheCopiesEvent(t *testing.T) {
	ev := marketdata.Event{
		Kind:     marketdata.KindTrade,
		TsRecv:   100,
		Sequence: 7,
		Trade:    marketdata.Trade{Px: 10, Qty: 4, Aggressor: core.SideAsk},
	}
	rt := NewRuntime(core.ESZ5(), nil)
	rt.Observe(&ev)
	ev.Trade.Px = 99
	ev.Sequence = 0
	got, ok := rt.LastTrade()
	if !ok || got.Px != 10 || got.Qty != 4 {
		t.Fatalf("LastTrade = (%v, %v), want px 10 after buffer overwrite", got, ok)
	}
	last, ok := rt.Last()
	if !ok || last.Sequence != 7 || last.Trade.Px != 10 {
		t.Fatalf("Last = (%v, %v), want the copied trade", last, ok)
	}
}

func TestCacheSeparatesTradeAndQuote(t *testing.T) {
	rt := NewRuntime(core.ESZ5(), nil)
	if _, ok := rt.LastTrade(); ok {
		t.Fatal("empty cache must not report a trade")
	}
	rt.Observe(&marketdata.Event{
		Kind:  marketdata.KindTrade,
		Trade: marketdata.Trade{Px: 1, Qty: 1, Aggressor: core.SideBid},
	})
	rt.Observe(&marketdata.Event{
		Kind:  marketdata.KindQuote,
		Quote: marketdata.Quote{BidPx: 2, AskPx: 3},
	})
	tr, ok := rt.LastTrade()
	if !ok || tr.Px != 1 {
		t.Fatalf("trade after quote = (%v, %v), want px 1", tr, ok)
	}
	q, ok := rt.Quote()
	if !ok || q.BidPx != 2 || q.AskPx != 3 {
		t.Fatalf("quote = (%v, %v)", q, ok)
	}
}

func TestLogStrategyTwoSourceTypesSameLog(t *testing.T) {
	evs := sampleEvents()
	var a, b bytes.Buffer
	s := LogStrategy{}
	if err := Run(&sliceSrc{evs: evs}, s, NewRuntime(core.ESZ5(), &a)); err != nil {
		t.Fatal(err)
	}
	if err := Run(&copySrc{inner: sliceSrc{evs: evs}}, s, NewRuntime(core.ESZ5(), &b)); err != nil {
		t.Fatal(err)
	}
	if a.String() != b.String() {
		t.Fatalf("same events, two Source types, different logs:\n%s---\n%s", a.String(), b.String())
	}
	if !strings.HasPrefix(a.String(), "start ESZ5\n") || !strings.HasSuffix(a.String(), "stop\n") {
		t.Fatalf("log missing start/stop: %q", a.String())
	}
}

func TestLogStrategySliceMatchesFixture(t *testing.T) {
	path := filepath.Join("..", "..", "data", "test", "mbp1_sample.csv")
	collect := func() []marketdata.Event {
		f, err := os.Open(path)
		if err != nil {
			t.Skip(err)
		}
		dec, err := databento.NewDecoder(f, core.ESZ5())
		if err != nil {
			f.Close()
			t.Fatal(err)
		}
		defer dec.Close()
		var evs []marketdata.Event
		for {
			var ev marketdata.Event
			err := dec.Next(&ev)
			if err == io.EOF {
				return evs
			}
			if err != nil {
				t.Fatal(err)
			}
			evs = append(evs, ev)
		}
	}
	evs := collect()
	hash := func(src Source) [32]byte {
		var buf bytes.Buffer
		if err := Run(src, LogStrategy{}, NewRuntime(core.ESZ5(), &buf)); err != nil {
			t.Fatal(err)
		}
		return sha256.Sum256(buf.Bytes())
	}
	f, err := os.Open(path)
	if err != nil {
		t.Skip(err)
	}
	dec, err := databento.NewDecoder(f, core.ESZ5())
	if err != nil {
		f.Close()
		t.Fatal(err)
	}
	defer dec.Close()
	if a, b := hash(&sliceSrc{evs: evs}), hash(dec); a != b {
		t.Fatal("LogStrategy on slice vs mbp1_sample.csv produced different hashes")
	}
}

func TestRunDoesNotCloseSource(t *testing.T) {
	src := &closeSrc{sliceSrc: sliceSrc{evs: sampleEvents()}}
	if err := Run(src, LogStrategy{}, NewRuntime(core.ESZ5(), io.Discard)); err != nil {
		t.Fatal(err)
	}
	if src.closed != 0 {
		t.Fatalf("Close called %d times; caller owns the source", src.closed)
	}
}

func TestRunEmptySourceStillStartsAndStops(t *testing.T) {
	var buf bytes.Buffer
	if err := Run(&sliceSrc{}, LogStrategy{}, NewRuntime(core.ESZ5(), &buf)); err != nil {
		t.Fatal(err)
	}
	if buf.String() != "start ESZ5\nstop\n" {
		t.Fatalf("empty run = %q", buf.String())
	}
}

func TestRunNilArgs(t *testing.T) {
	rt := NewRuntime(core.ESZ5(), nil)
	src := &sliceSrc{}
	if err := Run(nil, LogStrategy{}, rt); err == nil {
		t.Fatal("want error for nil source")
	}
	if err := Run(src, nil, rt); err == nil {
		t.Fatal("want error for nil strategy")
	}
	if err := Run(src, LogStrategy{}, nil); err == nil {
		t.Fatal("want error for nil runtime")
	}
}

func TestRuntimeHasNoMode(t *testing.T) {
	tpe := reflect.TypeOf(Runtime{})
	for i := 0; i < tpe.NumField(); i++ {
		n := strings.ToLower(tpe.Field(i).Name)
		if n == "mode" || strings.Contains(n, "live") || strings.Contains(n, "backtest") {
			t.Fatalf("field %s lets a strategy tell backtest from live", tpe.Field(i).Name)
		}
	}
}

func TestNowUsesVirtualClock(t *testing.T) {
	rt := NewRuntime(core.ESZ5(), nil)
	const ts int64 = 1_700_000_000_000_000_000
	rt.Observe(&marketdata.Event{TsRecv: ts})
	if got := rt.Now().UTC().UnixNano(); got != ts {
		t.Fatalf("Now = %d, want TsRecv %d", got, ts)
	}
	if rt.UnixNano() != ts {
		t.Fatalf("UnixNano = %d, want %d", rt.UnixNano(), ts)
	}
}

type phaseLog struct{ phases []string }

func (p *phaseLog) OnStart(Context) error { return nil }
func (p *phaseLog) OnStop(Context) error  { return nil }
func (p *phaseLog) OnOrder(Context, execution.OrderEvent) error {
	p.phases = append(p.phases, "order")
	return nil
}
func (p *phaseLog) OnEvent(ctx Context, _ *marketdata.Event) error {
	p.phases = append(p.phases, "event")
	pos := ctx.Position()
	if pos.Qty == 0 && !contains(p.phases, "order") {
		_, err := ctx.Submit(execution.Order{Side: core.SideBid, Qty: 1})
		return err
	}
	if pos.Qty > 0 {
		_, err := ctx.Submit(execution.Order{Side: core.SideAsk, Qty: 1})
		return err
	}
	return nil
}

func contains(s []string, w string) bool {
	for _, x := range s {
		if x == w {
			return true
		}
	}
	return false
}

func TestCascadeEventOrderFillEvent(t *testing.T) {
	src := &sliceSrc{evs: []marketdata.Event{{
		Kind:   marketdata.KindQuote,
		TsRecv: 10,
		Quote:  marketdata.Quote{BidPx: 99, AskPx: 100},
	}}}
	s := &phaseLog{}
	if err := Run(src, s, NewRuntime(core.ESZ5(), nil)); err != nil {
		t.Fatal(err)
	}
	want := []string{"event", "order", "event", "order", "event"}
	if len(s.phases) != len(want) {
		t.Fatalf("phases = %v, want %v", s.phases, want)
	}
	for i := range want {
		if s.phases[i] != want[i] {
			t.Fatalf("phases = %v, want %v", s.phases, want)
		}
	}
}

type oneTick struct {
	bought bool
	buyTs  int64
}

func (s *oneTick) OnStart(Context) error { return nil }
func (s *oneTick) OnStop(Context) error  { return nil }
func (s *oneTick) OnOrder(Context, execution.OrderEvent) error {
	return nil
}
func (s *oneTick) OnEvent(ctx Context, ev *marketdata.Event) error {
	if ev.Kind != marketdata.KindQuote {
		return nil
	}
	if !s.bought {
		s.bought = true
		s.buyTs = ctx.UnixNano()
		_, err := ctx.Submit(execution.Order{Side: core.SideBid, Qty: 1})
		return err
	}
	// Cascade re-enters OnEvent at the same TsRecv. Selling here
	// would hit the entry bid and lose a tick; wait for the next quote.
	if ctx.Position().Qty > 0 && ctx.UnixNano() > s.buyTs {
		_, err := ctx.Submit(execution.Order{Side: core.SideAsk, Qty: 1})
		return err
	}
	return nil
}

func TestZeroLatencySamePnLAsStep20(t *testing.T) {
	const x core.Ticks = 26800
	src := func() Source {
		return &sliceSrc{evs: []marketdata.Event{
			{Kind: marketdata.KindQuote, TsRecv: 1, Quote: marketdata.Quote{BidPx: x - 1, AskPx: x}},
			{Kind: marketdata.KindQuote, TsRecv: 2, Quote: marketdata.Quote{BidPx: x + 1, AskPx: x + 2}},
		}}
	}
	fees := execution.Fees{CommissionCents: 100, FeeCents: 12}
	run := func(lat execution.Latency) portfolio.Position {
		t.Helper()
		rt := NewRuntime(core.ESZ5(), nil)
		if err := rt.SetFees(fees); err != nil {
			t.Fatal(err)
		}
		if err := rt.SetLatency(lat); err != nil {
			t.Fatal(err)
		}
		if err := Run(src(), &oneTick{}, rt); err != nil {
			t.Fatal(err)
		}
		return rt.Position()
	}
	zero := run(execution.Latency{})
	want := portfolio.CentsPerTick(core.ESZ5()) - 2*(fees.CommissionCents+fees.FeeCents)
	if zero.Qty != 0 || zero.Realized != want {
		t.Fatalf("zero latency qty=%d realized=%d, want step-20 %d", zero.Qty, zero.Realized, want)
	}
}

func TestBlotterDeterministic(t *testing.T) {
	const x core.Ticks = 26800
	src := func() Source {
		return &sliceSrc{evs: []marketdata.Event{
			{Kind: marketdata.KindQuote, TsRecv: 1, Quote: marketdata.Quote{BidPx: x - 1, AskPx: x}},
			{Kind: marketdata.KindQuote, TsRecv: 2, Quote: marketdata.Quote{BidPx: x + 1, AskPx: x + 2}},
		}}
	}
	hash := func() [32]byte {
		rt := NewRuntime(core.ESZ5(), nil)
		if err := Run(src(), &oneTick{}, rt); err != nil {
			t.Fatal(err)
		}
		b := rt.Blotter()
		return sha256.Sum256([]byte(b.Text() + b.Metrics().Text()))
	}
	if a, b := hash(), hash(); a != b {
		t.Fatal("same data + config produced different blotter bytes")
	}
}

func TestOnEventClockIsTsRecv(t *testing.T) {
	p := &clockProbe{}
	src := &sliceSrc{evs: []marketdata.Event{{
		Kind: marketdata.KindTrade, TsEvent: 5, TsRecv: 100, TsInDelta: 40,
	}}}
	if err := Run(src, p, NewRuntime(core.ESZ5(), nil)); err != nil {
		t.Fatal(err)
	}
	if len(p.nows) != 1 || p.nows[0] != 100 {
		t.Fatalf("strategy clock = %v, want TsRecv 100 (feed delay already applied)", p.nows)
	}
}

func TestBuyXSellXPlusOneTick(t *testing.T) {
	const x core.Ticks = 26800
	src := &sliceSrc{evs: []marketdata.Event{
		{Kind: marketdata.KindQuote, TsRecv: 1, Quote: marketdata.Quote{BidPx: x - 1, AskPx: x}},
		{Kind: marketdata.KindQuote, TsRecv: 2, Quote: marketdata.Quote{BidPx: x + 1, AskPx: x + 2}},
	}}
	fees := execution.Fees{CommissionCents: 100, FeeCents: 12}
	rt := NewRuntime(core.ESZ5(), nil)
	if err := rt.SetFees(fees); err != nil {
		t.Fatal(err)
	}
	if err := Run(src, &oneTick{}, rt); err != nil {
		t.Fatal(err)
	}
	want := portfolio.CentsPerTick(core.ESZ5()) - 2*(fees.CommissionCents+fees.FeeCents)
	pos := rt.Position()
	if pos.Qty != 0 || pos.Realized != want {
		t.Fatalf("qty=%d realized=%d, want 0 and %d (Multiplier*TickSize − fees)", pos.Qty, pos.Realized, want)
	}
}

func TestSubmitCancelSameEvent(t *testing.T) {
	var canceled bool
	s := &cancelOnSubmit{done: &canceled}
	src := &sliceSrc{evs: []marketdata.Event{{
		Kind:  marketdata.KindQuote,
		Quote: marketdata.Quote{BidPx: 1, AskPx: 2},
	}}}
	rt := NewRuntime(core.ESZ5(), nil)
	if err := Run(src, s, rt); err != nil {
		t.Fatal(err)
	}
	if rt.Position().Qty != 0 {
		t.Fatal("canceled order must not fill")
	}
}

type cancelOnSubmit struct{ done *bool }

func (s *cancelOnSubmit) OnStart(Context) error { return nil }
func (s *cancelOnSubmit) OnStop(Context) error  { return nil }
func (s *cancelOnSubmit) OnOrder(Context, execution.OrderEvent) error {
	return nil
}
func (s *cancelOnSubmit) OnEvent(ctx Context, _ *marketdata.Event) error {
	if *s.done {
		return nil
	}
	id, err := ctx.Submit(execution.Order{Side: core.SideBid, Qty: 1})
	if err != nil {
		return err
	}
	*s.done = true
	return ctx.Cancel(id)
}

type buyOnce struct {
	n      int
	last   error
	qty    core.Qty
	ignore error
}

func (s *buyOnce) OnStart(Context) error { return nil }
func (s *buyOnce) OnStop(Context) error  { return nil }
func (s *buyOnce) OnOrder(Context, execution.OrderEvent) error {
	return nil
}
func (s *buyOnce) OnEvent(ctx Context, ev *marketdata.Event) error {
	if ev.Kind != marketdata.KindQuote {
		return nil
	}
	s.n++
	if s.n > 1 {
		return nil
	}
	_, err := ctx.Submit(execution.Order{Side: core.SideBid, Qty: s.qtyOr1()})
	s.last = err
	if s.ignore != nil && errors.Is(err, s.ignore) {
		return nil
	}
	return err
}

func (s *buyOnce) qtyOr1() core.Qty {
	if s.qty == 0 {
		return 1
	}
	return s.qty
}

func TestPaperLiveDataSimulatedFill(t *testing.T) {
	src := &sliceSrc{evs: []marketdata.Event{{
		Kind:   marketdata.KindQuote,
		TsRecv: 10,
		Quote:  marketdata.Quote{BidPx: 26800, AskPx: 26801, BidQty: 1, AskQty: 1},
	}}}
	paper, err := execution.NewPaper(execution.Fees{}, execution.Limits{MaxQty: 1, MaxAbsPosition: 1})
	if err != nil {
		t.Fatal(err)
	}
	rt := NewRuntime(core.ESZ5(), nil)
	if err := rt.SetPaper(paper); err != nil {
		t.Fatal(err)
	}
	if err := Run(src, &buyOnce{}, rt); err != nil {
		t.Fatal(err)
	}
	pos := rt.Position()
	if pos.Qty != 1 || pos.AvgPx != 26801 {
		t.Fatalf("paper fill qty=%d avg=%d, want 1 @ ask 26801 (simulated, not a broker)", pos.Qty, pos.AvgPx)
	}
}

func TestPaperKillRejectsSubmit(t *testing.T) {
	src := &sliceSrc{evs: []marketdata.Event{{
		Kind:   marketdata.KindQuote,
		TsRecv: 10,
		Quote:  marketdata.Quote{BidPx: 1, AskPx: 2},
	}}}
	paper, err := execution.NewPaper(execution.Fees{}, execution.Limits{})
	if err != nil {
		t.Fatal(err)
	}
	paper.Kill()
	rt := NewRuntime(core.ESZ5(), nil)
	if err := rt.SetPaper(paper); err != nil {
		t.Fatal(err)
	}
	s := &buyOnce{ignore: execution.ErrRiskKilled}
	if err := Run(src, s, rt); err != nil {
		t.Fatal(err)
	}
	if !errors.Is(s.last, execution.ErrRiskKilled) {
		t.Fatalf("Submit after Kill: %v", s.last)
	}
	if rt.Position().Qty != 0 {
		t.Fatal("killed submit must not fill")
	}
}

func TestSetRiskGatesSubmit(t *testing.T) {
	r, err := execution.NewRisk(execution.Limits{MaxQty: 1})
	if err != nil {
		t.Fatal(err)
	}
	rt := NewRuntime(core.ESZ5(), nil)
	if err := rt.SetRisk(r); err != nil {
		t.Fatal(err)
	}
	rt.Observe(&marketdata.Event{
		Kind:   marketdata.KindQuote,
		TsRecv: 1,
		Quote:  marketdata.Quote{BidPx: 1, AskPx: 2},
	})
	if _, err := rt.Submit(execution.Order{Side: core.SideBid, Qty: 3}); !errors.Is(err, execution.ErrRiskQty) {
		t.Fatalf("SetRisk: %v", err)
	}
	if evs := rt.venue.Settle(1); evs != nil {
		t.Fatalf("rejected submit reached the venue: %+v", evs)
	}
}

type startProbe struct {
	started int
	pos     portfolio.Position
}

func (s *startProbe) OnStart(ctx Context) error {
	s.started++
	s.pos = ctx.Position()
	return nil
}
func (s *startProbe) OnStop(Context) error { return nil }
func (s *startProbe) OnOrder(Context, execution.OrderEvent) error {
	return nil
}
func (s *startProbe) OnEvent(Context, *marketdata.Event) error { return nil }

type restOnce struct{ sent bool }

func (s *restOnce) OnStart(Context) error { return nil }
func (s *restOnce) OnStop(Context) error  { return nil }
func (s *restOnce) OnOrder(Context, execution.OrderEvent) error {
	return nil
}
func (s *restOnce) OnEvent(ctx Context, ev *marketdata.Event) error {
	if s.sent || ev.Kind != marketdata.KindQuote {
		return nil
	}
	s.sent = true
	_, err := ctx.Submit(execution.Order{Side: core.SideBid, Qty: 1, Kind: execution.KindLimit, Px: 99})
	return err
}

func TestRunRequiresReconcileBeforeOnStart(t *testing.T) {
	rt := NewRuntime(core.ESZ5(), nil)
	rt.RequireReconcile()
	s := &startProbe{}
	err := Run(&sliceSrc{}, s, rt)
	if !errors.Is(err, ErrReconcileRequired) {
		t.Fatalf("Run = %v, want ErrReconcileRequired", err)
	}
	if s.started != 0 {
		t.Fatal("OnStart must not run before reconcile")
	}
}

func TestRestartRestoresPositionFromStore(t *testing.T) {
	src := &sliceSrc{evs: []marketdata.Event{{
		Kind:   marketdata.KindQuote,
		TsRecv: 10,
		Quote:  marketdata.Quote{BidPx: 26800, AskPx: 26801},
	}}}
	store := portfolio.NewStore()
	rt := NewRuntime(core.ESZ5(), nil)
	if err := rt.SetStore(store); err != nil {
		t.Fatal(err)
	}
	if err := Run(src, &buyOnce{}, rt); err != nil {
		t.Fatal(err)
	}
	want := rt.Position()
	if want.Qty != 1 || want.AvgPx != 26801 {
		t.Fatalf("session1 pos = %+v", want)
	}
	var dump bytes.Buffer
	if err := store.WriteTo(&dump); err != nil {
		t.Fatal(err)
	}

	alive := storeFromDump(t, dump.Bytes())
	rt2 := NewRuntime(core.ESZ5(), nil)
	if err := rt2.SetStore(alive); err != nil {
		t.Fatal(err)
	}
	rt2.RequireReconcile()
	if err := rt2.Restore(); err != nil {
		t.Fatal(err)
	}
	if _, err := rt2.Reconcile(execution.Snapshot{LastID: alive.LastID()}); err != nil {
		t.Fatal(err)
	}
	s := &startProbe{}
	if err := Run(&sliceSrc{}, s, rt2); err != nil {
		t.Fatal(err)
	}
	if s.started != 1 {
		t.Fatalf("OnStart count = %d", s.started)
	}
	if s.pos != want || rt2.Position() != want {
		t.Fatalf("restart pos %+v, want %+v", rt2.Position(), want)
	}
}

func TestRestartRestoresRestingLimit(t *testing.T) {
	src := &sliceSrc{evs: []marketdata.Event{{
		Kind:   marketdata.KindQuote,
		TsRecv: 10,
		Quote:  marketdata.Quote{BidPx: 100, AskPx: 101},
	}}}
	store := portfolio.NewStore()
	rt := NewRuntime(core.ESZ5(), nil)
	if err := rt.SetStore(store); err != nil {
		t.Fatal(err)
	}
	if err := Run(src, &restOnce{}, rt); err != nil {
		t.Fatal(err)
	}
	if rt.Position().Qty != 0 {
		t.Fatal("limit at 99 must rest")
	}
	working := store.WorkingOrders()
	if len(working) != 1 || working[0].Px != 99 {
		t.Fatalf("store working = %+v", working)
	}
	var dump bytes.Buffer
	if err := store.WriteTo(&dump); err != nil {
		t.Fatal(err)
	}

	alive := storeFromDump(t, dump.Bytes())
	rt2 := NewRuntime(core.ESZ5(), nil)
	if err := rt2.SetStore(alive); err != nil {
		t.Fatal(err)
	}
	if err := rt2.Restore(); err != nil {
		t.Fatal(err)
	}
	snap := execution.Snapshot{Working: alive.WorkingOrders(), LastID: alive.LastID()}
	rep, err := rt2.Reconcile(snap)
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Keep) != 1 || len(rep.Ghost)+len(rep.Adopt) != 0 {
		t.Fatalf("paper snapshot should keep the rest: %+v", rep)
	}
	got := rt2.venue.Working()
	if len(got) != 1 || got[0].Px != 99 {
		t.Fatalf("adopted working = %+v", got)
	}
}

func TestReconcileDropsGhostAfterRestart(t *testing.T) {
	store := portfolio.NewStore()
	o := execution.Order{ID: 5, Side: core.SideBid, Qty: 1, Kind: execution.KindLimit, Px: 50}
	if err := store.AppendOrder(1, o, 0); err != nil {
		t.Fatal(err)
	}
	rt := NewRuntime(core.ESZ5(), nil)
	if err := rt.SetStore(store); err != nil {
		t.Fatal(err)
	}
	if err := rt.Restore(); err != nil {
		t.Fatal(err)
	}
	rep, err := rt.Reconcile(execution.Snapshot{LastID: 5})
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Ghost) != 1 || rep.Ghost[0].ID != 5 {
		t.Fatalf("want ghost 5, got %+v", rep)
	}
	if rt.venue.Working() != nil {
		t.Fatal("ghost must not be adopted")
	}
}

func storeFromDump(t *testing.T, raw []byte) *portfolio.Store {
	t.Helper()
	s := portfolio.NewStore()
	if err := s.Load(bytes.NewReader(raw)); err != nil {
		t.Fatal(err)
	}
	return s
}

func TestDepsExcludeFeed(t *testing.T) {
	cmd := exec.Command("go", "list", "-deps", ".")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go list -deps: %v\n%s", err, out)
	}
	s := string(out)
	for _, bad := range []string{"/internal/feed", "/internal/replay", "/internal/chart"} {
		if strings.Contains(s, bad) {
			t.Fatalf("go list -deps ./internal/strategy contains %s:\n%s", bad, s)
		}
	}
}

func TestPackageConstraints(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		b, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		s := string(b)
		for _, bad := range []string{
			"time.Now(", "os.Open", "\ngo ", " chan ", "\tchan ", "\nselect ", " select ",
			"internal/feed", "internal/replay", "internal/chart",
		} {
			if strings.Contains(s, bad) {
				t.Errorf("%s contains %q", name, strings.TrimSpace(bad))
			}
		}
	}
}
