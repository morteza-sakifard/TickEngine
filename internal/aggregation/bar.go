package aggregation

import (
	"time"

	"github.com/morteza-sakifard/market-data-lab/internal/core"
	"github.com/morteza-sakifard/market-data-lab/internal/marketdata"
	"github.com/morteza-sakifard/market-data-lab/internal/session"
)

type Bar struct {
	Start, End             time.Time
	TradingDate            time.Time
	Session                session.Session
	Open, High, Low, Close core.Ticks
	Volume                 core.Qty
	BuyVolume, SellVolume  core.Qty
	TradeCount             int64
}

func (b *Bar) apply(px core.Ticks, qty core.Qty, side core.Side) {
	if b.TradeCount == 0 {
		b.Open, b.High, b.Low, b.Close = px, px, px, px
	} else {
		if px > b.High {
			b.High = px
		}
		if px < b.Low {
			b.Low = px
		}
		b.Close = px
	}
	b.Volume += qty
	switch side {
	case core.SideBid:
		b.BuyVolume += qty
	case core.SideAsk:
		b.SellVolume += qty
	}
	b.TradeCount++
}

type sink struct {
	bars []Bar
	open *Bar
}

func (s *sink) Flush() {
	if s.open != nil {
		s.bars = append(s.bars, *s.open)
		s.open = nil
	}
}

func (s *sink) Bars() []Bar {
	out := make([]Bar, 0, len(s.bars)+1)
	out = append(out, s.bars...)
	if s.open != nil {
		out = append(out, *s.open)
	}
	return out
}

func accept(cal session.Calendar, sessions session.Set, t time.Time) (time.Time, session.Session, bool) {
	td, sess := cal.Classify(t)
	if !sessions.Contains(sess) {
		return time.Time{}, session.Closed, false
	}
	return td, sess, true
}

func tradeOf(ev *marketdata.Event) (t time.Time, px core.Ticks, qty core.Qty, side core.Side, ok bool) {
	if ev == nil || ev.Kind != marketdata.KindTrade {
		return time.Time{}, 0, 0, core.SideNone, false
	}
	return ev.EventTime(), ev.Trade.Px, ev.Trade.Qty, ev.Trade.Aggressor, true
}
