package databento

import (
	"bufio"
	"fmt"
	"io"
	"strconv"

	"github.com/morteza-sakifard/market-data-lab/internal/core"
	"github.com/morteza-sakifard/market-data-lab/internal/feed"
	"github.com/morteza-sakifard/market-data-lab/internal/marketdata"
)

var _ feed.Source = (*MBP10)(nil)

const mbp10Levels = marketdata.MaxDepth
const mbp10Cols = 13 + mbp10Levels*6 + 1

// mbp10Header is Databento's MBP-10 CSV: the MBP-1 envelope plus
// bid_px_00..ask_ct_09, then symbol.
func mbp10Header() string {
	h := "ts_recv,ts_event,rtype,publisher_id,instrument_id,action,side,depth," +
		"price,size,flags,ts_in_delta,sequence"
	for i := 0; i < mbp10Levels; i++ {
		h += fmt.Sprintf(",bid_px_%02d,ask_px_%02d,bid_sz_%02d,ask_sz_%02d,bid_ct_%02d,ask_ct_%02d",
			i, i, i, i, i, i)
	}
	return h + ",symbol"
}

// MBP10 reads an MBP-10 CSV. Each row is the full 10-level book
// after that action — the same contract as MBP-1's top-of-book,
// just deeper. FlagSnapshot is on the Event; the book layer resets.
type MBP10 struct {
	rc         io.Closer
	r          *bufio.Reader
	inst       core.Instrument
	line       int
	pending    bool
	pendingEvt marketdata.Event
}

func NewMBP10(rc io.ReadCloser, inst core.Instrument) (*MBP10, error) {
	br := bufio.NewReader(rc)
	head, err := br.ReadSlice('\n')
	if err != nil && err != io.EOF {
		return nil, fmt.Errorf("databento: read header: %w", err)
	}
	want := mbp10Header()
	if got := string(trimRow(head)); got != want {
		return nil, fmt.Errorf("databento: unexpected MBP-10 header %q, want %q", got, want)
	}
	return &MBP10{rc: rc, r: br, inst: inst, line: 1}, nil
}

func (d *MBP10) Close() error { return d.rc.Close() }

func (d *MBP10) Next(dst *marketdata.Event) error {
	if d.pending {
		*dst = d.pendingEvt
		d.pending = false
		return nil
	}
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

	var offs [mbp10Cols][2]int
	if n := splitFieldsN(line, offs[:]); n != mbp10Cols {
		return fmt.Errorf("databento: line %d: got %d fields, want %d", d.line, n, mbp10Cols)
	}

	instrumentID, err := strconv.ParseUint(fieldN(line, offs[:], colInstrumentID), 10, 32)
	if err != nil {
		return fmt.Errorf("databento: line %d: instrument_id: %w", d.line, err)
	}
	if core.InstrumentID(instrumentID) != d.inst.ID {
		return fmt.Errorf("databento: line %d: instrument_id %d does not match configured instrument %d (%s)",
			d.line, instrumentID, d.inst.ID, d.inst.Symbol)
	}

	tsRecv, err := parseTsNano(fieldN(line, offs[:], colTsRecv))
	if err != nil {
		return fmt.Errorf("databento: line %d: ts_recv: %w", d.line, err)
	}
	tsEvent, err := parseTsNano(fieldN(line, offs[:], colTsEvent))
	if err != nil {
		return fmt.Errorf("databento: line %d: ts_event: %w", d.line, err)
	}
	flags, err := strconv.ParseUint(fieldN(line, offs[:], colFlags), 10, 8)
	if err != nil {
		return fmt.Errorf("databento: line %d: flags: %w", d.line, err)
	}
	tsInDelta, err := strconv.ParseInt(fieldN(line, offs[:], colTsInDelta), 10, 32)
	if err != nil {
		return fmt.Errorf("databento: line %d: ts_in_delta: %w", d.line, err)
	}
	sequence, err := strconv.ParseUint(fieldN(line, offs[:], colSequence), 10, 32)
	if err != nil {
		return fmt.Errorf("databento: line %d: sequence: %w", d.line, err)
	}

	var depth marketdata.Depth
	for i := 0; i < mbp10Levels; i++ {
		base := 13 + i*6
		bid, ask, err := parseMBP10Level(d.inst, line, offs[:], base)
		if err != nil {
			return fmt.Errorf("databento: line %d: level %d: %w", d.line, i, err)
		}
		depth.Bids[i] = bid
		depth.Asks[i] = ask
	}
	quote := marketdata.Quote{
		BidPx: depth.Bids[0].Px, AskPx: depth.Asks[0].Px,
		BidQty: depth.Bids[0].Qty, AskQty: depth.Asks[0].Qty,
		BidCt: depth.Bids[0].Count, AskCt: depth.Asks[0].Count,
	}

	envelope := marketdata.Event{
		Instrument: core.InstrumentID(instrumentID),
		TsEvent:    tsEvent,
		TsRecv:     tsRecv,
		TsInDelta:  int32(tsInDelta),
		Sequence:   uint32(sequence),
		Flags:      marketdata.Flags(flags),
		Quote:      quote,
		Depth:      depth,
	}

	action := fieldN(line, offs[:], colAction)
	side := fieldN(line, offs[:], colSide)
	if action != actionTrade {
		*dst = envelope
		dst.Kind = marketdata.KindQuote
		return nil
	}

	priceNano, err := core.ParsePriceNano(fieldN(line, offs[:], colPrice))
	if err != nil {
		return fmt.Errorf("databento: line %d: price: %w", d.line, err)
	}
	tradePx, err := d.inst.TicksFrom(priceNano)
	if err != nil {
		return fmt.Errorf("databento: line %d: price: %w", d.line, err)
	}
	size, err := strconv.ParseInt(fieldN(line, offs[:], colSize), 10, 64)
	if err != nil {
		return fmt.Errorf("databento: line %d: size: %w", d.line, err)
	}
	var aggr core.Side
	switch side {
	case "B":
		aggr = core.SideBid
	case "A":
		aggr = core.SideAsk
	default:
		aggr = core.SideNone
	}
	*dst = envelope
	dst.Kind = marketdata.KindTrade
	dst.Trade = marketdata.Trade{Px: tradePx, Qty: core.Qty(size), Aggressor: aggr}
	dst.Quote = marketdata.Quote{}
	dst.Depth = marketdata.Depth{}

	d.pendingEvt = envelope
	d.pendingEvt.Kind = marketdata.KindQuote
	d.pending = true
	return nil
}

func parseMBP10Level(inst core.Instrument, line []byte, offs [][2]int, base int) (bid, ask marketdata.Level, err error) {
	bid.Px, err = parseBookPx(inst, fieldN(line, offs, base))
	if err != nil {
		return bid, ask, err
	}
	ask.Px, err = parseBookPx(inst, fieldN(line, offs, base+1))
	if err != nil {
		return bid, ask, err
	}
	bs, err := strconv.ParseInt(fieldN(line, offs, base+2), 10, 64)
	if err != nil {
		return bid, ask, err
	}
	as, err := strconv.ParseInt(fieldN(line, offs, base+3), 10, 64)
	if err != nil {
		return bid, ask, err
	}
	bc, err := strconv.ParseUint(fieldN(line, offs, base+4), 10, 32)
	if err != nil {
		return bid, ask, err
	}
	ac, err := strconv.ParseUint(fieldN(line, offs, base+5), 10, 32)
	if err != nil {
		return bid, ask, err
	}
	bid.Qty, ask.Qty = core.Qty(bs), core.Qty(as)
	bid.Count, ask.Count = uint32(bc), uint32(ac)
	return bid, ask, nil
}
