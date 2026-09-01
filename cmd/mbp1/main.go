package main

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/morteza-sakifard/market-data-lab/internal/mbp1"
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

	tradeReader := mbp1.NewTradeReader(reader)

	for {
		_, err := tradeReader.Read()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			log.Fatalf("Failed to read row: %v", err)
		}
	}

	fmt.Printf("Total rows: %d\n", reader.Row())
	fmt.Printf("Trade rows: %d\n", tradeReader.Row())
}
