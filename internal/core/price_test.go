package core

import (
	"math"
	"testing"
)

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
		{name: "no integer part", in: ".25", want: 250_000_000},
		{name: "tick size", in: "0.25", want: 250_000_000},
		{name: "negative", in: "-0.25", want: -250_000_000},
		{name: "plus sign", in: "+3.5", want: 3_500_000_000},
		{name: "zero", in: "0.000000000", want: 0},
		{name: "undef sentinel", in: "9223372036.854775807", want: math.MaxInt64},
		{name: "min int64", in: "-9223372036.854775808", want: math.MinInt64},
		{name: "ten decimals", in: "1.0000000001", wantErr: true},
		{name: "empty", in: "", wantErr: true},
		{name: "not a number", in: "abc", wantErr: true},
		{name: "just a dot", in: ".", wantErr: true},
		{name: "negative fraction", in: "1.-5", wantErr: true},
		{name: "overflow", in: "9223372037.000000000", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParsePriceNano(tt.in)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ParsePriceNano(%q) error = %v, wantErr %v", tt.in, err, tt.wantErr)
			}
			if err == nil && got != tt.want {
				t.Errorf("ParsePriceNano(%q) = %d, want %d", tt.in, got, tt.want)
			}
		})
	}
}

// TestParsePriceNanoAllocs is the L0 half of TestNextAllocs: a reused
// []byte is sliced to a string the same way feed/databento field()
// does. If an error path still embeds s instead of strings.Clone(s),
// this reports 1 alloc/op even though the input is well-formed.
func TestParsePriceNanoAllocs(t *testing.T) {
	buf := []byte("pad6713.500000000pad")
	const want int64 = 6713_500_000_000
	allocs := testing.AllocsPerRun(1000, func() {
		s := string(buf[3:17])
		n, err := ParsePriceNano(s)
		if err != nil || n != want {
			panic("ParsePriceNanoAllocs")
		}
	})
	if allocs != 0 {
		t.Fatalf("ParsePriceNano allocs/op = %v, want 0 (error paths must Clone the input)", allocs)
	}
}

func TestFormatNano(t *testing.T) {
	tests := []struct {
		in   int64
		want string
	}{
		{in: 6713_500_000_000, want: "6713.500000000"},
		{in: 250_000_000, want: "0.250000000"},
		{in: -250_000_000, want: "-0.250000000"},
		{in: 0, want: "0.000000000"},
		{in: math.MaxInt64, want: "9223372036.854775807"},
		{in: math.MinInt64, want: "-9223372036.854775808"},
	}
	for _, tt := range tests {
		if got := FormatNano(tt.in); got != tt.want {
			t.Errorf("FormatNano(%d) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestFormatNanoRoundTrip(t *testing.T) {
	inputs := []string{
		"0.000000000",
		"0.250000000",
		"6713.500000000",
		"-0.250000000",
		"9223372036.854775807",
		"-9223372036.854775808",
	}
	for _, s := range inputs {
		n, err := ParsePriceNano(s)
		if err != nil {
			t.Fatalf("ParsePriceNano(%q): %v", s, err)
		}
		if got := FormatNano(n); got != s {
			t.Errorf("round-trip %q -> %d -> %q", s, n, got)
		}
	}
}

func FuzzParsePriceNano(f *testing.F) {
	for _, s := range []string{
		"6713.500000000", "0.25", "-1", "", ".", "abc",
		"9223372036.854775807", "-9223372036.854775808",
		"+3.5", "1.2.3", "999999999999999999999",
	} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		_, _ = ParsePriceNano(s)
	})
}
