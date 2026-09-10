package orderflow

import (
	"testing"
	"time"

	"github.com/morteza-sakifard/TickEngine/internal/core"
)

func addFP(fp *Footprint, px, qty int64, side core.Side) {
	e := tr(px, qty, side, time.Time{})
	fp.OnTrade(&e)
}

func TestFootprintEmpty(t *testing.T) {
	var fp Footprint
	if got := fp.Levels(); len(got) != 0 {
		t.Fatalf("Levels = %v, want empty", got)
	}
}

func TestFootprintBuySellByPrice(t *testing.T) {
	var fp Footprint
	addFP(&fp, 100, 5, core.SideBid)
	addFP(&fp, 101, 3, core.SideAsk)
	addFP(&fp, 100, 2, core.SideAsk)
	addFP(&fp, 100, 1, core.SideBid)
	addFP(&fp, 100, 9, core.SideNone)
	got := fp.Levels()
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	if got[0] != (FootprintLevel{Price: 100, Buy: 6, Sell: 2}) {
		t.Fatalf("levels[0] = %+v, want buy 6 sell 2 at 100", got[0])
	}
	if got[1] != (FootprintLevel{Price: 101, Buy: 0, Sell: 3}) {
		t.Fatalf("levels[1] = %+v", got[1])
	}
}

func TestFootprintGroupedPreservesVolume(t *testing.T) {
	var fp Footprint
	addFP(&fp, 100, 1, core.SideBid)
	addFP(&fp, 101, 1, core.SideBid)
	addFP(&fp, 102, 1, core.SideAsk)
	addFP(&fp, 103, 1, core.SideBid)
	addFP(&fp, 104, 2, core.SideBid)
	var rawBuy, rawSell core.Qty
	for _, lv := range fp.Levels() {
		rawBuy += lv.Buy
		rawSell += lv.Sell
	}
	got := fp.Grouped(4)
	if len(got) != 2 || got[0].Price != 100 || got[1].Price != 104 {
		t.Fatalf("Grouped(4) = %+v, want rows at 100 and 104", got)
	}
	if got[0].Buy != 3 || got[0].Sell != 1 || got[1].Buy != 2 || got[1].Sell != 0 {
		t.Fatalf("grouped volumes = %+v", got)
	}
	var gBuy, gSell core.Qty
	for _, lv := range got {
		gBuy += lv.Buy
		gSell += lv.Sell
	}
	if gBuy != rawBuy || gSell != rawSell {
		t.Fatalf("grouped %d/%d != raw %d/%d", gBuy, gSell, rawBuy, rawSell)
	}
}

func TestFootprintDiagonalImbalance(t *testing.T) {
	var fp Footprint
	addFP(&fp, 100, 1, core.SideAsk)
	addFP(&fp, 101, 3, core.SideBid)
	imbs := fp.Imbalances(3, 1)
	if len(imbs) != 1 || imbs[0].Price != 101 || imbs[0].Dir != core.SideAsk {
		t.Fatalf("ask imbalance = %+v, want one SideAsk at 101", imbs)
	}

	var bid Footprint
	addFP(&bid, 100, 3, core.SideAsk)
	addFP(&bid, 101, 1, core.SideBid)
	imbs = bid.Imbalances(3, 1)
	if len(imbs) != 1 || imbs[0].Price != 100 || imbs[0].Dir != core.SideBid {
		t.Fatalf("bid imbalance = %+v, want one SideBid at 100", imbs)
	}
}

func TestFootprintStackedImbalances(t *testing.T) {
	var fp Footprint
	// Ask(P) = 3, Bid(P-1) = 1 for P = 11,12,13.
	addFP(&fp, 10, 1, core.SideAsk)
	addFP(&fp, 11, 3, core.SideBid)
	addFP(&fp, 11, 1, core.SideAsk)
	addFP(&fp, 12, 3, core.SideBid)
	addFP(&fp, 12, 1, core.SideAsk)
	addFP(&fp, 13, 3, core.SideBid)
	stacks := fp.StackedImbalances(3, 3, 1)
	if len(stacks) != 1 {
		t.Fatalf("stacks = %+v, want one ask run", stacks)
	}
	if stacks[0].Dir != core.SideAsk || stacks[0].From != 11 || stacks[0].To != 13 || stacks[0].Count != 3 {
		t.Fatalf("stack = %+v, want Ask 11..13 count 3", stacks[0])
	}
	if got := fp.StackedImbalances(4, 3, 1); len(got) != 0 {
		t.Fatalf("minRun 4 = %+v, want none", got)
	}
}

func TestFootprintReset(t *testing.T) {
	var fp Footprint
	addFP(&fp, 26800, 2, core.SideBid)
	fp.Reset()
	if len(fp.Levels()) != 0 {
		t.Fatal("Reset did not clear levels")
	}
	addFP(&fp, 26800, 2, core.SideBid)
	fresh := Footprint{}
	addFP(&fresh, 26800, 2, core.SideBid)
	if fp.Levels()[0] != fresh.Levels()[0] {
		t.Fatal("after Reset+OnTrade, footprint differs from a fresh one")
	}
}
