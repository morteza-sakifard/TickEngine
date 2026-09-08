package main

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/morteza-sakifard/market-data-lab/internal/aggregation"
	"github.com/morteza-sakifard/market-data-lab/internal/core"
	"github.com/morteza-sakifard/market-data-lab/internal/feed/databento"
	"github.com/morteza-sakifard/market-data-lab/internal/marketdata"
	"github.com/morteza-sakifard/market-data-lab/internal/session"
)

func TestParseInterval(t *testing.T) {
	tests := []struct {
		in      string
		want    time.Duration
		wantErr bool
	}{
		{in: "5m", want: 5 * time.Minute},
		{in: "30m", want: 30 * time.Minute},
		{in: "1h", want: time.Hour},
		{in: "4h", want: 4 * time.Hour},
		{in: "1d", want: 24 * time.Hour},
		{in: "0s", wantErr: true},
		{in: "bogus", wantErr: true},
	}
	for _, tt := range tests {
		got, err := parseInterval(tt.in)
		if (err != nil) != tt.wantErr {
			t.Errorf("parseInterval(%q) err = %v, wantErr %v", tt.in, err, tt.wantErr)
			continue
		}
		if !tt.wantErr && got != tt.want {
			t.Errorf("parseInterval(%q) = %s, want %s", tt.in, got, tt.want)
		}
	}
}

func TestParseSession(t *testing.T) {
	sess, set, anchor, err := parseSession("RTH")
	if err != nil || sess != session.RTH || set != session.SetRTH || anchor != aggregation.AnchorRTHOpen {
		t.Fatalf("RTH: sess=%v set=%v anchor=%v err=%v", sess, set, anchor, err)
	}
	sess, set, anchor, err = parseSession("ETH")
	if err != nil || sess != session.ETH || set != session.SetETH || anchor != aggregation.AnchorSessionOpen {
		t.Fatalf("ETH: sess=%v set=%v anchor=%v err=%v", sess, set, anchor, err)
	}
	if _, _, _, err := parseSession("rth"); err == nil {
		t.Fatal("lowercase rth should be rejected")
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

func TestParseDate(t *testing.T) {
	loc, err := time.LoadLocation("America/Chicago")
	if err != nil {
		t.Fatal(err)
	}
	got, err := parseDate("2025-11-24", loc)
	if err != nil {
		t.Fatal(err)
	}
	if y, m, d := got.Date(); y != 2025 || m != time.November || d != 24 {
		t.Fatalf("date = %s", got)
	}
	if got.Location() != loc {
		t.Fatal("date must be in the calendar location")
	}
	if _, err := parseDate("11/24/2025", loc); err == nil {
		t.Fatal("want error for non-ISO date")
	}
}

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

func esCal(t *testing.T) session.Calendar {
	t.Helper()
	cal, err := session.ForProduct("ES")
	if err != nil {
		t.Fatal(err)
	}
	return cal
}

func TestRenderFixtureETH(t *testing.T) {
	// testdata trades sit on Sunday evening 2025-09-21/22 UTC, which
	// Classify assigns to Monday 2025-09-22 ETH — not RTH.
	f, err := os.Open("../../testdata/mbp1_sample.csv")
	if err != nil {
		t.Fatal(err)
	}
	dec, err := databento.NewDecoder(f, core.ESZ5())
	if err != nil {
		f.Close()
		t.Fatal(err)
	}
	defer dec.Close()

	cal := esCal(t)
	var buf bytes.Buffer
	n, err := render(dec, &buf, core.ESZ5(), cal, spec{
		Date:     time.Date(2025, time.September, 22, 0, 0, 0, 0, cal.Location),
		Session:  session.ETH,
		Sessions: session.SetETH,
		Anchor:   aggregation.AnchorSessionOpen,
		Interval: 5 * time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}
	if n < 1 {
		t.Fatalf("bars = %d, want at least 1", n)
	}
	svg := buf.String()
	if !strings.Contains(svg, "<svg") || !strings.Contains(svg, "ESZ5") {
		t.Fatalf("output is not an ESZ5 SVG")
	}
	if !strings.Contains(svg, "ETH") {
		t.Fatalf("header missing ETH")
	}
	if !strings.Contains(svg, "2025-09-22") {
		t.Fatalf("header missing trading date")
	}
	if !strings.Contains(svg, "<polyline") {
		t.Fatal("ETH fixture SVG missing VWAP/CVD polyline")
	}
	if !strings.Contains(svg, "CVD") {
		t.Fatal("ETH fixture SVG missing CVD panel")
	}
	if !strings.Contains(svg, "stroke-dasharray") {
		t.Fatal("ETH fixture SVG missing volume-profile POC line")
	}
	if !strings.Contains(svg, "fill-opacity") {
		t.Fatal("ETH fixture SVG missing footprint cells")
	}
	if !strings.Contains(svg, "tpo-clip") {
		t.Fatal("ETH fixture SVG missing TPO letters")
	}
}

func TestRenderNoTradesIsError(t *testing.T) {
	cal := esCal(t)
	// A Closed Sunday morning — Classify yields no trading date.
	ev := marketdata.Event{
		Kind:    marketdata.KindTrade,
		TsEvent: time.Date(2025, time.September, 21, 12, 0, 0, 0, cal.Location).UnixNano(),
		Trade:   marketdata.Trade{Px: 26800, Qty: 1, Aggressor: core.SideBid},
	}
	var buf bytes.Buffer
	_, err := render(&sliceSrc{evs: []marketdata.Event{ev}}, &buf, core.ESZ5(), cal, spec{
		Date:     time.Date(2025, time.September, 22, 0, 0, 0, 0, cal.Location),
		Session:  session.RTH,
		Sessions: session.SetRTH,
		Anchor:   aggregation.AnchorRTHOpen,
		Interval: 5 * time.Minute,
	})
	if err == nil {
		t.Fatal("want error when no trades match the requested date/session")
	}
}
