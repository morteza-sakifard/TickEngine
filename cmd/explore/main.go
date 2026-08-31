package main

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"strconv"
	"time"
)

type action string

const (
	ActionAdd    action = "A"
	ActionCancel action = "C"
	ActionModify action = "M"
	ActionClear  action = "R"
	ActionTrade  action = "T"
)

type side string

const (
	SideAsk     side = "A"
	SideBid     side = "B"
	SideUnknown side = "N"
)

type statics struct {
	records      uint
	actionCounts map[action]uint
	sideCounts   map[side]uint
	minTsEvent   time.Time
	maxTsEvent   time.Time
	minTsRecv    time.Time
	maxTsRecv    time.Time
	minPrice     float64
	maxPrice     float64
	symbol       string
	// Action == ActionTrade
	trades          uint
	tradeSideCounts map[side]uint
	tradeVolumes    map[side]uint64
	minTradeSize    uint64
	maxTradeSize    uint64
	minTradePrice   float64
	maxTradePrice   float64
}

func main() {
	file, err := os.Open("databento_glbx.mdp3_mbp_1.csv")
	if err != nil {
		log.Fatalf("Failed to open file: %v", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.ReuseRecord = true

	header, err := reader.Read()
	if err != nil {
		log.Fatalf("Failed to read header: %v", err)
	}

	headerMap := make(map[string]int)
	for idx, name := range header {
		headerMap[name] = idx
	}

	actionIdx, hasAction := headerMap["action"]
	sideIdx, hasSide := headerMap["side"]
	tsRecvIdx, hasTsRecv := headerMap["ts_recv"]
	tsEventIdx, hasTsEvent := headerMap["ts_event"]
	priceIdx, hasPrice := headerMap["price"]
	sizeIdx, hasSize := headerMap["size"]
	symbolIdx, hasSymbol := headerMap["symbol"]

	stats := statics{
		actionCounts:    make(map[action]uint),
		sideCounts:      make(map[side]uint),
		tradeSideCounts: make(map[side]uint),
		tradeVolumes:    make(map[side]uint64),
	}

	for {
		row, err := reader.Read()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			log.Printf("Failed to read: %v", err)
			continue
		}

		stats.records++

		if hasSymbol && symbolIdx < len(row) {
			stats.symbol = row[symbolIdx]
		}

		var act action
		if hasAction && actionIdx < len(row) {
			act = action(row[actionIdx])
			stats.actionCounts[act]++
		}

		var currentSide *side
		if hasSide && sideIdx < len(row) {
			s := side(row[sideIdx])
			currentSide = &s
			stats.sideCounts[*currentSide]++
		}

		if hasTsRecv && tsRecvIdx < len(row) {
			if tRecv, err := time.Parse(time.RFC3339Nano, row[tsRecvIdx]); err == nil {
				if stats.minTsRecv.IsZero() || tRecv.Before(stats.minTsRecv) {
					stats.minTsRecv = tRecv
				}
				if tRecv.After(stats.maxTsRecv) {
					stats.maxTsRecv = tRecv
				}
			}
		}

		if hasTsEvent && tsEventIdx < len(row) {
			if tEvent, err := time.Parse(time.RFC3339Nano, row[tsEventIdx]); err == nil {
				if stats.minTsEvent.IsZero() || tEvent.Before(stats.minTsEvent) {
					stats.minTsEvent = tEvent
				}
				if tEvent.After(stats.maxTsEvent) {
					stats.maxTsEvent = tEvent
				}
			}
		}

		tickSize := 0.25
		eps := 1e-9
		var price *float64
		if hasPrice && priceIdx < len(row) {
			if p, err := strconv.ParseFloat(row[priceIdx], 64); err == nil {
				reminder := math.Mod(p, tickSize)
				if reminder < eps || reminder > (tickSize-eps) {
					price = &p

					if stats.minPrice == 0 || p < stats.minPrice {
						stats.minPrice = p
					}
					if p > stats.maxPrice {
						stats.maxPrice = p
					}
				}
			}
		}

		if act == ActionTrade {
			stats.trades++

			var size *uint64
			if hasSize && sizeIdx < len(row) {
				if s, err := strconv.ParseUint(row[sizeIdx], 10, 64); err == nil {
					size = &s
					if stats.minTradeSize == 0 || s < stats.minTradeSize {
						stats.minTradeSize = s
					}
					if s > stats.maxTradeSize {
						stats.maxTradeSize = s
					}
				}
			}

			if currentSide != nil {
				stats.tradeSideCounts[*currentSide]++

				if size != nil {
					stats.tradeVolumes[*currentSide] += *size
				}
			}

			if price != nil {
				if stats.minTradePrice == 0 || *price < stats.minTradePrice {
					stats.minTradePrice = *price
				}
				if *price > stats.maxTradePrice {
					stats.maxTradePrice = *price
				}
			}
		}
	}

	stats.prettyPrint()

	fmt.Println("Streaming complete!")
}

func (s *statics) prettyPrint() {
	timeLayout := "Jan 2, 2006 15:04:05.000000000"

	fmt.Println("\n==================================================")
	fmt.Printf("MARKET DATA ANALYSIS SUMMARY: %s\n", s.symbol)
	fmt.Println("==================================================")

	// Global Metrics
	fmt.Printf("%-24s: %d\n", "Total Records Processed", s.records)

	// Time Metrics
	fmt.Println("\nTimestamps:")
	if !s.minTsRecv.IsZero() {
		fmt.Printf("  %-22s: %s\n", "Min Receive Time", s.minTsRecv.Format(timeLayout))
		fmt.Printf("  %-22s: %s\n", "Max Receive Time", s.maxTsRecv.Format(timeLayout))
		fmt.Printf("  %-22s: %v\n", "Receive Duration", s.maxTsRecv.Sub(s.minTsRecv))
	}
	if !s.minTsEvent.IsZero() {
		fmt.Printf("  %-22s: %s\n", "Min Event Time", s.minTsEvent.Format(timeLayout))
		fmt.Printf("  %-22s: %s\n", "Max Event Time", s.maxTsEvent.Format(timeLayout))
	}

	// Action Breakdown
	fmt.Println("\nAction Counts:")
	for act, count := range s.actionCounts {
		fmt.Printf("  Action [%s] %-13s: %d\n", act, "", count)
	}

	// Book Side Breakdown
	fmt.Println("\nBook Side Counts:")
	for sd, count := range s.sideCounts {
		fmt.Printf("  Side [%s] %-15s: %d\n", sd, "", count)
	}

	// Global Price Boundaries
	fmt.Println("\nPrice Boundaries (All Actions):")
	fmt.Printf("  %-22s: %.2f\n", "Min Price", s.minPrice)
	fmt.Printf("  %-22s: %.2f\n", "Max Price", s.maxPrice)

	// Trade Specific Analytics
	fmt.Println("\nTrade Execution Analytics (Action = T):")
	fmt.Printf("  %-22s: %d\n", "Total Trades", s.trades)

	fmt.Println("  Trade Volumes & Frequencies:")
	for sd, count := range s.tradeSideCounts {
		vol := s.tradeVolumes[sd]
		fmt.Printf("    Side [%s] -> %-11s: %d executions (Vol: %d)\n", sd, "", count, vol)
	}

	fmt.Println("  Trade Bounds:")
	fmt.Printf("    %-20s: %d\n", "Min Size", s.minTradeSize)
	fmt.Printf("    %-20s: %d\n", "Max Size", s.maxTradeSize)
	fmt.Printf("    %-20s: %.2f\n", "Min Price", s.minTradePrice)
	fmt.Printf("    %-20s: %.2f\n", "Max Price", s.maxTradePrice)
	fmt.Printf("  %-20s: %.2f$\n", "Price Volatility", s.minTradePrice-s.maxTradePrice)
	fmt.Println("==================================================")
}
