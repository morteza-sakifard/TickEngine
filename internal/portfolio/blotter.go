package portfolio

import (
	"fmt"

	"github.com/morteza-sakifard/TickEngine/internal/core"
	"github.com/morteza-sakifard/TickEngine/internal/execution"
)

// Entry is one fill as the blotter saw it. PnL is the change in
// realized cents on this ticket — the story Sharpe cannot tell
// (bought above VWAP, sold inside the spread).
type Entry struct {
	Ts       int64
	OrderID  execution.OrderID
	Side     core.Side
	Px       core.Ticks
	Qty      core.Qty
	FeeCents int64
	PnL      int64
	Realized int64
	QtyAfter core.Qty
}

func (e Entry) Line() string {
	return fmt.Sprintf("fill ts=%d id=%d side=%s px=%d qty=%d fee=%d pnl=%d realized=%d pos=%d\n",
		e.Ts, e.OrderID, e.Side, e.Px, e.Qty, e.FeeCents, e.PnL, e.Realized, e.QtyAfter)
}

// Blotter is the fill tape. Metrics and chart marks both read this
// list; they do not re-simulate the venue.
type Blotter struct {
	entries      []Entry
	openRealized int64
	trades       []int64
}

func (b *Blotter) Record(f execution.Fill, before, after Position) {
	if b == nil {
		return
	}
	e := Entry{
		Ts:       f.Ts,
		OrderID:  f.OrderID,
		Side:     f.Side,
		Px:       f.Px,
		Qty:      f.Qty,
		FeeCents: f.FeeCents,
		PnL:      after.Realized - before.Realized,
		Realized: after.Realized,
		QtyAfter: after.Qty,
	}
	b.entries = append(b.entries, e)
	b.noteTrade(before, after)
}

func (b *Blotter) noteTrade(before, after Position) {
	if before.Qty == 0 && after.Qty != 0 {
		b.openRealized = before.Realized
		return
	}
	if before.Qty != 0 && after.Qty == 0 {
		b.trades = append(b.trades, after.Realized-b.openRealized)
		return
	}
	if before.Qty != 0 && after.Qty != 0 && !sameWay(before.Qty, after.Qty) {
		b.trades = append(b.trades, after.Realized-b.openRealized)
		b.openRealized = after.Realized
		return
	}
	if before.Qty != 0 && absQty(after.Qty) < absQty(before.Qty) {
		b.trades = append(b.trades, after.Realized-before.Realized)
	}
}

func (b *Blotter) Entries() []Entry {
	if b == nil {
		return nil
	}
	out := make([]Entry, len(b.entries))
	copy(out, b.entries)
	return out
}

func (b *Blotter) Text() string {
	if b == nil {
		return ""
	}
	var s string
	for _, e := range b.entries {
		s += e.Line()
	}
	return s
}

func (b *Blotter) Metrics() Metrics {
	if b == nil {
		return Metrics{}
	}
	return computeMetrics(b.entries, b.trades)
}
