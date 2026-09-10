package aggregation

import (
	"github.com/morteza-sakifard/TickEngine/internal/core"
	"github.com/morteza-sakifard/TickEngine/internal/marketdata"
	"github.com/morteza-sakifard/TickEngine/internal/session"
)

type rangeBuilder struct {
	spec BarSpec
	cal  session.Calendar
	sink
}

func (b *rangeBuilder) Add(ev *marketdata.Event) {
	t, px, qty, side, ok := tradeOf(ev)
	if !ok {
		return
	}
	td, sess, ok := accept(b.cal, b.spec.Sessions, t)
	if !ok {
		return
	}
	if b.open != nil && rangeExceeds(b.open, px, b.spec.Band) {
		b.Flush()
	}
	if b.open == nil {
		b.open = &Bar{Start: t, End: t, TradingDate: td, Session: sess}
	}
	b.open.apply(px, qty, side)
	b.open.End = t
}

func rangeExceeds(bar *Bar, px, band core.Ticks) bool {
	if bar.TradeCount == 0 {
		return false
	}
	high, low := bar.High, bar.Low
	if px > high {
		high = px
	}
	if px < low {
		low = px
	}
	return high-low > band
}
