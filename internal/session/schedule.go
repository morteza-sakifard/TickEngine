package session

import "time"

type Clock struct {
	Hour, Min int
}

type DayTemplate struct {
	Closed            bool
	EthOpenDayOffset  int
	RTHOpen, RTHClose Clock
	ETHOpen, ETHClose Clock
}

type CivilDate struct {
	Year  int
	Month time.Month
	Day   int
}

func NewCivilDate(year int, Month time.Month, day int) CivilDate {
	return CivilDate{Year: year, Month: Month, Day: day}
}

func civilDateOf(t time.Time) CivilDate {
	y, m, d := t.Date()
	return CivilDate{Year: y, Month: m, Day: d}
}

type ProductSchedule struct {
	Weekly    [7]DayTemplate
	Overrides map[CivilDate]DayTemplate
}

func (s ProductSchedule) HoursFor(tradingDate time.Time) Hours {
	tmpl, ok := s.Overrides[civilDateOf(tradingDate)]
	if !ok {
		tmpl = s.Weekly[tradingDate.Weekday()]
	}
	if tmpl.Closed {
		return Hours{Closed: true}
	}

	loc := tradingDate.Location()
	at := func(date time.Time, c Clock) time.Time {
		y, m, d := date.Date()
		return time.Date(y, m, d, c.Hour, c.Min, 0, 0, loc)
	}
	return Hours{
		ETHOpen:  at(tradingDate.AddDate(0, 0, tmpl.EthOpenDayOffset), tmpl.ETHOpen),
		ETHClose: at(tradingDate, tmpl.ETHClose),
		RTHOpen:  at(tradingDate, tmpl.RTHOpen),
		RTHClose: at(tradingDate, tmpl.RTHClose),
	}

}
