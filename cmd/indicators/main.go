package main

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/morteza-sakifard/market-data-lab/internal/indicators"
	"github.com/morteza-sakifard/market-data-lab/internal/mbp1"
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

	var vwap indicators.VWAP
	var delta indicators.Delta
	var cvd indicators.CVD

	for {
		t, err := stream.Read()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			log.Fatalf("Failed to read trade: %v", err)
		}
		vwap.Add(t)
		delta.Add(t)
		cvd.Add(t)
	}

	fmt.Printf("Trades: %d\n", stream.Row())
	fmt.Printf("VWAP:   %.4f\n", vwap.Value())
	fmt.Printf("Delta:  %d\n", delta.Value())
	fmt.Printf("CVD final:  %d\n", cvd.Value())
	fmt.Printf("CVD points: %d\n", len(cvd.Series()))
}
