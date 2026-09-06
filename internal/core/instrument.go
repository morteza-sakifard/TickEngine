package core

import (
	"fmt"
	"time"
)

// InstrumentID is Databento's numeric instrument_id.
type InstrumentID uint32

// Instrument is the static description of one contract.
// Tick size lives here, not on each price: a tick is a property of
// the instrument. Product is the future rollover key (ESZ5 and ESH6
// share Product "ES") — no continuous contract is built in this step.
type Instrument struct {
	ID           InstrumentID
	Symbol       string // "ESZ5"
	Product      string // "ES"
	TickSizeNano int64  // 0.25 -> 250_000_000
	Multiplier   int64  // ES -> $50 per full point
	Currency     string
	Expiry       CivilDate
}

// TicksFrom converts nanounits to ticks. It errors when the price is
// not an exact tick multiple: in clean data that does not happen, so
// seeing it means the Instrument is wrong or the row is an implied
// / spread price that must be handled separately.
func (i Instrument) TicksFrom(nano int64) (Ticks, error) {
	if i.TickSizeNano <= 0 {
		return 0, fmt.Errorf("instrument %s: TickSizeNano must be positive", i.Symbol)
	}
	if nano%i.TickSizeNano != 0 {
		return 0, fmt.Errorf("price %s is not a multiple of tick %s",
			FormatNano(nano), FormatNano(i.TickSizeNano))
	}
	return Ticks(nano / i.TickSizeNano), nil
}

func (i Instrument) Nano(t Ticks) int64 {
	return int64(t) * i.TickSizeNano
}

// Text is for display. Canonical wire form is FormatNano(i.Nano(t)).
func (i Instrument) Text(t Ticks) string {
	return trimFrac(FormatNano(i.Nano(t)))
}

// ESZ5 is the front-month in the current dataset (instrument_id 294973).
// Expiry is the 3rd Friday of December 2025.
func ESZ5() Instrument {
	return Instrument{
		ID:           294973,
		Symbol:       "ESZ5",
		Product:      "ES",
		TickSizeNano: 250_000_000,
		Multiplier:   50,
		Currency:     "USD",
		Expiry:       NewCivilDate(2025, time.December, 19),
	}
}
