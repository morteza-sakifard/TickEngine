package chart

import (
	"bytes"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/morteza-sakifard/market-data-lab/internal/aggregation"
	"github.com/morteza-sakifard/market-data-lab/internal/core"
	"github.com/morteza-sakifard/market-data-lab/internal/session"
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
