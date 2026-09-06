package main

import "math"

// Hist is a fixed-width bucket histogram for computing approximate
// percentiles over a stream that does not fit in memory.
//
// Values are bucketed by v/width. Values at or above width*len(buckets)
// land in an overflow counter and negative values in a separate counter;
// neither contributes a bucket. Min, Max and Count stay exact regardless
// of bucketing.
type Hist struct {
	width    int64
	buckets  []int64
	overflow int64
	under    int64
	count    int64
	min, max int64
	seen     bool
}

// NewHist returns a histogram with n buckets, each width units wide.
// It panics on non-positive arguments: those are programmer errors, not
// runtime conditions.
func NewHist(width, n int64) *Hist {
	if width <= 0 {
		panic("hist: width must be positive")
	}
	if n <= 0 {
		panic("hist: bucket count must be positive")
	}
	return &Hist{width: width, buckets: make([]int64, n)}
}

func (h *Hist) Add(v int64) {
	h.count++
	if !h.seen {
		h.min, h.max, h.seen = v, v, true
	} else {
		if v < h.min {
			h.min = v
		}
		if v > h.max {
			h.max = v
		}
	}

	if v < 0 {
		h.under++
		return
	}
	if i := v / h.width; i < int64(len(h.buckets)) {
		h.buckets[i]++
		return
	}
	h.overflow++
}

// Percentile returns the value at quantile p, rounded down to the bucket
// lower bound; accuracy is therefore the bucket width. It returns Min()
// when the quantile falls among negative values and Max() when it falls
// in the overflow bucket, because their exact values are not retained.
func (h *Hist) Percentile(p float64) int64 {
	if h.count == 0 {
		return 0
	}
	if p < 0 {
		p = 0
	}
	if p > 1 {
		p = 1
	}

	target := max(int64(math.Ceil(p*float64(h.count))), 1)

	cum := h.under
	if cum >= target {
		return h.min
	}
	for i, n := range h.buckets {
		cum += n
		if cum >= target {
			return int64(i) * h.width
		}
	}
	return h.max
}

func (h *Hist) Count() int64    { return h.count }
func (h *Hist) Overflow() int64 { return h.overflow }
func (h *Hist) Under() int64    { return h.under }

func (h *Hist) Min() int64 {
	if !h.seen {
		return 0
	}
	return h.min
}

func (h *Hist) Max() int64 {
	if !h.seen {
		return 0
	}
	return h.max
}
