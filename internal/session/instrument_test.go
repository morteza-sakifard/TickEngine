package session

import (
	"testing"
	"time"
)

func TestForProductES(t *testing.T) {
	cal, err := ForProduct("ES")
	if err != nil {
		t.Fatal(err)
	}
	if cal.Location.String() != "America/Chicago" {
		t.Errorf("Location = %s, want America/Chicago", cal.Location)
	}
	if _, sess := cal.Classify(ctTime(t, 2025, time.September, 23, 10, 0, 0)); sess != RTH {
		t.Errorf("ES Tuesday 10:00 session = %v, want RTH", sess)
	}
}

func TestForProductNQSharesESHolidays(t *testing.T) {
	cal, err := ForProduct("NQ")
	if err != nil {
		t.Fatal(err)
	}
	if _, sess := cal.Classify(ctTime(t, 2025, time.November, 27, 10, 0, 0)); sess != Closed {
		t.Errorf("NQ Thanksgiving session = %v, want Closed (must share ES holiday table)", sess)
	}
}

func TestForProductUnknown(t *testing.T) {
	if _, err := ForProduct("CL"); err == nil {
		t.Fatal("expected an error for a product with no Schedule yet")
	}
}
