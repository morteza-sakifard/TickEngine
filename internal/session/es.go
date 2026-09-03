package session

import "time"

var weekDays = []time.Weekday{time.Monday, time.Tuesday, time.Wednesday, time.Thursday, time.Friday}

func ESRegularSchedule() ProductSchedule {
	regular := DayTemplate{
		EthOpenDayOffset: -1,
		ETHOpen:          Clock{17, 0},
		ETHClose:         Clock{16, 0},
		RTHOpen:          Clock{8, 30},
		// 08:30-15:00 CT = 09:30-16:00 ET, aligned with the NYSE/Nasdaq
		// cash close. CME eliminated the daily 15:15-15:30 CT Globex
		// trading halt for equity index futures/options on 2021-06-28
		// (CFTC Submission No. 21-244R), so ES/NQ now trade continuously
		// from RTH close straight through to the 16:00-17:00 CT
		// maintenance break below. See ESRegularSchedule's Overrides for
		// the once-a-month exception where the halt still applies.
		RTHClose: Clock{15, 0},
	}

	var weekly [7]DayTemplate
	for _, d := range weekDays {
		weekly[d] = regular
	}

	weekly[time.Saturday] = DayTemplate{Closed: true}
	weekly[time.Sunday] = DayTemplate{Closed: true}

	return ProductSchedule{
		Weekly: weekly,
		Overrides: map[CivilDate]DayTemplate{
			// Christmas Day: exchange fully closed.
			NewCivilDate(2025, time.December, 25): {Closed: true},
			// Thanksgiving Day: exchange fully closed, same as Christmas.
			NewCivilDate(2025, time.November, 27): {Closed: true},
			// Day after Thanksgiving: early close, no afternoon RTH or evening ETH.
			NewCivilDate(2025, time.November, 28): {
				EthOpenDayOffset: -1,
				ETHOpen:          Clock{17, 0},
				ETHClose:         Clock{12, 15},
				RTHOpen:          Clock{8, 30},
				RTHClose:         Clock{12, 15},
			},
			// Last trading day of December (month/quarter/year-end index
			// fixing): CME extends RTH to 16:00 CT with an embedded
			// 15:15-15:30 CT halt, instead of the normal continuous
			// 15:00-16:00 CT post-RTH ETH. See cmegroup.com's End-of-Month
			// Settlement Procedures FAQ. Add the remaining 11 months'
			// last-trading-day dates here the same way if needed.
			NewCivilDate(2025, time.December, 31): {
				EthOpenDayOffset: -1,
				ETHOpen:          Clock{17, 0},
				ETHClose:         Clock{16, 0},
				RTHOpen:          Clock{8, 30},
				RTHClose:         Clock{16, 0},
				HaltOpen:         Clock{15, 15},
				HaltClose:        Clock{15, 30},
			},
		},
	}
}

func NQRegularSchedule() ProductSchedule {
	return ProductSchedule{
		Weekly:    ESRegularSchedule().Weekly,
		Overrides: map[CivilDate]DayTemplate{
			// NQ-specific holiday/early-close overrides go here.
		},
	}
}
