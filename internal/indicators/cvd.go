package indicators

import (
	"time"

	"github.com/morteza-sakifard/market-data-lab/internal/trade"
)

type CVDPoint struct {
	Time  time.Time
	Value int64
}

type CVD struct {
	points []CVDPoint
	delta  Delta
}

func (c *CVD) Add(t trade.Trade) {
	c.delta.Add(t)
	c.points = append(c.points, CVDPoint{Time: t.Time, Value: c.delta.Value()})
}

func (c *CVD) Value() int64 {
	return c.delta.Value()
}

func (c *CVD) Series() []CVDPoint {
	return c.points
}
