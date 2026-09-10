package chart

import (
	"strings"
	"testing"

	"github.com/morteza-sakifard/TickEngine/internal/aggregation"
	"github.com/morteza-sakifard/TickEngine/internal/core"
)

func TestIndexByTime(t *testing.T) {
	bars := basicView(t).Bars
	if i := IndexByTime(bars[1].Start.UnixNano(), bars); i != 1 {
		t.Fatalf("index of bar 1 start = %d", i)
	}
	if i := IndexByTime(bars[1].End.UnixNano()-1, bars); i != 1 {
		t.Fatalf("index just before bar 1 end = %d", i)
	}
	if i := IndexByTime(bars[0].Start.UnixNano()-1, bars); i != 0 {
		t.Fatalf("before first = %d", i)
	}
	if i := IndexByTime(bars[len(bars)-1].End.UnixNano()+1, bars); i != len(bars)-1 {
		t.Fatalf("after last = %d", i)
	}
}

func TestMarksDrawnOnSVG(t *testing.T) {
	v := basicView(t)
	v.Marks = []Mark{
		{Index: 0, Px: 26800, Qty: 1, Side: core.SideBid},
		{Index: 2, Px: 26818, Qty: 1, Side: core.SideAsk},
	}
	svg := render(t, v, Options{Width: 800, Height: 480, Location: chicago(t)})
	if strings.Count(svg, `class="mark"`) != 2 {
		t.Fatalf("want 2 marks, svg:\n%s", svg)
	}
	if !strings.Contains(svg, `data-side="bid"`) || !strings.Contains(svg, `data-side="ask"`) {
		t.Fatal("marks missing side")
	}
	if !strings.Contains(svg, `data-px="26800"`) {
		t.Fatal("buy mark missing price")
	}
}

func TestEmptyMarksLeaveGoldenLayout(t *testing.T) {
	v := basicView(t)
	svg := render(t, v, Options{Width: 800, Height: 480, Location: chicago(t)})
	if strings.Contains(svg, `class="mark"`) {
		t.Fatal("empty Marks must not emit polygons")
	}
}

func TestMarksJSONRoundTrip(t *testing.T) {
	v := basicView(t)
	v.Marks = []Mark{{Index: 1, Px: 100, Qty: 2, Side: core.SideAsk}}
	var buf strings.Builder
	if err := WriteJSON(&buf, v); err != nil {
		t.Fatal(err)
	}
	got, err := ReadJSON(strings.NewReader(buf.String()))
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Marks) != 1 || got.Marks[0].Px != 100 || got.Marks[0].Side != core.SideAsk {
		t.Fatalf("round-trip marks = %+v", got.Marks)
	}
}

func TestIndexByTimeEmpty(t *testing.T) {
	if IndexByTime(1, nil) != 0 {
		t.Fatal("empty bars")
	}
}

func TestIndexOnSingleBar(t *testing.T) {
	b := []aggregation.Bar{bar(t, 8, 30, 1, 2, 0, 1)}
	if IndexByTime(b[0].Start.UnixNano(), b) != 0 {
		t.Fatal("single bar")
	}
}
