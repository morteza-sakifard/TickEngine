package main

import (
	"bytes"
	"os"
	"slices"
	"strings"
	"testing"
	"time"
)

const fixturePath = "../../testdata/mbp1_sample.csv"

func seq(from, to int64) []int64 {
	out := make([]int64, 0, to-from+1)
	for v := from; v <= to; v++ {
		out = append(out, v)
	}
	return out
}

func TestHistPercentile(t *testing.T) {
	tests := []struct {
		name   string
		width  int64
		n      int64
		values []int64
		p      float64
		want   int64
	}{
		{name: "uniform 1..100 median", width: 1, n: 1000, values: seq(1, 100), p: 0.5, want: 50},
		{name: "single value", width: 1, n: 10, values: []int64{7}, p: 0.5, want: 7},
		{name: "p99 of 1..100", width: 1, n: 1000, values: seq(1, 100), p: 0.99, want: 99},
		{name: "bucket width 10 rounds down", width: 10, n: 100, values: []int64{0, 5, 9, 15}, p: 0.5, want: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewHist(tt.width, tt.n)
			for _, v := range tt.values {
				h.Add(v)
			}
			if got := h.Percentile(tt.p); got != tt.want {
				t.Errorf("Percentile(%v) = %d, want %d", tt.p, got, tt.want)
			}
		})
	}
}

func TestHistOverflow(t *testing.T) {
	h := NewHist(1, 10)
	h.Add(5)
	h.Add(100)
	h.Add(1000)

	if got := h.Overflow(); got != 2 {
		t.Errorf("Overflow() = %d, want 2", got)
	}
	if got := h.Count(); got != 3 {
		t.Errorf("Count() = %d, want 3", got)
	}
	if got := h.Max(); got != 1000 {
		t.Errorf("Max() = %d, want 1000 (Max must stay exact, not bucketed)", got)
	}
}

func TestHistNegative(t *testing.T) {
	h := NewHist(1, 10)
	h.Add(-50)
	h.Add(3)

	if got := h.Under(); got != 1 {
		t.Errorf("Under() = %d, want 1", got)
	}
	if got := h.Min(); got != -50 {
		t.Errorf("Min() = %d, want -50", got)
	}
}

func TestHistEmpty(t *testing.T) {
	h := NewHist(1, 10)
	if got := h.Percentile(0.5); got != 0 {
		t.Errorf("Percentile on empty = %d, want 0", got)
	}
	if got := h.Count(); got != 0 {
		t.Errorf("Count() = %d, want 0", got)
	}
}

func TestFlagNames(t *testing.T) {
	tests := []struct {
		name string
		raw  uint8
		want []string
	}{
		{name: "row 1 of the real file", raw: 168, want: []string{"LAST", "SNAPSHOT", "BAD_TS_RECV"}},
		{name: "row 2 of the real file", raw: 128, want: []string{"LAST"}},
		{name: "none", raw: 0, want: nil},
		{
			name: "all six",
			raw:  0x80 | 0x40 | 0x20 | 0x10 | 0x08 | 0x04,
			want: []string{"LAST", "TOB", "SNAPSHOT", "MBP", "BAD_TS_RECV", "MAYBE_BAD_BOOK"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := flagNames(tt.raw); !slices.Equal(got, tt.want) {
				t.Errorf("flagNames(%d) = %v, want %v", tt.raw, got, tt.want)
			}
		})
	}
}

func TestUnknownFlagBits(t *testing.T) {
	if got := unknownFlagBits(168); got != 0 {
		t.Errorf("unknownFlagBits(168) = 0x%02X, want 0", got)
	}
	if got := unknownFlagBits(0x03); got != 0x03 {
		t.Errorf("unknownFlagBits(0x03) = 0x%02X, want 0x03", got)
	}
}

func TestParsePriceNano(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    int64
		wantErr bool
	}{
		{name: "nine decimals", in: "6713.500000000", want: 6713_500_000_000},
		{name: "two decimals", in: "6713.50", want: 6713_500_000_000},
		{name: "integer", in: "6713", want: 6713_000_000_000},
		{name: "tick size", in: "0.25", want: 250_000_000},
		{name: "no integer part", in: ".25", want: 250_000_000},
		{name: "negative", in: "-0.25", want: -250_000_000},
		{name: "zero", in: "0.000000000", want: 0},
		{name: "undef price sentinel", in: "9223372036.854775807", want: 9223372036854775807},
		{name: "min int64", in: "-9223372036.854775808", want: -9223372036854775808},
		{name: "ten decimals", in: "1.0000000001", wantErr: true},
		{name: "empty", in: "", wantErr: true},
		{name: "not a number", in: "abc", wantErr: true},
		{name: "negative fraction", in: "1.-5", wantErr: true},
		{name: "just a dot", in: ".", wantErr: true},
		{name: "overflow", in: "9223372037.000000000", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parsePriceNano(tt.in)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parsePriceNano(%q) error = %v, wantErr %v", tt.in, err, tt.wantErr)
			}
			if err == nil && got != tt.want {
				t.Errorf("parsePriceNano(%q) = %d, want %d", tt.in, got, tt.want)
			}
		})
	}
}

func TestClassifyPrice(t *testing.T) {
	tests := []struct {
		in   string
		want priceState
	}{
		{in: "6713.500000000", want: priceOK},
		{in: "9223372036.854775807", want: priceUndef},
		{in: "-9223372036.854775808", want: priceUndef},
		{in: "0.000000000", want: priceOK},
		{in: "", want: priceFail},
	}

	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			if _, got := classifyPrice(tt.in); got != tt.want {
				t.Errorf("classifyPrice(%q) state = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestFormatNano(t *testing.T) {
	tests := []struct {
		in   int64
		want string
	}{
		{in: 6713_500_000_000, want: "6713.5"},
		{in: 250_000_000, want: "0.25"},
		{in: -250_000_000, want: "-0.25"},
		{in: 0, want: "0"},
		{in: 9223372036854775807, want: "UNDEF"},
	}

	for _, tt := range tests {
		if got := formatNano(tt.in); got != tt.want {
			t.Errorf("formatNano(%d) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func FuzzParsePriceNano(f *testing.F) {
	for _, s := range []string{
		"6713.500000000", "0.25", "-1", "", ".", "abc",
		"9223372036.854775807", "+3.5", "1.2.3", "999999999999999999999",
	} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		// The only contract under fuzzing is "never panic".
		_, _ = parsePriceNano(s)
	})
}

func runFixture(t *testing.T, opt Options) *Census {
	t.Helper()
	data, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatal(err)
	}
	if opt.TickNano == 0 {
		opt.TickNano = 250_000_000
	}
	if opt.Loc == nil {
		opt.Loc = time.UTC
	}
	c, err := Run(strings.NewReader(string(data)), opt)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestCensusOnFixture(t *testing.T) {
	c := runFixture(t, Options{})

	counts := []struct {
		name string
		got  int64
		want int64
	}{
		{"Records", c.Records, 8},
		{"Trades", c.Trades, 3},

		{"flag LAST", c.FlagBits["LAST"], 8},
		{"flag SNAPSHOT", c.FlagBits["SNAPSHOT"], 1},
		{"flag BAD_TS_RECV", c.FlagBits["BAD_TS_RECV"], 1},
		{"flag MAYBE_BAD_BOOK", c.FlagBits["MAYBE_BAD_BOOK"], 1},
		{"flag TOB", c.FlagBits["TOB"], 0},
		{"unknown flag bits", c.FlagUnknown, 0},

		{"action A", c.Actions["A"], 4},
		{"action C", c.Actions["C"], 1},
		{"action T", c.Actions["T"], 3},

		{"QuoteBoth", c.QuoteBoth, 7},
		{"QuoteBidOnly", c.QuoteBidOnly, 1},
		{"UndefAskPx", c.UndefAskPx, 1},
		{"UndefBidPx", c.UndefBidPx, 0},
		{"LockedBook", c.LockedBook, 0},
		{"CrossedBook", c.CrossedBook, 1},

		{"price undefined", c.Price.Undef, 1},
		{"price off-tick", c.Price.OffTick, 0},
		{"price parse failures", c.Price.Fail, 0},

		{"ts_in_delta == 0", c.InDeltaZero, 1},
		{"recv before event", c.RecvBeforeEvent, 0},

		{"trade volume side B", c.TradeSideVolume["B"], 3},
		{"trade volume side A", c.TradeSideVolume["A"], 5},
		{"trade volume side N", c.TradeSideVolume["N"], 1},
		{"trade min size", c.TradeSizeMin, 1},
		{"trade max size", c.TradeSizeMax, 5},
		{"size undefined", c.SizeUndef, 0},
	}

	for _, ch := range counts {
		if ch.got != ch.want {
			t.Errorf("%s = %d, want %d", ch.name, ch.got, ch.want)
		}
	}

	if got, want := formatNano(c.Price.MinNano), "6713.5"; got != want {
		t.Errorf("price min = %s, want %s", got, want)
	}
	if got, want := formatNano(c.Price.MaxNano), "6715.5"; got != want {
		t.Errorf("price max = %s, want %s (UNDEF must not leak into max)", got, want)
	}
	if got, want := formatNano(c.TradePrice.MinNano), "6714.75"; got != want {
		t.Errorf("trade price min = %s, want %s", got, want)
	}
	if got, want := formatNano(c.TradePrice.MaxNano), "6715"; got != want {
		t.Errorf("trade price max = %s, want %s", got, want)
	}

	if len(c.Symbols) != 1 {
		t.Errorf("Symbols = %d, want 1", len(c.Symbols))
	}
	if len(c.Days) != 1 {
		t.Errorf("Days = %d, want 1", len(c.Days))
	}
	if len(c.RType) != 1 || len(c.Publisher) != 1 || len(c.Depth) != 1 {
		t.Errorf("rtype/publisher_id/depth should each be constant, got %d/%d/%d",
			len(c.RType), len(c.Publisher), len(c.Depth))
	}

	// 7 rows have both quote sides, 1 of those is crossed, so 6 spreads.
	if got := c.SpreadTicks.Count(); got != 6 {
		t.Errorf("SpreadTicks count = %d, want 6", got)
	}
	if got := c.SpreadTicks.Max(); got != 1 {
		t.Errorf("SpreadTicks max = %d, want 1", got)
	}

	// Row 1 sits 121ms past the 100ms histogram range, so it overflows.
	// That is the point: the report must surface it instead of silently
	// reporting a wrong p99.
	if got := c.RecvMinusEvent.Overflow(); got != 1 {
		t.Errorf("RecvMinusEvent overflow = %d, want 1", got)
	}
}

func TestCensusLimit(t *testing.T) {
	c := runFixture(t, Options{Limit: 3})
	if c.Records != 3 {
		t.Errorf("Records = %d, want 3", c.Records)
	}
	if !c.LimitReached {
		t.Error("LimitReached = false, want true")
	}
	if c.Trades != 1 {
		t.Errorf("Trades = %d, want 1 (only row 3 is a trade in the first 3)", c.Trades)
	}
}

func TestCensusMissingColumn(t *testing.T) {
	in := "ts_recv,action\n2025-09-22T00:00:00.000000000Z,A\n"
	_, err := Run(strings.NewReader(in), Options{TickNano: 250_000_000})
	if err == nil {
		t.Fatal("expected an error for a CSV missing required columns")
	}
	if !strings.Contains(err.Error(), "missing required column") {
		t.Errorf("error = %v, want it to name the missing column", err)
	}
}

func TestCensusRejectsZeroTick(t *testing.T) {
	if _, err := Run(strings.NewReader(""), Options{TickNano: 0}); err == nil {
		t.Fatal("expected an error for TickNano == 0 (it would divide by zero)")
	}
}

func TestReportDoesNotPanic(t *testing.T) {
	c := runFixture(t, Options{})
	var buf bytes.Buffer
	c.Report(&buf)
	if buf.Len() == 0 {
		t.Fatal("Report wrote nothing")
	}
	for _, want := range []string{"MBP-1 dataset census", "SNAPSHOT", "crossed", "ESZ5"} {
		if !strings.Contains(buf.String(), want) {
			t.Errorf("report is missing %q", want)
		}
	}
}

func BenchmarkCensus(b *testing.B) {
	data, err := os.ReadFile(fixturePath)
	if err != nil {
		b.Fatal(err)
	}
	opt := Options{TickNano: 250_000_000, Loc: time.UTC}

	b.SetBytes(int64(len(data)))
	b.ReportAllocs()
	for b.Loop() {
		if _, err := Run(bytes.NewReader(data), opt); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkTimeParse(b *testing.B) {
	const s = "2025-09-22T00:00:00.010483640Z"
	b.ReportAllocs()
	for b.Loop() {
		if _, err := time.Parse(time.RFC3339Nano, s); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkParsePriceNano(b *testing.B) {
	const s = "6713.500000000"
	b.ReportAllocs()
	for b.Loop() {
		if _, err := parsePriceNano(s); err != nil {
			b.Fatal(err)
		}
	}
}
