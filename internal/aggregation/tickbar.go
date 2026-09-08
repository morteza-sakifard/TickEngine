package aggregation

import (
	"github.com/morteza-sakifard/market-data-lab/internal/marketdata"
	"github.com/morteza-sakifard/market-data-lab/internal/session"
)

type tickBuilder struct {
	spec BarSpec
	cal  session.Calendar
	sink
}

func (b *tickBuilder) Add(ev *marketdata.Event) {
	t, px, qty, side, ok := tradeOf(ev)
	if !ok {
		return
	}
	td, sess, ok := accept(b.cal, b.spec.Sessions, t)
	if !ok {
		return
	}
	if b.open == nil {
		b.open = &Bar{Start: t, End: t, TradingDate: td, Session: sess}
	}
	b.open.apply(px, qty, side)
	b.open.End = t
	if b.open.TradeCount >= b.spec.Count {
		b.Flush()
	}
}
