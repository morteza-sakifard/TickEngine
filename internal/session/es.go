package session

import "time"

var weekDays = []time.Weekday{time.Monday, time.Tuesday, time.Wednesday, time.Thursday, time.Friday}

func ESRegularSchedule() ProductSchedule {
	regular := DayTemplate{
		EthOpenDayOffset: -1,
		ETHOpen:          Clock{17, 0},
		ETHClose:         Clock{16, 0},
		RTHOpen:          Clock{8, 30},
		RTHClose:         Clock{15, 15},
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
			// Thanksgiving: early close, no afternoon RTH or evening ETH.
			NewCivilDate(2025, time.November, 27): {
				EthOpenDayOffset: -1,
				ETHOpen:          Clock{17, 0},
				ETHClose:         Clock{12, 15},
				RTHOpen:          Clock{8, 30},
				RTHClose:         Clock{12, 15},
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
