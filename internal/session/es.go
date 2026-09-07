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
		Weekly:    weekly,
		Overrides: equityIndexOverrides(),
	}
}

func NQRegularSchedule() ProductSchedule {
	// NQ trades the same CME Equity Index hours as ES, including the
	// same holidays, early closes, and month-end halt. A later
	// product-specific exception goes in NQ's own Overrides map, not
	// by forking the weekly template.
	es := ESRegularSchedule()
	return ProductSchedule{Weekly: es.Weekly, Overrides: es.Overrides}
}
