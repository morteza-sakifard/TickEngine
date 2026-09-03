package trade

import (
	"fmt"
	"time"

	"github.com/morteza-sakifard/market-data-lab/internal/mbp1"
)

type Side string

const (
	Buy  Side = "Buy"
	Sell Side = "Sell"
)

type Trade struct {
	Time  time.Time
	Price float64
	Size  uint32
	Side  Side
}

func FromRecord(rec mbp1.Record) (Trade, error) {
	if rec.Action != mbp1.ActionTrade {
		return Trade{}, fmt.Errorf("trade: record action %q is not a trade", rec.Action)
	}

	var side Side
	switch rec.Side {
	case mbp1.SideBid:
		side = Buy
	case mbp1.SideAsk:
		side = Sell
	default:
		return Trade{}, fmt.Errorf("trade: unknown aggressor side %q", rec.Side)
	}

	return Trade{
		Time:  rec.TsRecv,
		Price: rec.Price,
		Size:  rec.Size,
		Side:  side,
	}, nil
}

type Reader struct {
	tr *mbp1.TradeReader
}

func NewReader(tr *mbp1.TradeReader) *Reader {
	return &Reader{
		tr: tr,
	}
}

func (r *Reader) Read() (Trade, error) {
	rec, err := r.tr.Read()
	if err != nil {
		return Trade{}, err
	}
	return FromRecord(rec)
}

func (r *Reader) Row() uint64 {
	return r.tr.Row()
}
