package session

import (
	"fmt"
	"time"
)

// chicago is the timezone CME Equity Index futures are quoted in.
// CL and GC will need their own locations when those products get a
// Schedule; this function is the place that decision will branch.
func chicago() (*time.Location, error) {
	return time.LoadLocation("America/Chicago")
}

// ForProduct returns a Calendar for a futures product (the
// Instrument.Product field from step 2: "ES", not "ESZ5"). ES and NQ
// share CME Equity Index hours, including the holiday table in
// holidays.go. Any other product is an error — inventing a CL or GC
// schedule here would be a guess, and docs/00-architecture.md L3
// lists those as a later addition, not this step's job.
func ForProduct(product string) (Calendar, error) {
	loc, err := chicago()
	if err != nil {
		return Calendar{}, fmt.Errorf("session: load America/Chicago: %w", err)
	}
	switch product {
	case "ES":
		return Calendar{Location: loc, Schedule: ESRegularSchedule()}, nil
	case "NQ":
		return Calendar{Location: loc, Schedule: NQRegularSchedule()}, nil
	default:
		return Calendar{}, fmt.Errorf("session: no schedule for product %q", product)
	}
}
