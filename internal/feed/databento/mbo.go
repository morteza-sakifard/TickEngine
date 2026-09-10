package databento

import (
	"bufio"
	"fmt"
	"io"
	"strconv"

	"github.com/morteza-sakifard/TickEngine/internal/core"
	"github.com/morteza-sakifard/TickEngine/internal/feed"
	"github.com/morteza-sakifard/TickEngine/internal/marketdata"
)

var _ feed.Source = (*MBO)(nil)

const mboCols = 15

const mboHeader = "ts_recv,ts_event,rtype,publisher_id,instrument_id,action,side,price,size," +
	"channel_id,order_id,flags,ts_in_delta,sequence,symbol"

const (
	mboColTsRecv = iota
	mboColTsEvent
	mboColRType
	mboColPublisherID
	mboColInstrumentID
	mboColAction
	mboColSide
	mboColPrice
	mboColSize
	mboColChannelID
	mboColOrderID
	mboColFlags
	mboColTsInDelta
	mboColSequence
	mboColSymbol
)

// MBO reads Databento's MBO CSV. One row is one Event: Add/Cancel/
// Modify/Clear become KindBook, T becomes KindTrade. F and N are
// skipped — Fill does not change the book, and None is a heartbeat.
type MBO struct {
	rc   io.Closer
	r    *bufio.Reader
	inst core.Instrument
	line int
}

func NewMBO(rc io.ReadCloser, inst core.Instrument) (*MBO, error) {
	br := bufio.NewReader(rc)
	head, err := br.ReadSlice('\n')
	if err != nil && err != io.EOF {
		return nil, fmt.Errorf("databento: read header: %w", err)
	}
	if got := string(trimRow(head)); got != mboHeader {
		return nil, fmt.Errorf("databento: unexpected MBO header %q, want %q", got, mboHeader)
	}
	return &MBO{rc: rc, r: br, inst: inst, line: 1}, nil
}

func (d *MBO) Close() error { return d.rc.Close() }

func (d *MBO) Next(dst *marketdata.Event) error {
	for {
		line, err := d.r.ReadSlice('\n')
		if len(line) == 0 {
			if err == io.EOF {
				return io.EOF
			}
			if err != nil {
				return fmt.Errorf("databento: line %d: %w", d.line+1, err)
			}
		}
		d.line++

		var offs [mboCols][2]int
		if n := splitFieldsN(line, offs[:]); n != mboCols {
			return fmt.Errorf("databento: line %d: got %d fields, want %d", d.line, n, mboCols)
		}

		instrumentID, err := strconv.ParseUint(fieldN(line, offs[:], mboColInstrumentID), 10, 32)
		if err != nil {
			return fmt.Errorf("databento: line %d: instrument_id: %w", d.line, err)
		}
		if core.InstrumentID(instrumentID) != d.inst.ID {
			return fmt.Errorf("databento: line %d: instrument_id %d does not match configured instrument %d (%s)",
				d.line, instrumentID, d.inst.ID, d.inst.Symbol)
		}

		action := fieldN(line, offs[:], mboColAction)
		if action == "F" || action == "N" {
			continue
		}

		tsRecv, err := parseTsNano(fieldN(line, offs[:], mboColTsRecv))
		if err != nil {
			return fmt.Errorf("databento: line %d: ts_recv: %w", d.line, err)
		}
		tsEvent, err := parseTsNano(fieldN(line, offs[:], mboColTsEvent))
		if err != nil {
			return fmt.Errorf("databento: line %d: ts_event: %w", d.line, err)
		}
		flags, err := strconv.ParseUint(fieldN(line, offs[:], mboColFlags), 10, 8)
		if err != nil {
			return fmt.Errorf("databento: line %d: flags: %w", d.line, err)
		}
		tsInDelta, err := strconv.ParseInt(fieldN(line, offs[:], mboColTsInDelta), 10, 32)
		if err != nil {
			return fmt.Errorf("databento: line %d: ts_in_delta: %w", d.line, err)
		}
		sequence, err := strconv.ParseUint(fieldN(line, offs[:], mboColSequence), 10, 32)
		if err != nil {
			return fmt.Errorf("databento: line %d: sequence: %w", d.line, err)
		}

		*dst = marketdata.Event{
			Instrument: core.InstrumentID(instrumentID),
			TsEvent:    tsEvent,
			TsRecv:     tsRecv,
			TsInDelta:  int32(tsInDelta),
			Sequence:   uint32(sequence),
			Flags:      marketdata.Flags(flags),
		}

		side := parseMBOSide(fieldN(line, offs[:], mboColSide))
		if action == actionTrade {
			priceNano, err := core.ParsePriceNano(fieldN(line, offs[:], mboColPrice))
			if err != nil {
				return fmt.Errorf("databento: line %d: price: %w", d.line, err)
			}
			tradePx, err := d.inst.TicksFrom(priceNano)
			if err != nil {
				return fmt.Errorf("databento: line %d: price: %w", d.line, err)
			}
			size, err := strconv.ParseInt(fieldN(line, offs[:], mboColSize), 10, 64)
			if err != nil {
				return fmt.Errorf("databento: line %d: size: %w", d.line, err)
			}
			dst.Kind = marketdata.KindTrade
			dst.Trade = marketdata.Trade{Px: tradePx, Qty: core.Qty(size), Aggressor: side}
			return nil
		}

		act, ok := parseBookAction(action)
		if !ok {
			return fmt.Errorf("databento: line %d: unknown action %q", d.line, action)
		}
		orderID, err := strconv.ParseUint(fieldN(line, offs[:], mboColOrderID), 10, 64)
		if err != nil {
			return fmt.Errorf("databento: line %d: order_id: %w", d.line, err)
		}
		size, err := strconv.ParseInt(fieldN(line, offs[:], mboColSize), 10, 64)
		if err != nil {
			return fmt.Errorf("databento: line %d: size: %w", d.line, err)
		}
		var px core.Ticks
		if act != marketdata.BookClear {
			px, err = parseBookPx(d.inst, fieldN(line, offs[:], mboColPrice))
			if err != nil {
				return fmt.Errorf("databento: line %d: price: %w", d.line, err)
			}
		}
		dst.Kind = marketdata.KindBook
		dst.Book = marketdata.Book{
			Action:  act,
			OrderID: orderID,
			Side:    side,
			Px:      px,
			Qty:     core.Qty(size),
		}
		return nil
	}
}

func parseBookAction(a string) (marketdata.BookAction, bool) {
	switch a {
	case "A":
		return marketdata.BookAdd, true
	case "C":
		return marketdata.BookCancel, true
	case "M":
		return marketdata.BookModify, true
	case "R":
		return marketdata.BookClear, true
	default:
		return 0, false
	}
}

func parseMBOSide(s string) core.Side {
	switch s {
	case "B":
		return core.SideBid
	case "A":
		return core.SideAsk
	default:
		return core.SideNone
	}
}
