package core

import (
	"testing"
	"time"
)

func TestTicksFrom(t *testing.T) {
	es := ESZ5()

	tests := []struct {
		name    string
		nano    int64
		want    Ticks
		wantErr bool
	}{
		{name: "6713.50", nano: 6713_500_000_000, want: 26854},
		{name: "zero", nano: 0, want: 0},
		{name: "one tick", nano: 250_000_000, want: 1},
		{name: "negative tick", nano: -250_000_000, want: -1},
		{name: "off tick", nano: 6713_510_000_000, wantErr: true},
		{name: "unaligned nano", nano: 1, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := es.TicksFrom(tt.nano)
			if (err != nil) != tt.wantErr {
				t.Fatalf("TicksFrom(%d) error = %v, wantErr %v", tt.nano, err, tt.wantErr)
			}
			if err == nil && got != tt.want {
				t.Errorf("TicksFrom(%d) = %d, want %d", tt.nano, got, tt.want)
			}
		})
	}
}

func TestTicksFromRejectsZeroTickSize(t *testing.T) {
	var i Instrument
	if _, err := i.TicksFrom(250_000_000); err == nil {
		t.Fatal("zero TickSizeNano must error (would divide by zero)")
	}
}

func TestNanoAndText(t *testing.T) {
	es := ESZ5()
	if got := es.Nano(26854); got != 6713_500_000_000 {
		t.Errorf("Nano(26854) = %d, want 6713500000000", got)
	}
	if got := es.Text(26854); got != "6713.5" {
		t.Errorf("Text(26854) = %q, want %q", got, "6713.5")
	}
	if got := es.Text(1); got != "0.25" {
		t.Errorf("Text(1) = %q, want %q", got, "0.25")
	}
}

func TestESZ5Identity(t *testing.T) {
	es := ESZ5()
	if es.Product != "ES" || es.Symbol != "ESZ5" {
		t.Fatalf("ESZ5 identity = %s/%s", es.Product, es.Symbol)
	}
	if es.Expiry != NewCivilDate(2025, time.December, 19) {
		t.Errorf("ESZ5 expiry = %s, want 2025-12-19", es.Expiry)
	}
}
