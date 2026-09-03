package ohlcv

import (
	"time"

	"github.com/morteza-sakifard/market-data-lab/internal/session"
	"github.com/morteza-sakifard/market-data-lab/internal/trade"
)

type Bar struct {
	BucketStart time.Time
	Symbol      string
	TradingDate time.Time
	Session     session.Session

	Open, High, Low, Close float64
	Volume                 int64
}

type Builder struct {
	Symbol   string
	Interval time.Duration
	Calendar session.Calendar

	bars []Bar
	open *Bar
}

func (b *Builder) Add(t trade.Trade) {
	start := bucketStart(t.Time, b.Interval)

	if b.open == nil || !b.open.BucketStart.Equal(start) {
		b.closeOpenBar()
		tradingDate, sess := b.Calendar.Classify(start)
		b.open = &Bar{
			BucketStart: start,
			Symbol:      b.Symbol,
			TradingDate: tradingDate,
			Session:     sess,
			Open:        t.Price,
			High:        t.Price,
			Low:         t.Price,
			Close:       t.Price,
			Volume:      int64(t.Size),
		}
		return
	}

	switch {
	case t.Price > b.open.High:
		b.open.High = t.Price
	case t.Price < b.open.Low:
		b.open.Low = t.Price
	}
	b.open.Close = t.Price
	b.open.Volume += int64(t.Size)
}

func (b *Builder) closeOpenBar() {
	if b.open != nil {
		b.bars = append(b.bars, *b.open)
		b.open = nil
	}
}

func (b *Builder) Bars() []Bar {
	result := make([]Bar, 0, len(b.bars)+1)
	result = append(result, b.bars...)

	if b.open != nil {
		result = append(result, *b.open)
	}
	return result
}

func (b *Builder) Flush() {
	b.closeOpenBar()
}

func bucketStart(t time.Time, interval time.Duration) time.Time {
	return t.UTC().Truncate(interval)
}
