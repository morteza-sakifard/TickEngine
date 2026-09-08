package chart

import (
	"bytes"
	"strings"
	"testing"

	"github.com/morteza-sakifard/market-data-lab/internal/session"
)

func TestWriteJSONDeterministic(t *testing.T) {
	v := basicView(t)
	var a, b bytes.Buffer
	if err := WriteJSON(&a, v); err != nil {
		t.Fatal(err)
	}
	if err := WriteJSON(&b, v); err != nil {
		t.Fatal(err)
	}
	if a.String() != b.String() {
		t.Fatal("WriteJSON is not byte-identical across two calls")
	}
}

func TestSameViewWritesJSONAndSVG(t *testing.T) {
	v := basicView(t)
	var js, svg bytes.Buffer
	if err := WriteJSON(&js, v); err != nil {
		t.Fatal(err)
	}
	if err := RenderSVG(&svg, v, Options{Width: 800, Height: 480, Location: chicago(t)}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(js.String(), `"Symbol":"ESZ5"`) {
		t.Fatalf("JSON missing ESZ5: %s", js.String())
	}
	if !strings.Contains(js.String(), `"Session":"RTH"`) {
		t.Fatal("Session must encode as the name RTH, not the iota")
	}
	if !strings.Contains(js.String(), `"Open":26800`) {
		t.Fatal("JSON missing first-bar Open in ticks")
	}
	if !strings.Contains(svg.String(), "<svg") || !strings.Contains(svg.String(), "ESZ5") {
		t.Fatal("same View did not render as SVG")
	}
	got, err := ReadJSON(bytes.NewReader(js.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Bars) != len(v.Bars) {
		t.Fatalf("round-trip bars = %d, want %d", len(got.Bars), len(v.Bars))
	}
	if got.Bars[0].Open != v.Bars[0].Open || got.Bars[0].Close != v.Bars[0].Close {
		t.Fatalf("round-trip OHLC drifted: %+v", got.Bars[0])
	}
	if got.Header.Session != session.RTH {
		t.Fatalf("round-trip Session = %v, want RTH", got.Header.Session)
	}
}

func TestWriteJSONNilSideViewsAreNull(t *testing.T) {
	v := basicView(t)
	var buf bytes.Buffer
	if err := WriteJSON(&buf, v); err != nil {
		t.Fatal(err)
	}
	s := buf.String()
	if !strings.Contains(s, `"Profile":null`) || !strings.Contains(s, `"Footprint":null`) || !strings.Contains(s, `"TPO":null`) {
		t.Fatalf("nil side views should stay null so the step-8 layout is visible on the wire: %s", s)
	}
}
