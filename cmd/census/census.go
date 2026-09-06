package main

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"math"
	"sort"
	"strconv"
	"time"
)

const (
	actionTrade = "T"

	// undefSize is Databento's UNDEF_ORDER_SIZE (UINT32_MAX).
	undefSize = int64(math.MaxUint32)
)

var requiredColumns = []string{
	"ts_recv", "ts_event", "rtype", "publisher_id", "instrument_id",
	"action", "side", "depth", "price", "size", "flags", "ts_in_delta",
	"sequence", "bid_px_00", "ask_px_00", "bid_sz_00", "ask_sz_00", "symbol",
}

// Expected value sets. Anything outside them is still counted and gets
// flagged in the report — that is how you discover an action you did not
// know existed.
var (
	actionOrder = []string{"A", "M", "C", "R", "T", "F", "N"}
	sideOrder   = []string{"B", "A", "N"}
)

type Options struct {
	TickNano int64
	Loc      *time.Location
	Limit    int64 // 0 means no limit
}

// tsStat tracks range and monotonicity of one timestamp column.
type tsStat struct {
	Min, Max  time.Time
	prev      time.Time
	Decreases int64
	MaxBackNs int64
	Fail      int64
}

func (s *tsStat) add(t time.Time) {
	if s.Min.IsZero() || t.Before(s.Min) {
		s.Min = t
	}
	if s.Max.IsZero() || t.After(s.Max) {
		s.Max = t
	}
	if !s.prev.IsZero() && t.Before(s.prev) {
		s.Decreases++
		if back := s.prev.Sub(t).Nanoseconds(); back > s.MaxBackNs {
			s.MaxBackNs = back
		}
	}
	s.prev = t
}

// priceStat tracks the distribution of one price column.
type priceStat struct {
	MinNano, MaxNano int64
	seen             bool
	OffTick          int64
	Undef            int64
	Fail             int64
}

func (p *priceStat) record(nano, tickNano int64) {
	if !p.seen {
		p.MinNano, p.MaxNano, p.seen = nano, nano, true
	} else {
		if nano < p.MinNano {
			p.MinNano = nano
		}
		if nano > p.MaxNano {
			p.MaxNano = nano
		}
	}
	if nano%tickNano != 0 {
		p.OffTick++
	}
}

type symbolStat struct {
	Symbol       string
	InstrumentID string
	Records      int64
	First, Last  time.Time
}

type dayStat struct {
	Date        string
	Records     int64
	Trades      int64
	Volume      int64
	First, Last time.Time
}

type Census struct {
	opt    Options
	Header []string

	Records      int64
	LimitReached bool

	RType     map[string]int64
	Publisher map[string]int64
	Depth     map[string]int64
	Symbols   map[string]*symbolStat

	Actions    map[string]int64
	Sides      map[string]int64
	ActionSide map[string]int64

	FlagBits    map[string]int64
	FlagRaw     map[uint8]int64
	FlagUnknown int64
	FlagFail    int64

	TsRecv  tsStat
	TsEvent tsStat

	RecvMinusEvent  *Hist
	RecvBeforeEvent int64
	InDelta         *Hist
	InDeltaZero     int64
	InDeltaFail     int64

	Price      priceStat
	TradePrice priceStat

	QuoteBoth    int64
	QuoteBidOnly int64
	QuoteAskOnly int64
	QuoteNeither int64
	UndefBidPx   int64
	UndefAskPx   int64
	LockedBook   int64
	CrossedBook  int64
	SpreadTicks  *Hist

	Trades          int64
	TradeSideCount  map[string]int64
	TradeSideVolume map[string]int64
	TradeSizeMin    int64
	TradeSizeMax    int64
	tradeSizeSeen   bool
	SizeUndef       int64
	SizeFail        int64

	Days map[string]*dayStat
}

func newCensus(opt Options) *Census {
	return &Census{
		opt:       opt,
		RType:     map[string]int64{},
		Publisher: map[string]int64{},
		Depth:     map[string]int64{},
		Symbols:   map[string]*symbolStat{},

		Actions:    map[string]int64{},
		Sides:      map[string]int64{},
		ActionSide: map[string]int64{},

		FlagBits: map[string]int64{},
		FlagRaw:  map[uint8]int64{},

		// 10us buckets up to 100ms: feed latency lives here.
		RecvMinusEvent: NewHist(10_000, 10_000),
		// 1us buckets up to 10ms: ts_in_delta is much tighter.
		InDelta: NewHist(1_000, 10_000),
		// 1-tick buckets up to 1000 ticks.
		SpreadTicks: NewHist(1, 1_000),

		TradeSideCount:  map[string]int64{},
		TradeSideVolume: map[string]int64{},

		Days: map[string]*dayStat{},
	}
}

// Run scans the CSV and returns the collected census.
func Run(r io.Reader, opt Options) (*Census, error) {
	if opt.TickNano <= 0 {
		return nil, errors.New("census: TickNano must be positive")
	}
	if opt.Loc == nil {
		opt.Loc = time.UTC
	}

	cr := csv.NewReader(r)
	cr.ReuseRecord = true

	header, err := cr.Read()
	if err != nil {
		return nil, fmt.Errorf("read header: %w", err)
	}

	col := make(map[string]int, len(header))
	for i, name := range header {
		col[name] = i
	}
	for _, name := range requiredColumns {
		if _, ok := col[name]; !ok {
			return nil, fmt.Errorf("missing required column %q", name)
		}
	}

	c := newCensus(opt)
	// ReuseRecord reuses the slice, so copy the header before the next Read.
	c.Header = append([]string(nil), header...)

	for {
		row, err := cr.Read()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, fmt.Errorf("read row %d: %w", c.Records+1, err)
		}
		c.observe(row, col)
		if opt.Limit > 0 && c.Records >= opt.Limit {
			c.LimitReached = true
			break
		}
	}
	return c, nil
}

func (c *Census) observe(row []string, col map[string]int) {
	at := func(name string) string { return row[col[name]] }

	c.Records++

	c.RType[at("rtype")]++
	c.Publisher[at("publisher_id")]++
	c.Depth[at("depth")]++

	action, side := at("action"), at("side")
	c.Actions[action]++
	c.Sides[side]++
	c.ActionSide[action+" / "+side]++

	recv, recvOK := c.observeTimestamp(&c.TsRecv, at("ts_recv"))
	event, eventOK := c.observeTimestamp(&c.TsEvent, at("ts_event"))

	if recvOK && eventOK {
		d := recv.Sub(event).Nanoseconds()
		if d < 0 {
			c.RecvBeforeEvent++
		}
		c.RecvMinusEvent.Add(d)
	}

	if v, err := strconv.ParseInt(at("ts_in_delta"), 10, 64); err != nil {
		c.InDeltaFail++
	} else {
		if v == 0 {
			c.InDeltaZero++
		}
		c.InDelta.Add(v)
	}

	if v, err := strconv.ParseUint(at("flags"), 10, 8); err != nil {
		c.FlagFail++
	} else {
		raw := uint8(v)
		c.FlagRaw[raw]++
		for _, name := range flagNames(raw) {
			c.FlagBits[name]++
		}
		if unknownFlagBits(raw) != 0 {
			c.FlagUnknown++
		}
	}

	pxNano, pxState := classifyPrice(at("price"))
	recordPrice(&c.Price, pxNano, pxState, c.opt.TickNano)

	size, sizeOK := c.observeSize(at("size"))
	c.observeQuote(at("bid_px_00"), at("ask_px_00"))

	sym := c.symbolStat(orEmpty(at("symbol")), at("instrument_id"))
	sym.Records++
	if recvOK {
		extend(&sym.First, &sym.Last, recv)
	}

	var day *dayStat
	if recvOK {
		day = c.dayStat(recv.In(c.opt.Loc).Format("2006-01-02"))
		day.Records++
		extend(&day.First, &day.Last, recv)
	}

	if action != actionTrade {
		return
	}

	c.Trades++
	c.TradeSideCount[side]++
	if sizeOK {
		c.TradeSideVolume[side] += size
		if !c.tradeSizeSeen {
			c.TradeSizeMin, c.TradeSizeMax, c.tradeSizeSeen = size, size, true
		} else {
			if size < c.TradeSizeMin {
				c.TradeSizeMin = size
			}
			if size > c.TradeSizeMax {
				c.TradeSizeMax = size
			}
		}
	}
	recordPrice(&c.TradePrice, pxNano, pxState, c.opt.TickNano)
	if day != nil {
		day.Trades++
		if sizeOK {
			day.Volume += size
		}
	}
}

// recordPrice routes a classified price into the right counter. Undefined
// and unparseable prices must never reach record(), or they poison min,
// max and the off-tick count.
func recordPrice(p *priceStat, nano int64, state priceState, tickNano int64) {
	switch state {
	case priceOK:
		p.record(nano, tickNano)
	case priceUndef:
		p.Undef++
	case priceFail:
		p.Fail++
	}
}

func (c *Census) observeTimestamp(s *tsStat, raw string) (time.Time, bool) {
	t, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil {
		s.Fail++
		return time.Time{}, false
	}
	t = t.UTC()
	s.add(t)
	return t, true
}

func (c *Census) observeSize(raw string) (int64, bool) {
	v, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		c.SizeFail++
		return 0, false
	}
	if v == undefSize {
		c.SizeUndef++
		return 0, false
	}
	return v, true
}

func (c *Census) observeQuote(bidRaw, askRaw string) {
	bid, bidState := classifyPrice(bidRaw)
	ask, askState := classifyPrice(askRaw)

	if bidState == priceUndef {
		c.UndefBidPx++
	}
	if askState == priceUndef {
		c.UndefAskPx++
	}

	bidOK, askOK := bidState == priceOK, askState == priceOK
	switch {
	case bidOK && askOK:
		c.QuoteBoth++
	case bidOK:
		c.QuoteBidOnly++
	case askOK:
		c.QuoteAskOnly++
	default:
		c.QuoteNeither++
	}

	if !bidOK || !askOK {
		return
	}
	switch {
	case bid == ask:
		c.LockedBook++
	case bid > ask:
		c.CrossedBook++
	default:
		c.SpreadTicks.Add((ask - bid) / c.opt.TickNano)
	}
}

func (c *Census) symbolStat(symbol, instrumentID string) *symbolStat {
	s, ok := c.Symbols[symbol]
	if !ok {
		s = &symbolStat{Symbol: symbol, InstrumentID: instrumentID}
		c.Symbols[symbol] = s
	}
	return s
}

func (c *Census) dayStat(date string) *dayStat {
	d, ok := c.Days[date]
	if !ok {
		d = &dayStat{Date: date}
		c.Days[date] = d
	}
	return d
}

// orEmpty keeps a blank column from becoming an unlabelled map key.
func orEmpty(s string) string {
	if s == "" {
		return "(empty)"
	}
	return s
}

func extend(first, last *time.Time, t time.Time) {
	if first.IsZero() || t.Before(*first) {
		*first = t
	}
	if last.IsZero() || t.After(*last) {
		*last = t
	}
}

// SortedDays returns the calendar dates present, ascending.
func (c *Census) SortedDays() []string {
	out := make([]string, 0, len(c.Days))
	for k := range c.Days {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
