package aggregation

import (
	"github.com/morteza-sakifard/TickEngine/internal/core"
	"github.com/morteza-sakifard/TickEngine/internal/marketdata"
	"github.com/morteza-sakifard/TickEngine/internal/session"
)

type volumeBuilder struct {
	spec BarSpec
	cal  session.Calendar
	sink
}

func (b *volumeBuilder) Add(ev *marketdata.Event) {
	t, px, qty, side, ok := tradeOf(ev)
	if !ok || qty <= 0 {
		return
	}
	td, sess, ok := accept(b.cal, b.spec.Sessions, t)
	if !ok {
		return
	}
	left := qty
	for left > 0 {
		if b.open == nil {
			b.open = &Bar{Start: t, End: t, TradingDate: td, Session: sess}
		}
		room := core.Qty(b.spec.Count) - b.open.Volume
		if room <= 0 {
			b.Flush()
			continue
		}
		take := left
		if take > room {
			take = room
		}
		b.open.apply(px, take, side)
		b.open.End = t
		left -= take
		if b.open.Volume >= core.Qty(b.spec.Count) {
			b.Flush()
		}
	}
}
