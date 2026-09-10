package main

import (
	"bytes"
	"os"
	"testing"
	"time"

	"github.com/morteza-sakifard/TickEngine/internal/core"
	"github.com/morteza-sakifard/TickEngine/internal/feed/databento"
	"github.com/morteza-sakifard/TickEngine/internal/marketdata"
	"github.com/morteza-sakifard/TickEngine/internal/replay"
)

func TestParseSpeed(t *testing.T) {
	tests := []struct {
		in      string
		want    float64
		wantErr bool
	}{
		{in: "0", want: 0},
		{in: "1", want: 1},
		{in: "100", want: 100},
		{in: "1000", want: 1000},
		{in: "0.5", want: 0.5},
		{in: "-1", wantErr: true},
		{in: "bogus", wantErr: true},
		{in: "NaN", wantErr: true},
		{in: "Inf", wantErr: true},
	}
	for _, tt := range tests {
		got, err := parseSpeed(tt.in)
		if (err != nil) != tt.wantErr {
			t.Errorf("parseSpeed(%q) err = %v, wantErr %v", tt.in, err, tt.wantErr)
			continue
		}
		if !tt.wantErr && got != tt.want {
			t.Errorf("parseSpeed(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

func TestLookupInstrument(t *testing.T) {
	got, err := lookupInstrument("ESZ5")
	if err != nil || got.ID != core.ESZ5().ID {
		t.Fatalf("ESZ5: got %+v err=%v", got, err)
	}
	if _, err := lookupInstrument("NQZ5"); err == nil {
		t.Fatal("unknown symbol should error")
	}
}

func TestFormatEvent(t *testing.T) {
	trade := formatEvent(&marketdata.Event{
		Kind:     marketdata.KindTrade,
		TsRecv:   1e9,
		Sequence: 7,
		Trade:    marketdata.Trade{Px: 26800, Qty: 3, Aggressor: core.SideBid},
	})
	wantTrade := "trade recv=1000000000 seq=7 px=26800 qty=3 side=bid"
	if trade != wantTrade {
		t.Fatalf("trade = %q, want %q", trade, wantTrade)
	}
	quote := formatEvent(&marketdata.Event{
		Kind:     marketdata.KindQuote,
		TsRecv:   2e9,
		Sequence: 8,
		Flags:    marketdata.FlagBadTsRecv,
		Quote:    marketdata.Quote{BidPx: 26799, AskPx: 26800, BidQty: 4, AskQty: 5},
	})
	wantQuote := "quote recv=2000000000 seq=8 bid=26799 ask=26800 bq=4 aq=5 bad_ts_recv"
	if quote != wantQuote {
		t.Fatalf("quote = %q, want %q", quote, wantQuote)
	}
}

func TestReplayLogIndependentOfSpeed(t *testing.T) {
	run := func(speed float64) string {
		t.Helper()
		f, err := os.Open("../../data/test/mbp1_sample.csv")
		if err != nil {
			t.Skip(err)
		}
		dec, err := databento.NewDecoder(f, core.ESZ5())
		if err != nil {
			f.Close()
			t.Fatal(err)
		}
		defer dec.Close()
		var buf bytes.Buffer
		if _, err := replaySource(dec, &buf, &replay.Pacer{
			Speed: speed,
			Sleep: func(time.Duration) {},
			Hold:  func() {},
			Step:  true,
		}); err != nil {
			t.Fatal(err)
		}
		return buf.String()
	}
	if run(0) != run(1000) {
		t.Fatal("speed 0 and speed 1000 produced different event logs")
	}
}
