package main

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"

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

	var buyVol, sellVol uint32
	for {
		t, err := stream.Read()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			log.Fatalf("Failed to read trade: %v", err)
		}

		switch t.Side {
		case trade.Buy:
			buyVol += t.Size
		case trade.Sell:
			sellVol += t.Size
		}
	}

	fmt.Printf("Total rows: %d\n", reader.Row())
	fmt.Printf("Trade rows: %d\n", stream.Row())
	fmt.Printf("Buy volume:  %d\n", buyVol)
	fmt.Printf("Sell volume: %d\n", sellVol)
}
