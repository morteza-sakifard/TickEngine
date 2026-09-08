package replay

import (
	"context"
	"crypto/sha256"
	"testing"
	"time"

	"github.com/morteza-sakifard/market-data-lab/internal/core"
	"github.com/morteza-sakifard/market-data-lab/internal/marketdata"
)

func TestPacerSpeedZeroDoesNotSleep(t *testing.T) {
	var slept []time.Duration
	p := &Pacer{Speed: 0, Sleep: func(d time.Duration) { slept = append(slept, d) }}
	p.Between(int64(time.Second))
	if len(slept) != 0 {
		t.Fatalf("speed 0 slept %v", slept)
	}
}

func TestPacerSpeedScalesGap(t *testing.T) {
	gaps := func(speed float64) []time.Duration {
		var slept []time.Duration
		e := New(&sliceSrc{evs: []marketdata.Event{
			{TsRecv: 0},
			{TsRecv: 1e9},
			{TsRecv: 2e9},
		}})
		e.SetPacer(&Pacer{Speed: speed, Sleep: func(d time.Duration) { slept = append(slept, d) }})
		e.Subscribe(HandlerFunc(func(*marketdata.Event) {}))
		if err := e.Run(context.Background()); err != nil {
			t.Fatal(err)
		}
		return slept
	}
	if got := gaps(0); len(got) != 0 {
		t.Fatalf("speed 0 slept %v", got)
	}
	if got := gaps(1); len(got) != 2 || got[0] != time.Second || got[1] != time.Second {
		t.Fatalf("speed 1, TsRecv 0/1s/2s → %v, want [1s 1s]", got)
	}
	if got := gaps(100); len(got) != 2 || got[0] != 10*time.Millisecond || got[1] != 10*time.Millisecond {
		t.Fatalf("speed 100, TsRecv 0/1s/2s → %v, want [10ms 10ms]", got)
	}
}

func TestPacerAwaitOnlyWhenStep(t *testing.T) {
	var n int
	p := &Pacer{Hold: func() { n++ }}
	p.Await()
	if n != 0 {
		t.Fatal("Await without Step must not Hold")
	}
	p.Step = true
	p.Await()
	if n != 1 {
		t.Fatalf("Hold called %d times, want 1", n)
	}
}

func TestPacerDoesNotChangeEventLog(t *testing.T) {
	evs := []marketdata.Event{
		{Kind: marketdata.KindTrade, TsEvent: 10, TsRecv: 1e9, Sequence: 1, Trade: marketdata.Trade{Px: 26800, Qty: 1, Aggressor: core.SideBid}},
		{Kind: marketdata.KindQuote, TsEvent: 20, TsRecv: 2e9, Sequence: 2, Quote: marketdata.Quote{BidPx: 26799, AskPx: 26800}},
		{Kind: marketdata.KindTrade, TsRecv: 3e9, Sequence: 3},
	}
	run := func(speed float64) [32]byte {
		e := New(&sliceSrc{evs: evs})
		e.SetPacer(&Pacer{
			Speed: speed,
			Sleep: func(time.Duration) {},
			Hold:  func() {},
			Step:  true,
		})
		var h logHandler
		e.Subscribe(&h)
		if err := e.Run(context.Background()); err != nil {
			t.Fatal(err)
		}
		return sha256.Sum256(h.buf.Bytes())
	}
	if run(0) != run(1000) {
		t.Fatal("speed 0 and speed 1000 produced different event logs")
	}
}

func TestPacerBetweenUsesClockDelta(t *testing.T) {
	var gaps []time.Duration
	src := &sliceSrc{evs: []marketdata.Event{
		{TsRecv: 1e9},
		{TsRecv: 2e9},
		{TsRecv: 2e9, Flags: marketdata.FlagBadTsRecv},
		{TsRecv: 3e9},
	}}
	e := New(src)
	e.SetPacer(&Pacer{Speed: 1, Sleep: func(d time.Duration) { gaps = append(gaps, d) }})
	e.Subscribe(HandlerFunc(func(*marketdata.Event) {}))
	if err := e.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	// first event: clock 0→1s, then 1s, then bad (0), then 1s
	if len(gaps) != 3 || gaps[0] != time.Second || gaps[1] != time.Second || gaps[2] != time.Second {
		t.Fatalf("gaps = %v, want [1s 1s 1s] (bad stamp adds no wait)", gaps)
	}
}

func TestPacerAwaitBeforeNextNotAfterLast(t *testing.T) {
	var holds int
	src := &sliceSrc{evs: []marketdata.Event{{TsRecv: 1}, {TsRecv: 2}, {TsRecv: 3}}}
	e := New(src)
	e.SetPacer(&Pacer{Step: true, Hold: func() { holds++ }})
	e.Subscribe(HandlerFunc(func(*marketdata.Event) {}))
	if err := e.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if holds != 2 {
		t.Fatalf("holds = %d, want 2 (not before the first, not after the last)", holds)
	}
}
