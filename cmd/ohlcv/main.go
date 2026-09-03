package main

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	"github.com/morteza-sakifard/market-data-lab/internal/indicators"
	"github.com/morteza-sakifard/market-data-lab/internal/mbp1"
	"github.com/morteza-sakifard/market-data-lab/internal/ohlcv"
	"github.com/morteza-sakifard/market-data-lab/internal/session"
	"github.com/morteza-sakifard/market-data-lab/internal/trade"
)

func main() {
	file, err := os.Open("databento_glbx.mdp3_mbp_1.csv")
	if err != nil {
		log.Fatalf("Failed to open file: %v", err)
	}
	defer file.Close()

	reader, err := mbp1.NewReader(file)
	if err != nil {
		log.Fatalf("Failed to read file: %v", err)
	}
	stream := trade.NewReader(mbp1.NewTradeReader(reader))

	var pipeline indicators.Pipeline

	loc, err := time.LoadLocation("America/Chicago")
	if err != nil {
		panic(err)
	}
	cal := session.Calendar{Location: loc, Schedule: session.ESRegularSchedule()}
	b := ohlcv.Builder{Symbol: "ESZ5", Interval: 5 * time.Minute, Calendar: cal}

	for {
		t, err := stream.Read()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			log.Fatalf("Failed to read trade: %v", err)
		}
		b.Add(t)
		pipeline.Add(t)
	}

	b.Flush()
	for _, bar := range b.Bars()[:5] {
		fmt.Println("bucket start:", bar.BucketStart.Format(time.RFC3339))
		fmt.Println("trading date:", bar.TradingDate.Format("2006-01-02"))
		fmt.Println("session:", bar.Session)
		fmt.Printf("OHLCV: O=%.2f H=%.2f L=%.2f C=%.2f V=%d\n", bar.Open, bar.High, bar.Low, bar.Close, bar.Volume)
	}
}
