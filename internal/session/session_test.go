package session

import (
	"testing"
	"time"
)

func mustChicago(t *testing.T) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation("America/Chicago")
	if err != nil {
		t.Fatalf("LoadLocation(America/Chicago): %v", err)
	}
	return loc
}

func ctTime(t *testing.T, y int, m time.Month, d, hh, mm, ss int) time.Time {
	return time.Date(y, m, d, hh, mm, ss, 0, mustChicago(t))
}

// Test 5 — RTH classification.
func TestClassify_RTH(t *testing.T) {
	cal := Calendar{Location: mustChicago(t), Schedule: ESRegularSchedule()}

	// Tuesday 2025-09-23, 10:00 CT is inside RTH (08:30-15:15).
	trd, sess := cal.Classify(ctTime(t, 2025, time.September, 23, 10, 0, 0))
	if sess != RTH {
		t.Fatalf("session = %v, want RTH", sess)
	}
	if want := ctTime(t, 2025, time.September, 23, 0, 0, 0); !trd.Equal(want) {
		t.Fatalf("tradingDate = %v, want %v", trd, want)
	}
}

// Test 6 — ETH classification.
func TestClassify_ETH(t *testing.T) {
	cal := Calendar{Location: mustChicago(t), Schedule: ESRegularSchedule()}

	cases := []struct {
		name string
		when time.Time
		want time.Time
	}{
		{"evening", ctTime(t, 2025, time.September, 23, 20, 0, 0), ctTime(t, 2025, time.September, 24, 0, 0, 0)},
		{"overnight", ctTime(t, 2025, time.September, 24, 3, 0, 0), ctTime(t, 2025, time.September, 24, 0, 0, 0)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			trd, sess := cal.Classify(tc.when)
			if sess != ETH {
				t.Fatalf("session = %v, want ETH", sess)
			}
			if !trd.Equal(tc.want) {
				t.Fatalf("tradingDate = %v, want %v", trd, tc.want)
			}
		})
	}
}

// Test 7 — RTH/ETH boundary: half-open [RthOpen, RthClose).
func TestClassify_RTHBoundary(t *testing.T) {
	cal := Calendar{Location: mustChicago(t), Schedule: ESRegularSchedule()}

	if _, sess := cal.Classify(ctTime(t, 2025, time.September, 23, 8, 29, 59)); sess != ETH {
		t.Errorf("08:29:59 session = %v, want ETH", sess)
	}
	if _, sess := cal.Classify(ctTime(t, 2025, time.September, 23, 8, 30, 0)); sess != RTH {
		t.Errorf("08:30:00 session = %v, want RTH", sess)
	}
	if _, sess := cal.Classify(ctTime(t, 2025, time.September, 23, 15, 14, 59)); sess != RTH {
		t.Errorf("15:14:59 session = %v, want RTH", sess)
	}
	if _, sess := cal.Classify(ctTime(t, 2025, time.September, 23, 15, 15, 0)); sess != ETH {
		t.Errorf("15:15:00 session = %v, want ETH", sess)
	}
}

// Test 8 — trading date does not always equal calendar date.
//
// CME's own Globex glossary defines it this way: "The start of the CME
// Globex session usually occurs in the afternoon or evening, and marks the
// beginning of the next trading day. For example, orders entered during
// Sunday's evening session are dated for and cleared on Monday." So an
// evening trade (calendar date still "today") already belongs to
// tomorrow's trading date.
func TestClassify_TradingDateCrossesCalendarDate(t *testing.T) {
	cal := Calendar{Location: mustChicago(t), Schedule: ESRegularSchedule()}

	evening := ctTime(t, 2025, time.September, 22, 20, 0, 0) // Monday 8pm CT
	trd, sess := cal.Classify(evening)
	if sess != ETH {
		t.Fatalf("session = %v, want ETH", sess)
	}
	wantTuesday := ctTime(t, 2025, time.September, 23, 0, 0, 0)
	if !trd.Equal(wantTuesday) {
		t.Fatalf("tradingDate = %v, want %v", trd, wantTuesday)
	}
	if trd.Equal(civilMidnight(evening)) {
		t.Fatal("trading date should differ from calendar date for an evening trade")
	}
}

// Test 9 — maintenance break and weekend gaps classify as Closed, with no
// trading date, rather than being attributed to either neighboring date.
func TestClassify_ClosedGaps(t *testing.T) {
	cal := Calendar{Location: mustChicago(t), Schedule: ESRegularSchedule()}

	trd, sess := cal.Classify(ctTime(t, 2025, time.September, 23, 16, 30, 0)) // daily break
	if sess != Closed {
		t.Errorf("maintenance break session = %v, want Closed", sess)
	}
	if !trd.IsZero() {
		t.Errorf("maintenance break tradingDate = %v, want zero value", trd)
	}

	if _, sess := cal.Classify(ctTime(t, 2025, time.September, 27, 12, 0, 0)); sess != Closed { // Saturday
		t.Errorf("weekend session = %v, want Closed", sess)
	}
}

// Test 10 — special schedule overrides the regular weekly template.
func TestHoursFor_SpecialScheduleOverridesRegularDay(t *testing.T) {
	cal := Calendar{Location: mustChicago(t), Schedule: ESRegularSchedule()}

	regularThursday := ctTime(t, 2025, time.September, 25, 13, 0, 0)
	if _, sess := cal.Classify(regularThursday); sess != RTH {
		t.Errorf("regular Thursday 13:00 session = %v, want RTH", sess)
	}

	// Thanksgiving (Thursday, override closes RTH at 12:15): 13:00 CT is
	// now Closed, even though it's the same weekday and clock time.
	thanksgiving := ctTime(t, 2025, time.November, 27, 13, 0, 0)
	if _, sess := cal.Classify(thanksgiving); sess != Closed {
		t.Errorf("Thanksgiving 13:00 session = %v, want Closed", sess)
	}

	// Christmas: closed all day regardless of clock time.
	christmasMorning := ctTime(t, 2025, time.December, 25, 10, 0, 0)
	if _, sess := cal.Classify(christmasMorning); sess != Closed {
		t.Errorf("Christmas 10:00 session = %v, want Closed", sess)
	}
}

// Test 11 — DST: America/Chicago session calculations stay correct across
// the daylight-saving transitions, using only time.LoadLocation/time.Date
// (no manual DST rule in this codebase).
func TestClassify_DaylightSavingTransitions(t *testing.T) {
	loc := mustChicago(t)
	cal := Calendar{Location: loc, Schedule: ESRegularSchedule()}

	winterOpen := time.Date(2025, time.January, 15, 8, 30, 0, 0, loc) // Wed, CST
	if got := winterOpen.UTC().Hour(); got != 14 {
		t.Fatalf("winter 08:30 CT should be 14:00 UTC (CST=UTC-6), got %d:00 UTC", got)
	}
	if _, sess := cal.Classify(winterOpen); sess != RTH {
		t.Errorf("winter session = %v, want RTH", sess)
	}

	summerOpen := time.Date(2025, time.July, 15, 8, 30, 0, 0, loc) // Tue, CDT
	if got := summerOpen.UTC().Hour(); got != 13 {
		t.Fatalf("summer 08:30 CT should be 13:00 UTC (CDT=UTC-5), got %d:00 UTC", got)
	}
	if _, sess := cal.Classify(summerOpen); sess != RTH {
		t.Errorf("summer session = %v, want RTH", sess)
	}

	// The transition itself always falls on a Sunday (RTH doesn't apply);
	// check the very next trading dates on each side.
	afterSpringForward := time.Date(2025, time.March, 10, 8, 30, 0, 0, loc) // Mon, first day in CDT
	if _, sess := cal.Classify(afterSpringForward); sess != RTH {
		t.Errorf("day after spring-forward session = %v, want RTH", sess)
	}
	afterFallBack := time.Date(2025, time.November, 3, 8, 30, 0, 0, loc) // Mon, first day in CST
	if _, sess := cal.Classify(afterFallBack); sess != RTH {
		t.Errorf("day after fall-back session = %v, want RTH", sess)
	}

	// Trading date Monday's ETH session opens Sunday 17:00 CT, already
	// past the 2am spring-forward jump, so it must resolve as a normal
	// CDT instant, not an ambiguous or skipped local time.
	sundayEveningOpen := time.Date(2025, time.March, 9, 17, 0, 0, 0, loc)
	trd, sess := cal.Classify(sundayEveningOpen)
	if sess != ETH {
		t.Fatalf("Sunday 17:00 CT session = %v, want ETH", sess)
	}
	wantMonday := time.Date(2025, time.March, 10, 0, 0, 0, 0, loc)
	if !trd.Equal(wantMonday) {
		t.Fatalf("tradingDate = %v, want %v", trd, wantMonday)
	}
}

// NQ compatibility: a different product just supplies a different
// ProductSchedule value; Calendar's logic is unchanged.
func TestNQRegularSchedule_SharesESHours(t *testing.T) {
	cal := Calendar{Location: mustChicago(t), Schedule: NQRegularSchedule()}

	trd, sess := cal.Classify(ctTime(t, 2025, time.September, 23, 10, 0, 0))
	if sess != RTH {
		t.Fatalf("session = %v, want RTH", sess)
	}
	if want := ctTime(t, 2025, time.September, 23, 0, 0, 0); !trd.Equal(want) {
		t.Fatalf("tradingDate = %v, want %v", trd, want)
	}
}
