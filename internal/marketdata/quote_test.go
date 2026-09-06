package marketdata

import (
	"testing"

	"github.com/morteza-sakifard/market-data-lab/internal/core"
)

func TestQuoteSpread(t *testing.T) {
	tests := []struct {
		name     string
		bid, ask core.Ticks
		want     core.Ticks
	}{
		{name: "normal book", bid: 100, ask: 101, want: 1},
		{name: "locked book", bid: 100, ask: 100, want: 0},
		{name: "crossed book", bid: 101, ask: 100, want: -1},
		{name: "empty quote", bid: 0, ask: 0, want: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q := Quote{BidPx: tt.bid, AskPx: tt.ask}
			if got := q.Spread(); got != tt.want {
				t.Errorf("Spread() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestQuoteMid(t *testing.T) {
	tests := []struct {
		name     string
		bid, ask core.Ticks
		want     core.Ticks
	}{
		{name: "even sum", bid: 100, ask: 102, want: 101},
		{name: "odd sum truncates toward bid", bid: 100, ask: 101, want: 100},
		{name: "empty quote does not panic", bid: 0, ask: 0, want: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q := Quote{BidPx: tt.bid, AskPx: tt.ask}
			if got := q.Mid(); got != tt.want {
				t.Errorf("Mid() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestQuoteMicroprice(t *testing.T) {
	t.Run("empty quote does not panic", func(t *testing.T) {
		var q Quote
		if got := q.Microprice(); got != 0 {
			t.Errorf("Microprice() on empty quote = %v, want 0", got)
		}
	})

	t.Run("balanced size is the simple mid", func(t *testing.T) {
		q := Quote{BidPx: 100, AskPx: 102, BidQty: 50, AskQty: 50}
		if got, want := q.Microprice(), 101.0; got != want {
			t.Errorf("Microprice() = %v, want %v", got, want)
		}
	})

	t.Run("heavier bid size pulls the estimate toward the ask", func(t *testing.T) {
		q := Quote{BidPx: 100, AskPx: 101, BidQty: 90, AskQty: 10}
		got := q.Microprice()
		if got <= 100 || got >= 101 {
			t.Fatalf("Microprice() = %v, want strictly between BidPx and AskPx", got)
		}
		if got <= 100.5 {
			t.Errorf("Microprice() = %v, want above the simple mid since BidQty > AskQty", got)
		}
	})
}

func TestQuoteImbalance(t *testing.T) {
	tests := []struct {
		name           string
		bidQty, askQty core.Qty
		want           float64
	}{
		{name: "balanced", bidQty: 50, askQty: 50, want: 0},
		{name: "all bid", bidQty: 100, askQty: 0, want: 1},
		{name: "all ask", bidQty: 0, askQty: 100, want: -1},
		{name: "empty quote does not panic", bidQty: 0, askQty: 0, want: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q := Quote{BidQty: tt.bidQty, AskQty: tt.askQty}
			if got := q.Imbalance(); got != tt.want {
				t.Errorf("Imbalance() = %v, want %v", got, tt.want)
			}
		})
	}
}
