package core

import (
	"fmt"
	"time"
)

// CivilDate is a calendar date with no time and no location.
// It is comparable, so it can be a map key (holiday tables).
type CivilDate struct {
	Year  int
	Month time.Month
	Day   int
}

func NewCivilDate(year int, month time.Month, day int) CivilDate {
	return CivilDate{Year: year, Month: month, Day: day}
}

func CivilDateOf(t time.Time) CivilDate {
	y, m, d := t.Date()
	return CivilDate{Year: y, Month: m, Day: d}
}

func (d CivilDate) String() string {
	return fmt.Sprintf("%04d-%02d-%02d", d.Year, int(d.Month), d.Day)
}

// Time is midnight of this date in loc. loc nil means UTC.
func (d CivilDate) Time(loc *time.Location) time.Time {
	if loc == nil {
		loc = time.UTC
	}
	return time.Date(d.Year, d.Month, d.Day, 0, 0, 0, 0, loc)
}
