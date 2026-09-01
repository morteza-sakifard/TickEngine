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
	var vp indicators.VolumeProfile
	var fp indicators.Footprint
	var tpo indicators.TPO

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
		vp.Add(t)
		fp.Add(t)
		tpo.Add(t)
	}

	var totalVolume int64
	for _, l := range vp.Levels() {
		totalVolume += l.Volume
	}

	var fpBuy, fpSell int64
	for _, l := range fp.Levels() {
		fpBuy += l.BuyVolume
		fpSell += l.SellVolume
	}

	fmt.Printf("Trades: %d\n", stream.Row())
	fmt.Printf("VWAP:   %.4f\n", vwap.Value())
	fmt.Printf("Delta:  %d\n", delta.Value())
	fmt.Printf("CVD final:  %d\n", cvd.Value())
	fmt.Printf("CVD points: %d\n", len(cvd.Series()))

	fmt.Printf("VP levels:  %d\n", len(vp.Levels()))
	fmt.Printf("VP total:   %d\n", totalVolume)
	fmt.Printf("VP POC:     %.2f (vol %d)\n", vp.POC().Price, vp.POC().Volume)

	fmt.Printf("FP levels:  %d\n", len(fp.Levels()))
	fmt.Printf("FP buy:     %d\n", fpBuy)
	fmt.Printf("FP sell:    %d\n", fpSell)
	fmt.Printf("FP delta:   %d\n", fpBuy-fpSell)

	fmt.Printf("TPO levels: %d\n", len(tpo.Levels()))
	fmt.Printf("TPO POC:    %.2f (periods %d)\n", tpo.POC().Price, tpo.POC().Count())
}
