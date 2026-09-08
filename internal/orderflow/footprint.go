package orderflow

import (
	"sort"

	"github.com/morteza-sakifard/market-data-lab/internal/core"
	"github.com/morteza-sakifard/market-data-lab/internal/marketdata"
)

// FootprintLevel is one bid×ask row. Buy is aggressive lifts (SideBid);
// Sell is aggressive hits (SideAsk). SideNone is not a side.
type FootprintLevel struct {
	Price     core.Ticks
	Buy, Sell core.Qty
}

// Imbalance is one diagonal ratio hit. Price is the cell to mark:
// the ask row P for an ask-dominant print, or the bid row P−n for a
// bid-dominant print. Ask(P) is compared with Bid(P−n) because those
// two quotes compete; px-1 is a Ticks subtraction, not a float.
type Imbalance struct {
	Price    core.Ticks
	Dir      core.Side
	Num, Den core.Qty
}

// Stack is minRun consecutive imbalances of one direction, stepping
// by ticksPerRow.
type Stack struct {
	Dir      core.Side
	From, To core.Ticks
	Count    int
}

// Footprint is bid×ask volume by price for one bar (or any window
// the caller Reset()s). It does not know what a bar is.
type Footprint struct {
	levels map[core.Ticks]footVol
}

type footVol struct{ buy, sell core.Qty }

func (f *Footprint) OnTrade(ev *marketdata.Event) {
	if ev == nil || ev.Kind != marketdata.KindTrade {
		return
	}
	qty := ev.Trade.Qty
	if qty <= 0 {
		return
	}
	if f.levels == nil {
		f.levels = make(map[core.Ticks]footVol)
	}
	v := f.levels[ev.Trade.Px]
	switch ev.Trade.Aggressor {
	case core.SideBid:
		v.buy += qty
	case core.SideAsk:
		v.sell += qty
	default:
		return
	}
	f.levels[ev.Trade.Px] = v
}

func (f *Footprint) Reset() { *f = Footprint{} }

// Levels returns ungrouped rows sorted by price.
func (f *Footprint) Levels() []FootprintLevel {
	return f.grouped(1)
}

// Grouped merges every ticksPerRow adjacent ticks into one row
// priced at the floor of the group. Volume is conserved: the sum of
// Buy and of Sell equals the ungrouped totals. ticksPerRow < 1 is 1.
func (f *Footprint) Grouped(ticksPerRow int) []FootprintLevel {
	if ticksPerRow < 1 {
		ticksPerRow = 1
	}
	return f.grouped(ticksPerRow)
}

func (f *Footprint) grouped(n int) []FootprintLevel {
	if len(f.levels) == 0 {
		return nil
	}
	if n <= 1 {
		out := make([]FootprintLevel, 0, len(f.levels))
		for px, v := range f.levels {
			out = append(out, FootprintLevel{Price: px, Buy: v.buy, Sell: v.sell})
		}
		sort.Slice(out, func(i, j int) bool { return out[i].Price < out[j].Price })
		return out
	}
	step := core.Ticks(n)
	acc := make(map[core.Ticks]footVol, len(f.levels))
	for px, v := range f.levels {
		g := px - px%step
		cur := acc[g]
		cur.buy += v.buy
		cur.sell += v.sell
		acc[g] = cur
	}
	out := make([]FootprintLevel, 0, len(acc))
	for px, v := range acc {
		out = append(out, FootprintLevel{Price: px, Buy: v.buy, Sell: v.sell})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Price < out[j].Price })
	return out
}

// Imbalances returns diagonal ratio hits on Grouped(ticksPerRow) rows.
// Ask-dominant: Buy(P) / Sell(P−n) >= ratio, marked at P.
// Bid-dominant: Sell(P−n) / Buy(P) >= ratio, marked at P−n.
// Both sides of the pair must be > 0 so the ratio is defined.
func (f *Footprint) Imbalances(ratio, ticksPerRow int) []Imbalance {
	if ratio < 1 {
		return nil
	}
	if ticksPerRow < 1 {
		ticksPerRow = 1
	}
	rows := f.grouped(ticksPerRow)
	if len(rows) == 0 {
		return nil
	}
	byPx := make(map[core.Ticks]FootprintLevel, len(rows))
	for _, lv := range rows {
		byPx[lv.Price] = lv
	}
	n := core.Ticks(ticksPerRow)
	need := core.Qty(ratio)
	var out []Imbalance
	for _, lv := range rows {
		ask := lv.Buy
		bid := byPx[lv.Price-n].Sell
		if ask <= 0 || bid <= 0 {
			continue
		}
		if ask >= need*bid {
			out = append(out, Imbalance{Price: lv.Price, Dir: core.SideAsk, Num: ask, Den: bid})
		}
		if bid >= need*ask {
			out = append(out, Imbalance{Price: lv.Price - n, Dir: core.SideBid, Num: bid, Den: ask})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Price != out[j].Price {
			return out[i].Price < out[j].Price
		}
		return out[i].Dir < out[j].Dir
	})
	return out
}

// StackedImbalances returns runs of at least minRun imbalances of
// the same direction whose marked prices step by ticksPerRow.
func (f *Footprint) StackedImbalances(minRun, ratio, ticksPerRow int) []Stack {
	if minRun < 1 {
		return nil
	}
	if ticksPerRow < 1 {
		ticksPerRow = 1
	}
	imbs := f.Imbalances(ratio, ticksPerRow)
	var out []Stack
	out = append(out, stacksOf(imbs, core.SideAsk, ticksPerRow, minRun)...)
	out = append(out, stacksOf(imbs, core.SideBid, ticksPerRow, minRun)...)
	return out
}

func stacksOf(imbs []Imbalance, dir core.Side, n, minRun int) []Stack {
	var px []core.Ticks
	for _, im := range imbs {
		if im.Dir == dir {
			px = append(px, im.Price)
		}
	}
	step := core.Ticks(n)
	var out []Stack
	i := 0
	for i < len(px) {
		j := i + 1
		for j < len(px) && px[j] == px[j-1]+step {
			j++
		}
		if j-i >= minRun {
			out = append(out, Stack{Dir: dir, From: px[i], To: px[j-1], Count: j - i})
		}
		i = j
	}
	return out
}
