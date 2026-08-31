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

const (
	tickSize      = 0.25
	tickEps       = 1e-6
	timeLayoutUTC = "2006-01-02T15:04:05.000000000Z"
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

func (s side) label() string {
	switch s {
	case SideBid:
		return "B buy aggressor "
	case SideAsk:
		return "A sell aggressor"
	case SideUnknown:
		return "N unknown       "
	default:
		return "NA"
	}
}

var (
	actionOrder     = []action{ActionAdd, ActionCancel, ActionModify, ActionClear, ActionTrade}
	sideOrder       = []side{SideBid, SideAsk, SideUnknown}
	requiredColumns = []string{
		"ts_recv",
		"ts_event",
		"action",
		"side",
		"price",
		"size",
		"symbol",
	}
)

type statics struct {
	records      uint
	actionCounts map[action]uint
	sideCounts   map[side]uint
	symbol       string

	minTsEvent     time.Time
	maxTsEvent     time.Time
	eventParseFail uint

	preTsRecv       time.Time
	minTsRecv       time.Time
	maxTsRecv       time.Time
	tsRecvMonotonic bool
	recvDecreases   uint
	recvParseFail   uint

	minPrice  float64
	maxPrice  float64
	offTick   uint
	priceFail uint

	// Action == ActionTrade
	trades          uint
	tradeSideCounts map[side]uint

	tradeVolumes map[side]uint
	minTradeSize uint
	maxTradeSize uint
	sizeFail     uint

	minTradePrice float64
	maxTradePrice float64
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

	fmt.Println("Columns:")
	col := make(map[string]int, len(header))
	for idx, name := range header {
		col[name] = idx
		fmt.Printf("  %2d  %s\n", idx, name)
	}

	for _, name := range requiredColumns {
		if _, ok := col[name]; !ok {
			log.Fatalf("missing required column %q", name)
		}
	}

	s := statics{
		actionCounts:    make(map[action]uint),
		sideCounts:      make(map[side]uint),
		tradeSideCounts: make(map[side]uint),
		tradeVolumes:    make(map[side]uint),
		tsRecvMonotonic: true,
	}

	for {
		row, err := reader.Read()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			log.Fatalf("read row after %d records: %v", s.records, err)
		}

		s.records++

		s.symbol = row[col["symbol"]]
		act := action(row[col["action"]])
		s.actionCounts[act]++

		sd := side(row[col["side"]])
		s.sideCounts[sd]++

		if tRecv, err := time.Parse(time.RFC3339Nano, row[col["ts_recv"]]); err != nil {
			s.recvParseFail++
		} else {
			tRecv = tRecv.UTC()
			switch {
			case s.minTsRecv.IsZero():
				s.minTsRecv, s.maxTsRecv = tRecv, tRecv
			case tRecv.Before(s.minTsRecv):
				s.minTsRecv = tRecv
			case tRecv.After(s.maxTsRecv):
				s.maxTsRecv = tRecv
			}
			if tRecv.Before(s.preTsRecv) {
				s.recvDecreases++
				s.tsRecvMonotonic = false
			}
			s.preTsRecv = tRecv
		}

		if tEvent, err := time.Parse(time.RFC3339Nano, row[col["ts_event"]]); err != nil {
			s.eventParseFail++
		} else {
			switch {
			case s.minTsEvent.IsZero():
				s.minTsEvent, s.maxTsEvent = tEvent, tEvent
			case tEvent.Before(s.minTsEvent):
				s.minTsEvent = tEvent
			case tEvent.After(s.maxTsEvent):
				s.maxTsEvent = tEvent
			}
		}

		price, priceOk := parsePrice(row[col["price"]])
		if !priceOk {
			s.priceFail++
		} else {
			switch {
			case s.minPrice == 0:
				s.minPrice, s.maxPrice = price, price
			case price < s.minPrice:
				s.minPrice = price
			case price > s.maxPrice:
				s.maxPrice = price
			}
			if !onTick(price) {
				s.offTick++
			}
		}

		if act != ActionTrade {
			continue
		}

		s.trades++
		s.tradeSideCounts[sd]++

		if size, ok := parseSize(row[col["size"]]); !ok {
			s.sizeFail++
		} else {
			s.tradeVolumes[sd] += size
			switch {
			case s.minTradeSize == 0:
				s.minTradeSize, s.maxTradeSize = size, size
			case size < s.minTradeSize:
				s.minTradeSize = size
			case size > s.maxTradeSize:
				s.maxTradeSize = size
			}
		}

		if priceOk {
			switch {
			case s.minTradePrice == 0:
				s.minTradePrice, s.maxTradePrice = price, price
			case price < s.minTradePrice:
				s.minTradePrice = price
			case price > s.maxTradePrice:
				s.maxTradePrice = price
			}
		}
	}

	s.report()

	fmt.Println("Streaming complete!")
}

func parsePrice(price string) (float64, bool) {
	p, err := strconv.ParseFloat(price, 64)
	if err != nil {
		return 0, false
	}
	return p, true
}

func onTick(price float64) bool {
	ticks := price / tickSize
	return math.Abs(ticks-math.Round(ticks)) <= tickEps
}

func parseSize(size string) (uint, bool) {
	s, err := strconv.ParseUint(size, 10, 64)
	if err != nil {
		return 0, false
	}
	return uint(s), true
}

func (s statics) report() {
	fmt.Println()
	fmt.Println("==================================================")
	fmt.Println("MBP-1 dataset census")
	fmt.Println("==================================================")
	fmt.Printf("%-28s %d\n", "Total records", s.records)
	fmt.Println("\nSymbols:")
	fmt.Printf("  %-26s\n", s.symbol)
	fmt.Println("\nTimestamps (UTC):")

	fmt.Printf("  %-26s %s\n", "ts_recv min", s.minTsRecv.UTC().Format(timeLayoutUTC))
	fmt.Printf("  %-26s %s\n", "ts_recv max", s.maxTsRecv.UTC().Format(timeLayoutUTC))
	fmt.Printf("  %-26s %s\n", "ts_recv span", s.maxTsRecv.Sub(s.minTsRecv))
	fmt.Printf("  %-26s %v\n", "ts_recv monotonic", s.tsRecvMonotonic)
	fmt.Printf("  %-26s %d\n", "ts_recv decreases", s.recvDecreases)

	fmt.Printf("  %-26s %s\n", "ts_event min", s.minTsEvent.UTC().Format(timeLayoutUTC))
	fmt.Printf("  %-26s %s\n", "ts_event max", s.maxTsEvent.UTC().Format(timeLayoutUTC))

	fmt.Printf("  %-26s %d\n", "ts_recv parse failures", s.recvParseFail)
	fmt.Printf("  %-26s %d\n", "ts_event parse failures", s.eventParseFail)

	fmt.Println("\nAction counts:")
	printCounts(s.actionCounts, actionOrder)

	fmt.Println("\nSide counts (all events):")
	printCounts(s.sideCounts, sideOrder)

	fmt.Println("\nPrice (all events, parsed):")
	fmt.Printf("  %-26s %.9f\n", "min", s.minPrice)
	fmt.Printf("  %-26s %.9f\n", "max", s.maxPrice)
	fmt.Printf("  %-26s %d\n", "off-tick (0.25)", s.offTick)
	fmt.Printf("  %-26s %d\n", "parse failures", s.priceFail)

	fmt.Println("\nTrades (action = T):")
	fmt.Printf("  %-26s %d\n", "count", s.trades)

	fmt.Println("  by aggressor side:")
	var tradeCountSum uint
	var tradeVolSum uint
	for _, sd := range sideOrder {
		n := s.tradeSideCounts[sd]
		v := s.tradeVolumes[sd]
		tradeCountSum += n
		tradeVolSum += v
		fmt.Printf("    %s  trades=%d  volume=%d\n", sd.label(), n, v)
	}

	fmt.Printf("  %-26s %d\n", "size parse failures", s.sizeFail)
	fmt.Printf("  %-26s %d\n", "min size", s.minTradeSize)
	fmt.Printf("  %-26s %d\n", "max size", s.maxTradeSize)

	fmt.Printf("  %-26s %.9f\n", "min price", s.minTradePrice)
	fmt.Printf("  %-26s %.9f\n", "max price", s.maxTradePrice)
	fmt.Println("==================================================")
}

func printCounts[K comparable](m map[K]uint, order []K) {
	seen := make(map[K]bool, len(order))
	for _, k := range order {
		fmt.Printf("  [%s] %d\n", k, m[k])
		seen[k] = true
	}
	for k, n := range m {
		if !seen[k] {
			fmt.Printf("  [%s] %d\n", k, n)
		}
	}
}
