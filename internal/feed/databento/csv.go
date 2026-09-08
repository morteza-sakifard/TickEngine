// Package databento decodes Databento's MBP-1 CSV export into
// normalized marketdata.Event values. See docs/01-roadmap.md step 4
// and docs/00-architecture.md L2: everything above this package sees
// only marketdata.Event through feed.Source, never a CSV row.
package databento

import (
	"bufio"
	"fmt"
	"io"
	"math"
	"strconv"
	"time"

	"github.com/morteza-sakifard/market-data-lab/internal/core"
	"github.com/morteza-sakifard/market-data-lab/internal/feed"
	"github.com/morteza-sakifard/market-data-lab/internal/marketdata"
)

var _ feed.Source = (*Decoder)(nil)

// numColumns and the column indices below are Databento's MBP-1 CSV
// layout exactly as step 1's census found it on the real file.
const numColumns = 20

const (
	colTsRecv = iota
	colTsEvent
	colRType
	colPublisherID
	colInstrumentID
	colAction
	colSide
	colDepth
	colPrice
	colSize
	colFlags
	colTsInDelta
	colSequence
	colBidPx
	colAskPx
	colBidSz
	colAskSz
	colBidCt
	colAskCt
	colSymbol
)

const expectedHeader = "ts_recv,ts_event,rtype,publisher_id,instrument_id,action,side,depth," +
	"price,size,flags,ts_in_delta,sequence,bid_px_00,ask_px_00,bid_sz_00,ask_sz_00,bid_ct_00,ask_ct_00,symbol"

// actionTrade is Databento's action code for an execution. Every other
// action (add, cancel, modify, ...) still carries the post-event
// top-of-book in bid_px_00..ask_ct_00, so it still produces a Quote —
// just no Trade.
const actionTrade = "T"

// Decoder reads Databento MBP-1 CSV rows and produces normalized
// marketdata.Event values, converting prices with inst's tick size.
// It implements feed.Source.
//
// Every row is both an event and a snapshot of the top-of-book after
// that event (docs/01-roadmap.md step 4). A row with action "T" is
// therefore split into two Events — Trade first, then the Quote that
// resulted from it, since the book is the state *after* the trade —
// and every other row produces a Quote only. Both events split from
// one row share the same envelope (TsEvent, TsRecv, TsInDelta,
// Sequence, Flags, Instrument): there is only one underlying wire
// message; Next simply cannot hand back two events from one call.
//
// Next performs zero allocations on the success path (TestNextAllocs
// proves it). See the "leaking parameter" decision in
// docs/steps/04-feed-databento.reference.md before changing any string
// handling in this file — it is easy to silently break.
type Decoder struct {
	rc   io.Closer
	r    *bufio.Reader
	inst core.Instrument
	line int // 1-based; the header is line 1

	pending    bool
	pendingEvt marketdata.Event
}

// NewDecoder validates rc's header against Databento's known MBP-1
// column layout and returns a Decoder that converts prices using
// inst's tick size and closes rc when Close is called. On error, rc is
// still open; the caller remains responsible for closing it.
//
// Every row's instrument_id is cross-checked against inst.ID in Next:
// converting a foreign instrument's price with the wrong tick size
// would silently produce a wrong-but-plausible-looking Ticks value
// instead of an error, which is exactly the kind of mistake this
// project does not let through quietly.
func NewDecoder(rc io.ReadCloser, inst core.Instrument) (*Decoder, error) {
	br := bufio.NewReader(rc)
	head, err := br.ReadSlice('\n')
	if err != nil && err != io.EOF {
		return nil, fmt.Errorf("databento: read header: %w", err)
	}
	if got := string(trimRow(head)); got != expectedHeader {
		return nil, fmt.Errorf("databento: unexpected header %q, want %q", got, expectedHeader)
	}
	return &Decoder{rc: rc, r: br, inst: inst, line: 1}, nil
}

// Close closes the underlying reader.
func (d *Decoder) Close() error { return d.rc.Close() }

// Next implements feed.Source.
func (d *Decoder) Next(dst *marketdata.Event) error {
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

	var offs [numColumns][2]int
	if n := splitFields(line, &offs); n != numColumns {
		return fmt.Errorf("databento: line %d: got %d fields, want %d", d.line, n, numColumns)
	}

	instrumentID, err := strconv.ParseUint(field(line, &offs, colInstrumentID), 10, 32)
	if err != nil {
		return fmt.Errorf("databento: line %d: instrument_id: %w", d.line, err)
	}
	if core.InstrumentID(instrumentID) != d.inst.ID {
		return fmt.Errorf("databento: line %d: instrument_id %d does not match configured instrument %d (%s)",
			d.line, instrumentID, d.inst.ID, d.inst.Symbol)
	}

	tsRecv, err := parseTsNano(field(line, &offs, colTsRecv))
	if err != nil {
		return fmt.Errorf("databento: line %d: ts_recv: %w", d.line, err)
	}
	tsEvent, err := parseTsNano(field(line, &offs, colTsEvent))
	if err != nil {
		return fmt.Errorf("databento: line %d: ts_event: %w", d.line, err)
	}
	action := field(line, &offs, colAction)
	side := field(line, &offs, colSide)

	flags, err := strconv.ParseUint(field(line, &offs, colFlags), 10, 8)
	if err != nil {
		return fmt.Errorf("databento: line %d: flags: %w", d.line, err)
	}
	tsInDelta, err := strconv.ParseInt(field(line, &offs, colTsInDelta), 10, 32)
	if err != nil {
		return fmt.Errorf("databento: line %d: ts_in_delta: %w", d.line, err)
	}
	sequence, err := strconv.ParseUint(field(line, &offs, colSequence), 10, 32)
	if err != nil {
		return fmt.Errorf("databento: line %d: sequence: %w", d.line, err)
	}

	// bid/ask price: UNDEF_PRICE (math.MaxInt64 or math.MinInt64 in
	// nanounits) passes straight through as core.Ticks — see the
	// UNDEF_PRICE decision above. TicksFrom would reject it as "not a
	// multiple of the tick size", which is correct for a real price but
	// wrong for a sentinel that says there is no price.
	bidPxNano, err := core.ParsePriceNano(field(line, &offs, colBidPx))
	if err != nil {
		return fmt.Errorf("databento: line %d: bid_px_00: %w", d.line, err)
	}
	bidPx := core.Ticks(bidPxNano)
	if bidPxNano != math.MaxInt64 && bidPxNano != math.MinInt64 {
		bidPx, err = d.inst.TicksFrom(bidPxNano)
		if err != nil {
			return fmt.Errorf("databento: line %d: bid_px_00: %w", d.line, err)
		}
	}
	askPxNano, err := core.ParsePriceNano(field(line, &offs, colAskPx))
	if err != nil {
		return fmt.Errorf("databento: line %d: ask_px_00: %w", d.line, err)
	}
	askPx := core.Ticks(askPxNano)
	if askPxNano != math.MaxInt64 && askPxNano != math.MinInt64 {
		askPx, err = d.inst.TicksFrom(askPxNano)
		if err != nil {
			return fmt.Errorf("databento: line %d: ask_px_00: %w", d.line, err)
		}
	}
	bidSz, err := strconv.ParseInt(field(line, &offs, colBidSz), 10, 64)
	if err != nil {
		return fmt.Errorf("databento: line %d: bid_sz_00: %w", d.line, err)
	}
	askSz, err := strconv.ParseInt(field(line, &offs, colAskSz), 10, 64)
	if err != nil {
		return fmt.Errorf("databento: line %d: ask_sz_00: %w", d.line, err)
	}
	bidCt, err := strconv.ParseUint(field(line, &offs, colBidCt), 10, 32)
	if err != nil {
		return fmt.Errorf("databento: line %d: bid_ct_00: %w", d.line, err)
	}
	askCt, err := strconv.ParseUint(field(line, &offs, colAskCt), 10, 32)
	if err != nil {
		return fmt.Errorf("databento: line %d: ask_ct_00: %w", d.line, err)
	}

	quote := marketdata.Quote{
		BidPx: bidPx, AskPx: askPx,
		BidQty: core.Qty(bidSz), AskQty: core.Qty(askSz),
		BidCt: uint32(bidCt), AskCt: uint32(askCt),
	}

	// envelope is the part every Event split from this row shares.
	envelope := marketdata.Event{
		Instrument: core.InstrumentID(instrumentID),
		TsEvent:    tsEvent,
		TsRecv:     tsRecv,
		TsInDelta:  int32(tsInDelta),
		Sequence:   uint32(sequence),
		Flags:      marketdata.Flags(flags),
	}

	if action == actionTrade {
		// A trade must carry a real, tick-aligned price: unlike a quote
		// side, there is no legitimate "no price" execution, so an
		// UNDEF_PRICE or off-tick price here is a hard decode error, not
		// a passthrough sentinel.
		priceNano, err := core.ParsePriceNano(field(line, &offs, colPrice))
		if err != nil {
			return fmt.Errorf("databento: line %d: price: %w", d.line, err)
		}
		tradePx, err := d.inst.TicksFrom(priceNano)
		if err != nil {
			return fmt.Errorf("databento: line %d: price: %w", d.line, err)
		}
		size, err := strconv.ParseInt(field(line, &offs, colSize), 10, 64)
		if err != nil {
			return fmt.Errorf("databento: line %d: size: %w", d.line, err)
		}

		var aggr core.Side
		switch side {
		case "B":
			aggr = core.SideBid // buyer aggressor: lifted the ask
		case "A":
			aggr = core.SideAsk // seller aggressor: hit the bid
		default:
			aggr = core.SideNone
		}

		*dst = envelope
		dst.Kind = marketdata.KindTrade
		dst.Trade = marketdata.Trade{Px: tradePx, Qty: core.Qty(size), Aggressor: aggr}

		d.pendingEvt = envelope
		d.pendingEvt.Kind = marketdata.KindQuote
		d.pendingEvt.Quote = quote
		d.pending = true
		return nil
	}

	*dst = envelope
	dst.Kind = marketdata.KindQuote
	dst.Quote = quote
	return nil
}

// trimRow strips a trailing \r\n or \n, so the header check and
// splitFields both work the same whether the file has Windows or Unix
// line endings.
func trimRow(b []byte) []byte {
	for len(b) > 0 && (b[len(b)-1] == '\n' || b[len(b)-1] == '\r') {
		b = b[:len(b)-1]
	}
	return b
}

// splitFields fills out[:n] with the [start,end) byte offsets of each
// comma-separated field in line and reports n. It never allocates:
// slicing a []byte, unlike converting one to a string, never copies.
func splitFields(line []byte, out *[numColumns][2]int) int {
	n := 0
	start := 0
	end := len(line)
	for end > 0 && (line[end-1] == '\n' || line[end-1] == '\r') {
		end--
	}
	for i := 0; i < end && n < numColumns; i++ {
		if line[i] == ',' {
			out[n] = [2]int{start, i}
			n++
			start = i + 1
		}
	}
	if n < numColumns {
		out[n] = [2]int{start, end}
		n++
	}
	return n
}

// field converts one column of line to a string. This is the only
// place Next turns bytes into a string, and it stays allocation-free
// only because every function fed the result (parseTsNano,
// core.ParsePriceNano, strconv.ParseInt/ParseUint) clones the string
// before ever putting it in a returned error, instead of embedding
// it directly. See the leak comment on core.ParsePriceNano.
func field(line []byte, offs *[numColumns][2]int, i int) string {
	return string(line[offs[i][0]:offs[i][1]])
}

// parseTsNano converts Databento's RFC3339Nano timestamp text straight
// to Unix nanoseconds, the same way cmd/census's observeTimestamp does
// (time.Parse(time.RFC3339Nano, raw)). Measured zero-alloc for a fresh
// substring of a reused line buffer — see the decision above for why
// that measurement, and not just cmd/census's existing
// BenchmarkTimeParse, is the one that actually matters here.
func parseTsNano(s string) (int64, error) {
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		return 0, err
	}
	return t.UnixNano(), nil
}
