package chart

import (
	"bytes"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/morteza-sakifard/TickEngine/internal/aggregation"
	"github.com/morteza-sakifard/TickEngine/internal/core"
	"github.com/morteza-sakifard/TickEngine/internal/session"
)

func TestWriteFrameDeterministic(t *testing.T) {
	inst := core.ESZ5()
	f := Frame{
		Type:       FrameHello,
		Instrument: &inst,
		Header: &Header{
			Symbol:      "ESZ5",
			TradingDate: time.Date(2025, time.September, 22, 0, 0, 0, 0, time.UTC),
			Session:     session.RTH,
		},
	}
	var a, b bytes.Buffer
	if err := WriteFrame(&a, f); err != nil {
		t.Fatal(err)
	}
	if err := WriteFrame(&b, f); err != nil {
		t.Fatal(err)
	}
	if a.String() != b.String() {
		t.Fatal("WriteFrame is not byte-identical across two calls")
	}
}

func TestWriteFrameBarRoundTrip(t *testing.T) {
	bar := aggregation.Bar{Open: 26800, High: 26804, Low: 26796, Close: 26802, Volume: 3}
	var buf bytes.Buffer
	if err := WriteFrame(&buf, Frame{
		Type:  FrameBar,
		Index: 2,
		Bar:   &bar,
		VWAP:  26801,
		CVD:   4,
	}); err != nil {
		t.Fatal(err)
	}
	got, err := ReadFrame(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if got.Type != FrameBar || got.Index != 2 || got.Bar == nil {
		t.Fatalf("got %+v", got)
	}
	if got.Bar.Open != 26800 || got.VWAP != 26801 || got.CVD != 4 {
		t.Fatalf("bar/vwap/cvd drifted: %+v", got)
	}
}

func TestWriteFrameCarriesOrderFlow(t *testing.T) {
	fp := BarFootprint{Levels: []FootprintLevel{{Price: 26800, Buy: 2, Sell: 1}}}
	var buf bytes.Buffer
	if err := WriteFrame(&buf, Frame{
		Type:      FrameBar,
		Index:     0,
		Footprint: &fp,
		Trade:     &TapePrint{TsEvent: 1, Px: 26800, Qty: 2, Side: core.SideBid},
		Profile:   &ProfileView{POC: 26800, VAL: 26796, VAH: 26804, Levels: []ProfileLevel{{Price: 26800, Volume: 3}}},
		TPO:       &TPOView{POC: 26800, PeriodCount: 1, Levels: []TPOViewLevel{{Price: 26800, Letters: "A"}}},
	}); err != nil {
		t.Fatal(err)
	}
	got, err := ReadFrame(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if got.Footprint == nil || got.Footprint.Levels[0].Buy != 2 {
		t.Fatalf("footprint = %+v", got.Footprint)
	}
	if got.Trade == nil || got.Trade.Side != core.SideBid || got.Trade.Qty != 2 {
		t.Fatalf("trade = %+v", got.Trade)
	}
	if got.Profile == nil || got.Profile.POC != 26800 || got.TPO == nil || got.TPO.Levels[0].Letters != "A" {
		t.Fatalf("profile/tpo = %+v %+v", got.Profile, got.TPO)
	}
}

func TestStreamFileHasNoConcurrencyPrimitives(t *testing.T) {
	b, err := os.ReadFile("stream.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, bad := range []string{"\ngo ", " chan ", "\tchan ", "\nselect ", " select ", "time.Now("} {
		if strings.Contains(s, bad) {
			t.Errorf("stream.go contains %q", strings.TrimSpace(bad))
		}
	}
}
