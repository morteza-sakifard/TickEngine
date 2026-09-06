package core

import (
	"testing"
	"time"
)

func TestCivilDateRoundTrip(t *testing.T) {
	loc, err := time.LoadLocation("America/Chicago")
	if err != nil {
		t.Fatal(err)
	}
	ts := time.Date(2025, time.November, 24, 15, 45, 0, 0, loc)
	d := CivilDateOf(ts)
	if d != NewCivilDate(2025, time.November, 24) {
		t.Errorf("CivilDateOf = %v", d)
	}
	if d.String() != "2025-11-24" {
		t.Errorf("String = %q", d.String())
	}
	midnight := d.Time(loc)
	if y, m, day := midnight.Date(); y != 2025 || m != time.November || day != 24 {
		t.Errorf("Time = %v", midnight)
	}
	if h, min, s := midnight.Clock(); h != 0 || min != 0 || s != 0 {
		t.Errorf("Time should be midnight, got %d:%d:%d", h, min, s)
	}
}

func TestCivilDateMapKey(t *testing.T) {
	m := map[CivilDate]string{
		NewCivilDate(2025, time.December, 25): "closed",
	}
	if m[CivilDateOf(time.Date(2025, 12, 25, 8, 30, 0, 0, time.UTC))] != "closed" {
		t.Fatal("CivilDate must be usable as a map key")
	}
}
