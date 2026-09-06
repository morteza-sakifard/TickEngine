package main

import (
	"fmt"
	"math"
	"strings"
)

const (
	// nanoDigits is the number of decimal places Databento writes for
	// prices in CSV output.
	nanoDigits = 9
	nanoScale  = 1_000_000_000
)

// priceState separates the three things a price column can mean.
type priceState uint8

const (
	// priceOK is a real, usable price.
	priceOK priceState = iota
	// priceUndef is Databento's UNDEF_PRICE (INT64_MAX in 1e-9 units,
	// written as "9223372036.854775807"). It means "there is no price
	// here" — an empty book side, or an event that carries no price. It
	// is valid data, not corruption.
	priceUndef
	// priceFail means the column did not parse at all.
	priceFail
)

// classifyPrice parses a fixed-point decimal string into int64 nanounits
// without going through float64, and reports whether the result is a real
// price, Databento's undefined sentinel, or unparseable.
func classifyPrice(s string) (int64, priceState) {
	nano, err := parsePriceNano(s)
	if err != nil {
		return 0, priceFail
	}
	if nano == math.MaxInt64 || nano == math.MinInt64 {
		return nano, priceUndef
	}
	return nano, priceOK
}

// parsePriceNano converts a decimal string to int64 nanounits exactly.
//
// float64 is deliberately avoided. It happens to be exact for ES, whose
// 0.25 tick is representable in binary, but it is not exact for
// instruments like CL (0.01 tick), and integer arithmetic is what
// tick-grouping and price-level map keys actually need.
func parsePriceNano(s string) (int64, error) {
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
	if len(fracPart) > nanoDigits {
		return 0, fmt.Errorf("more than %d decimal places in %q", nanoDigits, s)
	}

	var whole int64
	for i := 0; i < len(intPart); i++ {
		d := int64(intPart[i] - '0')
		if whole > (math.MaxInt64-d)/10 {
			return 0, fmt.Errorf("integer part of %q overflows int64", s)
		}
		whole = whole*10 + d
	}

	// Right-pad the fraction to exactly nanoDigits.
	var frac int64
	for i := range nanoDigits {
		var d int64
		if i < len(fracPart) {
			d = int64(fracPart[i] - '0')
		}
		frac = frac*10 + d
	}

	if whole > math.MaxInt64/nanoScale {
		return 0, fmt.Errorf("%q overflows int64 nanounits", s)
	}
	nano := whole * nanoScale
	if nano > math.MaxInt64-frac {
		// MinInt64 = -(MaxInt64 + 1). The positive magnitude does not
		// fit in int64; detect it before negation.
		if neg && whole == math.MaxInt64/nanoScale && frac == math.MaxInt64%nanoScale+1 {
			return math.MinInt64, nil
		}
		return 0, fmt.Errorf("%q overflows int64 nanounits", s)
	}
	nano += frac

	if neg {
		nano = -nano
	}
	return nano, nil
}

// formatNano renders int64 nanounits back to a trimmed decimal string.
func formatNano(nano int64) string {
	if nano == math.MaxInt64 || nano == math.MinInt64 {
		return "UNDEF"
	}
	sign := ""
	if nano < 0 {
		sign, nano = "-", -nano
	}
	s := fmt.Sprintf("%d.%09d", nano/nanoScale, nano%nanoScale)
	s = strings.TrimRight(s, "0")
	s = strings.TrimSuffix(s, ".")
	return sign + s
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
