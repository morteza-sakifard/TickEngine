package session

import (
	"time"

	"github.com/morteza-sakifard/TickEngine/internal/core"
)

// closedDay is a full exchange holiday: no ETH, no RTH, no evening
// reopen that calendar day. HoursFor returns Hours{Closed: true}, and
// Classify attributes nothing on that date to a trading session.
func closedDay() DayTemplate {
	return DayTemplate{Closed: true}
}

// earlyClose1215 is CME Equity Index's early-close template: ETH from
// the previous calendar day's 17:00 CT until 12:15 CT, RTH 08:30-12:15
// CT, and no 15:00-16:00 afternoon ETH. Used for the Friday after
// Thanksgiving and for Christmas Eve when that date is a weekday.
// Source: CME Equity Index futures holiday hours (12:15 CT close).
func earlyClose1215() DayTemplate {
	return DayTemplate{
		EthOpenDayOffset: -1,
		ETHOpen:          Clock{17, 0},
		ETHClose:         Clock{12, 15},
		RTHOpen:          Clock{8, 30},
		RTHClose:         Clock{12, 15},
	}
}

// monthEndSettlement is the last-trading-day-of-the-month index-fixing
// schedule: RTH extends to 16:00 CT with a 15:15-15:30 CT halt inside
// it, instead of the normal continuous 15:00-16:00 CT post-RTH ETH.
// Source: CME End-of-Month Settlement Procedures FAQ.
func monthEndSettlement() DayTemplate {
	return DayTemplate{
		EthOpenDayOffset: -1,
		ETHOpen:          Clock{17, 0},
		ETHClose:         Clock{16, 0},
		RTHOpen:          Clock{8, 30},
		RTHClose:         Clock{16, 0},
		HaltOpen:         Clock{15, 15},
		HaltClose:        Clock{15, 30},
	}
}

// equityIndexOverrides is the 2025 CME Equity Index (ES/NQ) holiday,
// early-close, and month-end table. The project's file starts on
// 2025-09-22 (docs/steps/01-data-census.md) and the roadmap says
// Christmas Eve 12:15 CT is inside the range, so the year the table
// covers is 2025. Dates before the file starts are still listed: they
// cost nothing, and Classify on those dates should not silently
// pretend they were regular weekdays.
//
// When a last-trading-day coincides with a holiday or early close
// (November 28 2025 is both the Friday after Thanksgiving and
// November's last weekday), the holiday/early-close template wins:
// there is no 15:15-15:30 halt on a day that is already closed or
// already done at 12:15.
func equityIndexOverrides() map[core.CivilDate]DayTemplate {
	m := make(map[core.CivilDate]DayTemplate, 32)

	for _, d := range []core.CivilDate{
		core.NewCivilDate(2025, time.January, 1),   // New Year's Day
		core.NewCivilDate(2025, time.January, 20),  // Martin Luther King Jr. Day
		core.NewCivilDate(2025, time.February, 17), // Presidents' Day
		core.NewCivilDate(2025, time.April, 18),    // Good Friday
		core.NewCivilDate(2025, time.May, 26),      // Memorial Day
		core.NewCivilDate(2025, time.June, 19),     // Juneteenth
		core.NewCivilDate(2025, time.July, 4),      // Independence Day
		core.NewCivilDate(2025, time.September, 1), // Labor Day
		core.NewCivilDate(2025, time.November, 27), // Thanksgiving Day
		core.NewCivilDate(2025, time.December, 25), // Christmas Day
		// New Year's Day 2026 is the morning after the file's last
		// month-end (Dec 31 2025). Without this override, Dec 31 17:00
		// would open a phantom ETH for Thursday Jan 1.
		core.NewCivilDate(2026, time.January, 1),
	} {
		m[d] = closedDay()
	}

	for _, d := range []core.CivilDate{
		core.NewCivilDate(2025, time.November, 28), // Friday after Thanksgiving
		core.NewCivilDate(2025, time.December, 24), // Christmas Eve
	} {
		m[d] = earlyClose1215()
	}

	for _, d := range []core.CivilDate{
		core.NewCivilDate(2025, time.January, 31),
		core.NewCivilDate(2025, time.February, 28),
		core.NewCivilDate(2025, time.March, 31),
		core.NewCivilDate(2025, time.April, 30),
		core.NewCivilDate(2025, time.May, 30), // May 31 is Saturday
		core.NewCivilDate(2025, time.June, 30),
		core.NewCivilDate(2025, time.July, 31),
		core.NewCivilDate(2025, time.August, 29), // August 31 is Sunday
		core.NewCivilDate(2025, time.September, 30),
		core.NewCivilDate(2025, time.October, 31),
		// November 28 is already earlyClose1215 above.
		core.NewCivilDate(2025, time.December, 31),
	} {
		if _, exists := m[d]; exists {
			continue
		}
		m[d] = monthEndSettlement()
	}
	return m
}
