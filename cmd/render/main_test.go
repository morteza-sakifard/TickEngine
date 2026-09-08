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
	"github.com/morteza-sakifard/market-data-lab/internal/orderflow"
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

func rthTrade(t *testing.T, cal session.Calendar, hour, min int, px, qty int64, side core.Side) marketdata.Event {
	t.Helper()
	ts := time.Date(2025, time.September, 23, hour, min, 0, 0, cal.Location)
	return marketdata.Event{
		Kind:    marketdata.KindTrade,
		TsEvent: ts.UnixNano(),
		Trade:   marketdata.Trade{Px: core.Ticks(px), Qty: core.Qty(qty), Aggressor: side},
	}
}

func TestSampleFlowVWAPAndCVD(t *testing.T) {
	cal := esCal(t)
	trades := []marketdata.Event{
		rthTrade(t, cal, 8, 30, 26800, 5, core.SideBid),
		rthTrade(t, cal, 8, 31, 26804, 2, core.SideAsk),
		rthTrade(t, cal, 8, 36, 26802, 1, core.SideBid),
	}
	agg, err := aggregation.New(aggregation.BarSpec{
		Kind:     aggregation.KindTime,
		Interval: 5 * time.Minute,
		Anchor:   aggregation.AnchorRTHOpen,
		Location: cal.Location,
		Sessions: session.SetRTH,
	}, cal)
	if err != nil {
		t.Fatal(err)
	}
	for i := range trades {
		agg.Add(&trades[i])
	}
	agg.Flush()
	bars := agg.Bars()
	if len(bars) != 2 {
		t.Fatalf("bars = %d, want 2", len(bars))
	}

	vwap, cvd := sampleFlow(trades, bars, cal)

	// Independent Σpx*qty/Σqty, truncated: (26800*5+26804*2)/7 = 26801
	// then +26802 / 8 = 26801.
	if vwap[0] != 26801 || vwap[1] != 26801 {
		t.Fatalf("VWAP = %v, want [26801 26801]", vwap)
	}
	lo, hi := bars[0].Low, bars[0].High
	for _, b := range bars[1:] {
		if b.Low < lo {
			lo = b.Low
		}
		if b.High > hi {
			hi = b.High
		}
	}
	for i, px := range vwap {
		if px < lo || px > hi {
			t.Fatalf("vwap[%d]=%d outside session [%d, %d]", i, px, lo, hi)
		}
	}

	var cum core.Qty
	for i, b := range bars {
		cum += b.BuyVolume - b.SellVolume
		if core.Ticks(cum) != cvd[i] {
			t.Fatalf("cvd[%d]=%d, want cumsum of bar delta %d", i, cvd[i], cum)
		}
	}
	if cvd[0] != 3 || cvd[1] != 4 {
		t.Fatalf("CVD = %v, want [3 4]", cvd)
	}

	var vp orderflow.VolumeProfile
	for i := range trades {
		vp.OnTrade(&trades[i])
	}
	prof := snapshotProfile(&vp)
	if prof == nil {
		t.Fatal("expected a profile")
	}
	if prof.VAL > prof.POC || prof.POC > prof.VAH {
		t.Fatalf("VAL ≤ POC ≤ VAH violated: %d %d %d", prof.VAL, prof.POC, prof.VAH)
	}
	if prof.POC < lo || prof.POC > hi {
		t.Fatalf("POC %d outside session [%d, %d]", prof.POC, lo, hi)
	}
	var sum core.Qty
	for _, lv := range prof.Levels {
		sum += lv.Volume
	}
	if sum != vp.Total() {
		t.Fatalf("profile levels sum %d != total %d", sum, vp.Total())
	}

	fp := snapshotFootprints(trades, bars)
	if fp == nil || len(fp.Bars) != len(bars) {
		t.Fatalf("footprint bars = %d, want %d", len(fp.Bars), len(bars))
	}
	for i, b := range bars {
		var buy, sell core.Qty
		for _, lv := range fp.Bars[i].Levels {
			buy += lv.Buy
			sell += lv.Sell
		}
		if buy != b.BuyVolume || sell != b.SellVolume {
			t.Fatalf("bar %d footprint buy/sell %d/%d != bar %d/%d", i, buy, sell, b.BuyVolume, b.SellVolume)
		}
	}
}

func TestSnapshotTPOFullRTHHas13Periods(t *testing.T) {
	cal := esCal(t)
	day := time.Date(2025, time.September, 23, 0, 0, 0, 0, cal.Location)
	h := cal.Schedule.HoursFor(day)
	var trades []marketdata.Event
	for i := 0; i < 13; i++ {
		at := h.RTHOpen.Add(time.Duration(i)*orderflow.TPOPeriod + time.Second)
		trades = append(trades, marketdata.Event{
			Kind:    marketdata.KindTrade,
			TsEvent: at.UnixNano(),
			Trade:   marketdata.Trade{Px: 26800, Qty: 1, Aggressor: core.SideBid},
		})
	}
	view := snapshotTPO(trades, cal, spec{Date: day, Session: session.RTH})
	if view == nil {
		t.Fatal("expected a TPO view")
	}
	if view.PeriodCount != 13 {
		t.Fatalf("PeriodCount = %d, want 13", view.PeriodCount)
	}
	if view.Levels[0].Letters != "ABCDEFGHIJKLM" {
		t.Fatalf("letters = %q, want A–M", view.Levels[0].Letters)
	}
}
