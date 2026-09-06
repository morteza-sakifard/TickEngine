package core

import (
	"fmt"
	"math"
	"strings"
)

const (
	// NanoDigits is how many fractional digits Databento writes for prices.
	NanoDigits = 9
	NanoScale  = 1_000_000_000
)

// ParsePriceNano converts a decimal string to int64 nanounits
// without going through float64.
//
//	"6713.500000000" -> 6713_500_000_000
//	"6713.50"        -> 6713_500_000_000  (short fraction is right-padded)
//	"1.0000000001"   -> error             (more than 9 places)
func ParsePriceNano(s string) (int64, error) {
	if s == "" {
		return 0, fmt.Errorf("empty price")
	}

	neg := false
	switch s[0] {
	case '-':
		neg, s = true, s[1:]
	case '+':
		s = s[1:]
	}

	intPart, fracPart := s, ""
	if i := strings.IndexByte(s, '.'); i >= 0 {
		intPart, fracPart = s[:i], s[i+1:]
	}
	if intPart == "" && fracPart == "" {
		return 0, fmt.Errorf("no digits in %q", s)
	}
	if intPart != "" && !allDigits(intPart) {
		return 0, fmt.Errorf("integer part of %q is not numeric", s)
	}
	if fracPart != "" && !allDigits(fracPart) {
		return 0, fmt.Errorf("fractional part of %q is not numeric", s)
	}
	if len(fracPart) > NanoDigits {
		return 0, fmt.Errorf("more than %d decimal places in %q", NanoDigits, s)
	}

	var whole uint64
	for i := 0; i < len(intPart); i++ {
		d := uint64(intPart[i] - '0')
		if whole > (math.MaxUint64-d)/10 {
			return 0, fmt.Errorf("integer part of %q overflows", s)
		}
		whole = whole*10 + d
	}

	var frac uint64
	for i := range NanoDigits {
		var d uint64
		if i < len(fracPart) {
			d = uint64(fracPart[i] - '0')
		}
		frac = frac*10 + d
	}

	if whole > math.MaxUint64/NanoScale {
		return 0, fmt.Errorf("%q overflows int64 nanounits", s)
	}
	mag := whole * NanoScale
	if mag > math.MaxUint64-frac {
		return 0, fmt.Errorf("%q overflows int64 nanounits", s)
	}
	mag += frac

	if neg {
		// MinInt64 = -(MaxInt64 + 1). The positive magnitude does not
		// fit in int64; detect it before the cast.
		if mag > uint64(math.MaxInt64)+1 {
			return 0, fmt.Errorf("%q overflows int64 nanounits", s)
		}
		if mag == uint64(math.MaxInt64)+1 {
			return math.MinInt64, nil
		}
		return -int64(mag), nil
	}
	if mag > uint64(math.MaxInt64) {
		return 0, fmt.Errorf("%q overflows int64 nanounits", s)
	}
	return int64(mag), nil
}

// FormatNano renders nanounits in Databento's 9-decimal form.
func FormatNano(nano int64) string {
	if nano == math.MinInt64 {
		return "-9223372036.854775808"
	}
	sign := ""
	if nano < 0 {
		sign, nano = "-", -nano
	}
	return fmt.Sprintf("%s%d.%09d", sign, nano/NanoScale, nano%NanoScale)
}

func allDigits(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

func trimFrac(s string) string {
	s = strings.TrimRight(s, "0")
	s = strings.TrimSuffix(s, ".")
	if s == "-" || s == "" {
		return "0"
	}
	return s
}
