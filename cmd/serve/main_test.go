package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/morteza-sakifard/market-data-lab/internal/aggregation"
	"github.com/morteza-sakifard/market-data-lab/internal/chart"
	"github.com/morteza-sakifard/market-data-lab/internal/compose"
	"github.com/morteza-sakifard/market-data-lab/internal/core"
	"github.com/morteza-sakifard/market-data-lab/internal/marketdata"
	"github.com/morteza-sakifard/market-data-lab/internal/session"
)

func TestParseSessionAndInterval(t *testing.T) {
	sess, set, anchor, err := parseSession("RTH")
	if err != nil || sess != session.RTH || set != session.SetRTH || anchor != aggregation.AnchorRTHOpen {
		t.Fatalf("RTH: %v %v %v %v", sess, set, anchor, err)
	}
	if _, _, _, err := parseSession("rth"); err == nil {
		t.Fatal("lowercase rth should be rejected")
	}
	d, err := parseInterval("5m")
	if err != nil || d != 5*time.Minute {
		t.Fatalf("5m = %s err=%v", d, err)
	}
}

func TestLookupInstrument(t *testing.T) {
	if _, err := lookupInstrument("NQZ5"); err == nil {
		t.Fatal("unknown symbol should error")
	}
}

func esCal(t *testing.T) session.Calendar {
	t.Helper()
	cal, err := session.ForProduct("ES")
	if err != nil {
		t.Fatal(err)
	}
	return cal
}

func testServer(t *testing.T) *server {
	t.Helper()
	cal := esCal(t)
	day := time.Date(2025, time.September, 23, 0, 0, 0, 0, cal.Location)
	trades := []marketdata.Event{
		{
			Kind:    marketdata.KindTrade,
			TsEvent: time.Date(2025, time.September, 23, 8, 30, 0, 0, cal.Location).UnixNano(),
			Trade:   marketdata.Trade{Px: 26800, Qty: 2, Aggressor: core.SideBid},
		},
		{
			Kind:    marketdata.KindTrade,
			TsEvent: time.Date(2025, time.September, 23, 8, 31, 0, 0, cal.Location).UnixNano(),
			Trade:   marketdata.Trade{Px: 26804, Qty: 1, Aggressor: core.SideAsk},
		},
	}
	return &server{
		inst:     core.ESZ5(),
		cal:      cal,
		date:     day,
		interval: 5 * time.Minute,
		defSess:  "RTH",
		trades:   trades,
		views:    map[session.Session]chart.View{},
	}
}

func TestAPIViewJSONAndSVGAreSameView(t *testing.T) {
	h := newMux(testServer(t))
	js := httptest.NewRecorder()
	h.ServeHTTP(js, httptest.NewRequest("GET", "/api/view?session=RTH", nil))
	if js.Code != 200 {
		t.Fatalf("JSON status %d: %s", js.Code, js.Body.String())
	}
	if ct := js.Header().Get("Content-Type"); !strings.Contains(ct, "application/json") {
		t.Fatalf("JSON content-type %q", ct)
	}
	v, err := chart.ReadJSON(js.Body)
	if err != nil {
		t.Fatal(err)
	}
	if len(v.Bars) != 1 || v.Bars[0].Open != 26800 || v.Header.Session != session.RTH {
		t.Fatalf("JSON view = %+v", v.Header)
	}

	svg := httptest.NewRecorder()
	h.ServeHTTP(svg, httptest.NewRequest("GET", "/api/view.svg?session=RTH", nil))
	if svg.Code != 200 {
		t.Fatalf("SVG status %d: %s", svg.Code, svg.Body.String())
	}
	body := svg.Body.String()
	if !strings.Contains(body, "<svg") || !strings.Contains(body, "ESZ5") || !strings.Contains(body, "RTH") {
		t.Fatal("SVG is not the same page as the JSON View")
	}
}

func TestAPIDefaultSessionAndBadSession(t *testing.T) {
	h := newMux(testServer(t))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/api/view", nil))
	if rec.Code != 200 {
		t.Fatalf("default session status %d", rec.Code)
	}
	bad := httptest.NewRecorder()
	h.ServeHTTP(bad, httptest.NewRequest("GET", "/api/view?session=rth", nil))
	if bad.Code != http.StatusBadRequest {
		t.Fatalf("bad session status %d, want 400", bad.Code)
	}
}

func TestEmbedIndexAndChartJS(t *testing.T) {
	h := newMux(testServer(t))
	idx := httptest.NewRecorder()
	h.ServeHTTP(idx, httptest.NewRequest("GET", "/", nil))
	if idx.Code != 200 || !strings.Contains(idx.Body.String(), "chart.js") {
		t.Fatalf("index status %d body %q", idx.Code, idx.Body.String())
	}
	js := httptest.NewRecorder()
	h.ServeHTTP(js, httptest.NewRequest("GET", "/chart.js", nil))
	if js.Code != 200 {
		t.Fatalf("chart.js status %d", js.Code)
	}
	src := js.Body.String()
	if !strings.Contains(src, "bar index") {
		t.Fatal("chart.js must say x is the bar index")
	}
	if !strings.Contains(src, "wheel") {
		t.Fatal("chart.js missing wheel zoom")
	}
}

func TestETHQueryRebuildsFromSameTrades(t *testing.T) {
	cal := esCal(t)
	day := time.Date(2025, time.September, 23, 0, 0, 0, 0, cal.Location)
	trades := []marketdata.Event{
		{
			Kind:    marketdata.KindTrade,
			TsEvent: time.Date(2025, time.September, 22, 18, 0, 0, 0, cal.Location).UnixNano(),
			Trade:   marketdata.Trade{Px: 26790, Qty: 1, Aggressor: core.SideBid},
		},
		{
			Kind:    marketdata.KindTrade,
			TsEvent: time.Date(2025, time.September, 23, 9, 0, 0, 0, cal.Location).UnixNano(),
			Trade:   marketdata.Trade{Px: 26800, Qty: 1, Aggressor: core.SideBid},
		},
	}
	s := &server{
		inst:     core.ESZ5(),
		cal:      cal,
		date:     day,
		interval: 5 * time.Minute,
		defSess:  "RTH",
		trades:   trades,
		views:    map[session.Session]chart.View{},
	}
	h := newMux(s)
	rth := httptest.NewRecorder()
	h.ServeHTTP(rth, httptest.NewRequest("GET", "/api/view?session=RTH", nil))
	eth := httptest.NewRecorder()
	h.ServeHTTP(eth, httptest.NewRequest("GET", "/api/view?session=ETH", nil))
	if rth.Code != 200 || eth.Code != 200 {
		t.Fatalf("RTH=%d ETH=%d rth=%s eth=%s", rth.Code, eth.Code, rth.Body.String(), eth.Body.String())
	}
	vr, err := chart.ReadJSON(rth.Body)
	if err != nil {
		t.Fatal(err)
	}
	ve, err := chart.ReadJSON(eth.Body)
	if err != nil {
		t.Fatal(err)
	}
	if vr.Header.Session != session.RTH || ve.Header.Session != session.ETH {
		t.Fatalf("sessions %s / %s", vr.Header.Session, ve.Header.Session)
	}
	if vr.Bars[0].Open == ve.Bars[0].Open {
		t.Fatal("RTH and ETH should not be the same first bar")
	}
}

func TestComposeCollectThenFromTrades(t *testing.T) {
	cal := esCal(t)
	day := time.Date(2025, time.September, 23, 0, 0, 0, 0, cal.Location)
	src := &sliceSrc{evs: []marketdata.Event{{
		Kind:    marketdata.KindTrade,
		TsEvent: time.Date(2025, time.September, 23, 8, 30, 0, 0, cal.Location).UnixNano(),
		Trade:   marketdata.Trade{Px: 26800, Qty: 1, Aggressor: core.SideBid},
	}}}
	trades, err := compose.Collect(src, cal, day, 0)
	if err != nil || len(trades) != 1 {
		t.Fatalf("collect = %d err=%v", len(trades), err)
	}
	v, err := compose.FromTrades(trades, core.ESZ5(), cal, compose.Spec{
		Date:     day,
		Session:  session.RTH,
		Sessions: session.SetRTH,
		Anchor:   aggregation.AnchorRTHOpen,
		Interval: 5 * time.Minute,
	})
	if err != nil || len(v.Bars) != 1 {
		t.Fatalf("from trades: bars=%d err=%v", len(v.Bars), err)
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
