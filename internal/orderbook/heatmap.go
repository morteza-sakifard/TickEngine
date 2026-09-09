package orderbook

import (
	"sort"

	"github.com/morteza-sakifard/market-data-lab/internal/core"
	"github.com/morteza-sakifard/market-data-lab/internal/marketdata"
)

// HeatCell is one grouped price in one time column. Bid and Ask are
// the largest resting size seen at that bucket while the column was
// open — a wall that pulls still leaves a mark.
type HeatCell struct {
	Price    core.Ticks
	Bid, Ask core.Qty
}

// HeatColumn is one time slice, priced rows sorted ascending.
type HeatColumn struct {
	Cells []HeatCell
}

// DOMLevel is one ladder row. Price descending on the ladder. Last
// is the most recent trade size at that grouped price, not a live
// tape.
type DOMLevel struct {
	Price    core.Ticks
	Bid, Ask core.Qty
	Last     core.Qty
}

type heatQty struct{ bid, ask core.Qty }

// Heatmap records the book through time. TicksPerRow is the price
// grouping. Cut closes a column; the type does not know what a bar
// is. DOM is the live book plus last prints, not a column.
type Heatmap struct {
	group   int
	book    L2
	lastPx  core.Ticks
	lastQty core.Qty
	lastAt  map[core.Ticks]core.Qty
	cur     map[core.Ticks]heatQty
	cols    []HeatColumn
}

func NewHeatmap(ticksPerRow int) *Heatmap {
	if ticksPerRow < 1 {
		ticksPerRow = 1
	}
	return &Heatmap{group: ticksPerRow}
}

func (h *Heatmap) TicksPerRow() int {
	if h == nil || h.group < 1 {
		return 1
	}
	return h.group
}

func (h *Heatmap) Reset() {
	if h == nil {
		return
	}
	g := h.group
	if g < 1 {
		g = 1
	}
	*h = Heatmap{group: g}
}

func (h *Heatmap) OnEvent(ev *marketdata.Event) {
	if h == nil || ev == nil {
		return
	}
	switch ev.Kind {
	case marketdata.KindTrade:
		if ev.Trade.Qty <= 0 {
			return
		}
		g := groupPx(ev.Trade.Px, h.TicksPerRow())
		h.lastPx, h.lastQty = ev.Trade.Px, ev.Trade.Qty
		if h.lastAt == nil {
			h.lastAt = make(map[core.Ticks]core.Qty)
		}
		h.lastAt[g] = ev.Trade.Qty
	case marketdata.KindQuote:
		h.book.Apply(ev)
		h.paint()
	}
}

func (h *Heatmap) paint() {
	n := h.TicksPerRow()
	snap := groupedBook(&h.book, n)
	if h.cur == nil {
		h.cur = make(map[core.Ticks]heatQty, len(snap))
	}
	for px, v := range snap {
		c := h.cur[px]
		if v.bid > c.bid {
			c.bid = v.bid
		}
		if v.ask > c.ask {
			c.ask = v.ask
		}
		h.cur[px] = c
	}
}

// Cut seals the open column, including an empty one so a caller
// aligning with bars keeps indexes matched.
func (h *Heatmap) Cut() {
	if h == nil {
		return
	}
	h.cols = append(h.cols, freeze(h.cur))
	h.cur = nil
}

func (h *Heatmap) Columns() []HeatColumn {
	if h == nil {
		return nil
	}
	return h.cols
}

func (h *Heatmap) Last() (core.Ticks, core.Qty) {
	if h == nil {
		return 0, 0
	}
	return h.lastPx, h.lastQty
}

// DOM is the current grouped book plus last size at each price,
// high price first (asks above bids).
func (h *Heatmap) DOM() []DOMLevel {
	if h == nil {
		return nil
	}
	n := h.TicksPerRow()
	book := groupedBook(&h.book, n)
	seen := make(map[core.Ticks]struct{}, len(book)+len(h.lastAt))
	pxs := make([]core.Ticks, 0, len(book)+len(h.lastAt))
	for px := range book {
		if _, ok := seen[px]; ok {
			continue
		}
		seen[px] = struct{}{}
		pxs = append(pxs, px)
	}
	for px := range h.lastAt {
		if _, ok := seen[px]; ok {
			continue
		}
		seen[px] = struct{}{}
		pxs = append(pxs, px)
	}
	if len(pxs) == 0 {
		return nil
	}
	sort.Slice(pxs, func(i, j int) bool { return pxs[i] > pxs[j] })
	out := make([]DOMLevel, len(pxs))
	for i, px := range pxs {
		q := book[px]
		out[i] = DOMLevel{Price: px, Bid: q.bid, Ask: q.ask, Last: h.lastAt[px]}
	}
	return out
}

func groupPx(px core.Ticks, n int) core.Ticks {
	if n <= 1 {
		return px
	}
	step := core.Ticks(n)
	return px - px%step
}

func groupedBook(book *L2, n int) map[core.Ticks]heatQty {
	m := make(map[core.Ticks]heatQty)
	if book == nil {
		return m
	}
	for _, lv := range book.Bids(marketdata.MaxDepth) {
		g := groupPx(lv.Px, n)
		c := m[g]
		c.bid += lv.Qty
		m[g] = c
	}
	for _, lv := range book.Asks(marketdata.MaxDepth) {
		g := groupPx(lv.Px, n)
		c := m[g]
		c.ask += lv.Qty
		m[g] = c
	}
	return m
}

func freeze(cur map[core.Ticks]heatQty) HeatColumn {
	if len(cur) == 0 {
		return HeatColumn{}
	}
	pxs := make([]core.Ticks, 0, len(cur))
	for px := range cur {
		pxs = append(pxs, px)
	}
	sort.Slice(pxs, func(i, j int) bool { return pxs[i] < pxs[j] })
	cells := make([]HeatCell, len(pxs))
	for i, px := range pxs {
		q := cur[px]
		cells[i] = HeatCell{Price: px, Bid: q.bid, Ask: q.ask}
	}
	return HeatColumn{Cells: cells}
}
