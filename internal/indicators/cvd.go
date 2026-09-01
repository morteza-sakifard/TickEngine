package indicators

import (
	"time"

	"github.com/morteza-sakifard/market-data-lab/internal/trade"
)

// CVDPoint is one sample of the cumulative volume delta series.
type CVDPoint struct {
	Time  time.Time
	Value int64
}

// CVD tracks cumulative volume delta over time:
// CVD[t] = CVD[t-1] + Delta[t]
type CVD struct {
	points []CVDPoint
	delta  Delta
}

// Add incorporates one trade and records the new cumulative value.
func (c *CVD) Add(t trade.Trade) {
	c.delta.Add(t)
	c.points = append(c.points, CVDPoint{Time: t.Time, Value: c.delta.Value()})
}

// Value returns the most recent CVD value.
func (c *CVD) Value() int64 {
	return c.delta.Value()
}

// Series returns the full CVD time series recorded so far.
func (c *CVD) Series() []CVDPoint {
	return c.points
}
