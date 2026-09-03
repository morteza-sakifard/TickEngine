package ohlcv_test

import (
	"fmt"
	"time"

	"github.com/morteza-sakifard/market-data-lab/internal/ohlcv"
	"github.com/morteza-sakifard/market-data-lab/internal/session"
	"github.com/morteza-sakifard/market-data-lab/internal/trade"
)

// Example traces one trade through every Step 11 stage: ts_recv -> local
// time -> trading date -> session -> bucket start -> OHLCV. Run with
// `go test ./internal/ohlcv/ -run Example -v` to see it end to end.
func Example() {
	loc, err := time.LoadLocation("America/Chicago")
	if err != nil {
		panic(err)
	}
	cal := session.Calendar{Location: loc, Schedule: session.ESRegularSchedule()}

	tsRecv := time.Date(2025, time.September, 23, 15, 31, 17, 0, time.UTC)
	tr := trade.Trade{Time: tsRecv, Price: 6714.75, Size: 2, Side: trade.Buy}

	local := tr.Time.In(loc)
	tradingDate, sess := cal.Classify(tr.Time)

	b := ohlcv.Builder{Symbol: "ESZ5", Interval: 5 * time.Minute, Calendar: cal}
	b.Add(tr)
	bar := b.Bars()[0]

	fmt.Println("ts_recv (UTC):", tr.Time.Format(time.RFC3339))
	fmt.Println("local (CT):", local.Format(time.RFC3339))
	fmt.Println("trading date:", tradingDate.Format("2006-01-02"))
	fmt.Println("session:", sess)
	fmt.Println("bucket start:", bar.BucketStart.Format(time.RFC3339))
	fmt.Printf("OHLCV: O=%.2f H=%.2f L=%.2f C=%.2f V=%d\n", bar.Open, bar.High, bar.Low, bar.Close, bar.Volume)

	// Output:
	// ts_recv (UTC): 2025-09-23T15:31:17Z
	// local (CT): 2025-09-23T10:31:17-05:00
	// trading date: 2025-09-23
	// session: RTH
	// bucket start: 2025-09-23T15:30:00Z
	// OHLCV: O=6714.75 H=6714.75 L=6714.75 C=6714.75 V=2
}
