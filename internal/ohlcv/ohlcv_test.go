package ohlcv

import (
	"testing"
	"time"

	"github.com/morteza-sakifard/market-data-lab/internal/session"
	"github.com/morteza-sakifard/market-data-lab/internal/trade"
)

func utcTime(y int, m time.Month, d, hh, mm, ss int) time.Time {
	return time.Date(y, m, d, hh, mm, ss, 0, time.UTC)
}

func testCalendar(t *testing.T) session.Calendar {
	t.Helper()
	loc, err := time.LoadLocation("America/Chicago")
	if err != nil {
		t.Fatalf("LoadLocation: %v", err)
	}
	return session.Calendar{Location: loc, Schedule: session.ESRegularSchedule()}
}

// Bucket boundaries are floor(t, interval) in UTC, independent of session.

func TestBucketStart_FloorsToInterval(t *testing.T) {
	got := bucketStart(utcTime(2025, time.September, 23, 10, 31, 17), 5*time.Minute)
	want := utcTime(2025, time.September, 23, 10, 30, 0)
	if !got.Equal(want) {
		t.Fatalf("bucketStart = %v, want %v", got, want)
	}
}

func TestBucketStart_ExactBoundaryIsUnchanged(t *testing.T) {
	got := bucketStart(utcTime(2025, time.September, 23, 10, 35, 0), 5*time.Minute)
	want := utcTime(2025, time.September, 23, 10, 35, 0)
	if !got.Equal(want) {
		t.Fatalf("bucketStart = %v, want %v", got, want)
	}
}

// End-of-stream usage: Flush() commits the last open bucket, then Bars()
// reads the final result.

func TestBuilder_AggregatesOHLCVWithinOneBucket(t *testing.T) {
	b := Builder{Symbol: "ESZ5", Interval: 5 * time.Minute, Calendar: testCalendar(t)}

	b.Add(trade.Trade{Time: utcTime(2025, time.September, 23, 14, 31, 0), Price: 6700.00, Size: 2})
	b.Add(trade.Trade{Time: utcTime(2025, time.September, 23, 14, 32, 0), Price: 6701.25, Size: 1})
	b.Add(trade.Trade{Time: utcTime(2025, time.September, 23, 14, 33, 0), Price: 6699.50, Size: 3})
	b.Add(trade.Trade{Time: utcTime(2025, time.September, 23, 14, 34, 0), Price: 6700.75, Size: 4})
	b.Flush()

	bars := b.Bars()
	if len(bars) != 1 {
		t.Fatalf("len(bars) = %d, want 1", len(bars))
	}
	got := bars[0]
	if got.Open != 6700.00 {
		t.Errorf("Open = %v, want 6700.00", got.Open)
	}
	if got.High != 6701.25 {
		t.Errorf("High = %v, want 6701.25", got.High)
	}
	if got.Low != 6699.50 {
		t.Errorf("Low = %v, want 6699.50", got.Low)
	}
	if got.Close != 6700.75 {
		t.Errorf("Close = %v, want 6700.75", got.Close)
	}
	if got.Volume != 10 {
		t.Errorf("Volume = %d, want 10", got.Volume)
	}
}

func TestBuilder_EmptyBucketProducesNoBar(t *testing.T) {
	b := Builder{Symbol: "ESZ5", Interval: 5 * time.Minute, Calendar: testCalendar(t)}

	b.Add(trade.Trade{Time: utcTime(2025, time.September, 23, 14, 31, 0), Price: 100, Size: 1}) // bucket 14:30
	b.Add(trade.Trade{Time: utcTime(2025, time.September, 23, 14, 41, 0), Price: 101, Size: 1}) // bucket 14:40
	b.Flush()

	bars := b.Bars()
	if len(bars) != 2 {
		t.Fatalf("len(bars) = %d, want 2 (no artificial bar for the empty 14:35 bucket)", len(bars))
	}
	if got, want := bars[0].BucketStart, utcTime(2025, time.September, 23, 14, 30, 0); !got.Equal(want) {
		t.Errorf("bars[0].BucketStart = %v, want %v", got, want)
	}
	if got, want := bars[1].BucketStart, utcTime(2025, time.September, 23, 14, 40, 0); !got.Equal(want) {
		t.Errorf("bars[1].BucketStart = %v, want %v", got, want)
	}
}

func TestBuilder_MaintenanceGapDoesNotCorruptNextBar(t *testing.T) {
	b := Builder{Symbol: "ESZ5", Interval: 5 * time.Minute, Calendar: testCalendar(t)}

	b.Add(trade.Trade{Time: utcTime(2025, time.September, 23, 20, 14, 30), Price: 6700, Size: 1}) // 15:14:30 CT, last RTH bucket
	b.Add(trade.Trade{Time: utcTime(2025, time.September, 23, 22, 6, 0), Price: 6750, Size: 5})   // 17:06:00 CT, after the maintenance break
	b.Flush()

	bars := b.Bars()
	if len(bars) != 2 {
		t.Fatalf("len(bars) = %d, want 2", len(bars))
	}
	if bars[0].Close != 6700 || bars[0].Volume != 1 {
		t.Errorf("bars[0] = %+v, want Close=6700 Volume=1", bars[0])
	}
	if bars[1].Open != 6750 || bars[1].Volume != 5 {
		t.Errorf("bars[1] = %+v, want Open=6750 Volume=5", bars[1])
	}
}

// Regression test: Bars() must be a read-only snapshot. Calling it mid-bucket
// must not close the open bucket, or a later trade in the same bucket would
// start a second Bar for a bucket a read had already "closed".
func TestBuilder_BarsDoesNotCloseOpenBucket(t *testing.T) {
	b := Builder{Symbol: "ESZ5", Interval: 5 * time.Minute, Calendar: testCalendar(t)}

	b.Add(trade.Trade{Time: utcTime(2025, time.September, 23, 14, 31, 0), Price: 100, Size: 1})
	if got := len(b.Bars()); got != 1 {
		t.Fatalf("Bars() after 1st trade: len = %d, want 1 (open bucket included as a snapshot)", got)
	}

	b.Add(trade.Trade{Time: utcTime(2025, time.September, 23, 14, 32, 0), Price: 101, Size: 1})
	if got := len(b.Bars()); got != 1 {
		t.Fatalf("Bars() after 2nd trade: len = %d, want 1 (still the same open bucket, not split)", got)
	}

	b.Add(trade.Trade{Time: utcTime(2025, time.September, 23, 14, 34, 0), Price: 99, Size: 1})
	b.Flush()
	bars := b.Bars()

	if len(bars) != 1 {
		t.Fatalf("len(bars) = %d, want 1 (mid-bucket Bars() calls must not split one bucket into two Bars)", len(bars))
	}
	got := bars[0]
	if got.Open != 100 || got.High != 101 || got.Low != 99 || got.Close != 99 || got.Volume != 3 {
		t.Errorf("got %+v, want Open=100 High=101 Low=99 Close=99 Volume=3", got)
	}
}
