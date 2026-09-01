package main

import (
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
	for {
		t, err := stream.Read()
		if err != nil {
			if err == io.EOF {
				break
			}
			log.Fatalf("Failed to read trade: %v", err)
		}
		vwap.Add(t)
	}

	fmt.Printf("Trades: %d\n", stream.Row())
	fmt.Printf("VWAP:   %.4f\n", vwap.Value())
}
