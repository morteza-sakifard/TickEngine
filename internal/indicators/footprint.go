package indicators

import (
	"sort"

	"github.com/morteza-sakifard/market-data-lab/internal/trade"
)

// FootprintLevel is one row of a footprint: buy and sell volume traded at a price.
type FootprintLevel struct {
	Price      float64
	BuyVolume  int64
	SellVolume int64
}

// Delta returns the order-flow imbalance at this level: BuyVolume - SellVolume.
func (l FootprintLevel) Delta() int64 {
	return l.BuyVolume - l.SellVolume
}

// Footprint aggregates buy and sell volume separately by price level.
type Footprint struct {
	levels map[float64]FootprintLevel
}

// Add incorporates one trade into the footprint.
func (fp *Footprint) Add(t trade.Trade) {
	if fp.levels == nil {
		fp.levels = make(map[float64]FootprintLevel)
	}
	lvl := fp.levels[t.Price]
	lvl.Price = t.Price
	switch t.Side {
	case trade.Buy:
		lvl.BuyVolume += int64(t.Size)
	case trade.Sell:
		lvl.SellVolume += int64(t.Size)
	}
	fp.levels[t.Price] = lvl
}

// Levels returns the footprint as a slice sorted by ascending price.
func (fp *Footprint) Levels() []FootprintLevel {
	levels := make([]FootprintLevel, 0, len(fp.levels))
	for _, l := range fp.levels {
		levels = append(levels, l)
	}
	sort.Slice(levels, func(i, j int) bool { return levels[i].Price < levels[j].Price })
	return levels
}
