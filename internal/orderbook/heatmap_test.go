package orderbook

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/morteza-sakifard/TickEngine/internal/core"
	"github.com/morteza-sakifard/TickEngine/internal/feed/databento"
	"github.com/morteza-sakifard/TickEngine/internal/marketdata"
)

func qev(bidPx, askPx core.Ticks, bidQty, askQty core.Qty) marketdata.Event {
	q := marketdata.Quote{BidPx: bidPx, AskPx: askPx, BidQty: bidQty, AskQty: askQty, BidCt: 1, AskCt: 1}
	return marketdata.Event{Kind: marketdata.KindQuote, Quote: q, Depth: marketdata.DepthFromQuote(q)}
}

func TestHeatmapPriceGrouping(t *testing.T) {
	h := NewHeatmap(4)
	h.OnEvent(&marketdata.Event{
		Kind: marketdata.KindQuote,
		Depth: marketdata.Depth{
			Bids: [10]Level{{Px: 100, Qty: 2}, {Px: 101, Qty: 3}},
			Asks: [10]Level{{Px: 104, Qty: 4}, {Px: 105, Qty: 1}},
		},
	})
	h.Cut()
	cols := h.Columns()
	if len(cols) != 1 || len(cols[0].Cells) != 2 {
		t.Fatalf("grouped columns = %+v", cols)
	}
	c0, c1 := cols[0].Cells[0], cols[0].Cells[1]
	if c0.Price != 100 || c0.Bid != 5 || c0.Ask != 0 {
		t.Fatalf("group 100 = %+v", c0)
	}
	if c1.Price != 104 || c1.Ask != 5 || c1.Bid != 0 {
		t.Fatalf("group 104 = %+v", c1)
	}
}

func TestHeatmapPullLeavesMax(t *testing.T) {
	h := NewHeatmap(1)
	q1, q2 := qev(100, 104, 1, 8), qev(100, 104, 1, 1)
	h.OnEvent(&q1)
	h.OnEvent(&q2)
	h.Cut()
	cell := h.Columns()[0].Cells
	var ask core.Qty
	for _, c := range cell {
		if c.Price == 104 {
			ask = c.Ask
		}
	}
	if ask != 8 {
		t.Fatalf("pulled wall must leave max 8, got %d cells=%+v", ask, cell)
	}
	dom := h.DOM()
	var live core.Qty
	for _, lv := range dom {
		if lv.Price == 104 {
			live = lv.Ask
		}
	}
	if live != 1 {
		t.Fatalf("DOM is live book, ask=%d", live)
	}
}

func TestDOMBidAskLast(t *testing.T) {
	h := NewHeatmap(1)
	q := qev(100, 101, 5, 4)
	h.OnEvent(&q)
	h.OnEvent(&marketdata.Event{
		Kind:  marketdata.KindTrade,
		Trade: marketdata.Trade{Px: 101, Qty: 2, Aggressor: core.SideBid},
	})
	dom := h.DOM()
	if len(dom) != 2 || dom[0].Price != 101 || dom[1].Price != 100 {
		t.Fatalf("ladder high-to-low: %+v", dom)
	}
	if dom[0].Ask != 4 || dom[0].Last != 2 || dom[1].Bid != 5 || dom[1].Last != 0 {
		t.Fatalf("bid/ask/last = %+v", dom)
	}
	px, qty := h.Last()
	if px != 101 || qty != 2 {
		t.Fatalf("last print %d %d", px, qty)
	}
}

func TestEmptyCutKeepsIndex(t *testing.T) {
	h := NewHeatmap(1)
	h.Cut()
	q := qev(1, 2, 1, 1)
	h.OnEvent(&q)
	h.Cut()
	if n := len(h.Columns()); n != 2 || len(h.Columns()[0].Cells) != 0 {
		t.Fatalf("want empty then one column, got %+v", h.Columns())
	}
}

func TestLiquidityAroundTouchOnFixture(t *testing.T) {
	path := filepath.Join("..", "..", "testdata", "mbp1_sample.csv")
	f, err := os.Open(path)
	if err != nil {
		t.Skip(err)
	}
	dec, err := databento.NewDecoder(f, core.ESZ5())
	if err != nil {
		f.Close()
		t.Fatal(err)
	}
	defer dec.Close()

	h := NewHeatmap(1)
	var ev marketdata.Event
	var lastQ marketdata.Quote
	n := 0
	for {
		err := dec.Next(&ev)
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		h.OnEvent(&ev)
		if ev.Kind == marketdata.KindQuote {
			lastQ = ev.Quote
			n++
		}
	}
	if n == 0 {
		t.Fatal("no quotes")
	}
	h.Cut()
	touch := map[core.Ticks]bool{lastQ.BidPx: true, lastQ.AskPx: true}
	var near, far core.Qty
	for _, lv := range h.DOM() {
		if lv.Bid+lv.Ask == 0 {
			continue
		}
		if touch[lv.Price] {
			near += lv.Bid + lv.Ask
			continue
		}
		far += lv.Bid + lv.Ask
	}
	if near == 0 {
		t.Fatal("no size at the touch")
	}
	if far != 0 {
		t.Fatalf("MBP-1 size must sit at the touch, far=%d near=%d", far, near)
	}
}
