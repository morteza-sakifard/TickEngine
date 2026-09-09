package chart

import (
	"strings"
	"testing"
)

func TestHeatmapAndDOMDrawn(t *testing.T) {
	v := basicView(t)
	v.Heatmap = &HeatmapView{
		TicksPerRow: 1,
		Columns: []HeatColumn{{
			Cells: []HeatCell{
				{Price: 26800, Bid: 5, Ask: 0},
				{Price: 26808, Bid: 0, Ask: 8},
			},
		}},
	}
	v.DOM = &DOMView{
		LastPx:  26808,
		LastQty: 2,
		Levels: []DOMLevel{
			{Price: 26808, Ask: 8, Last: 2},
			{Price: 26800, Bid: 5},
		},
	}
	svg := render(t, v, Options{Width: 800, Height: 480, Location: chicago(t)})
	if strings.Count(svg, `class="heat"`) != 2 {
		t.Fatalf("want 2 heat cells, svg:\n%s", svg)
	}
	if !strings.Contains(svg, `class="dom"`) || !strings.Contains(svg, `data-side="last"`) {
		t.Fatal("DOM missing last print")
	}
	if !strings.Contains(svg, heatBidCol) || !strings.Contains(svg, heatAskCol) {
		t.Fatal("heatmap missing bid/ask colors")
	}
}

func TestEmptyHeatmapLeavesGoldenLayout(t *testing.T) {
	v := basicView(t)
	svg := render(t, v, Options{Width: 800, Height: 480, Location: chicago(t)})
	if strings.Contains(svg, `class="heat"`) || strings.Contains(svg, `class="dom"`) {
		t.Fatal("empty heatmap/DOM must not emit cells")
	}
}

func TestHeatmapJSONRoundTrip(t *testing.T) {
	v := basicView(t)
	v.Heatmap = &HeatmapView{TicksPerRow: 4, Columns: []HeatColumn{{Cells: []HeatCell{{Price: 100, Bid: 2}}}}}
	v.DOM = &DOMView{LastPx: 100, LastQty: 1, Levels: []DOMLevel{{Price: 100, Bid: 2, Last: 1}}}
	var buf strings.Builder
	if err := WriteJSON(&buf, v); err != nil {
		t.Fatal(err)
	}
	got, err := ReadJSON(strings.NewReader(buf.String()))
	if err != nil {
		t.Fatal(err)
	}
	if got.Heatmap == nil || got.Heatmap.TicksPerRow != 4 || got.Heatmap.Columns[0].Cells[0].Bid != 2 {
		t.Fatalf("heatmap = %+v", got.Heatmap)
	}
	if got.DOM == nil || got.DOM.LastPx != 100 || got.DOM.Levels[0].Last != 1 {
		t.Fatalf("dom = %+v", got.DOM)
	}
}
