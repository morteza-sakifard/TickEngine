package session

import (
	"time"

	"github.com/morteza-sakifard/TickEngine/internal/core"
)

type Clock struct {
	Hour, Min int
}

type DayTemplate struct {
	Closed              bool
	EthOpenDayOffset    int
	RTHOpen, RTHClose   Clock
	ETHOpen, ETHClose   Clock
	HaltOpen, HaltClose Clock // zero value (both unset) means no halt that day
}

type ProductSchedule struct {
	Weekly    [7]DayTemplate
	Overrides map[core.CivilDate]DayTemplate
}

func (s ProductSchedule) HoursFor(tradingDate time.Time) Hours {
	tmpl, ok := s.Overrides[core.CivilDateOf(tradingDate)]
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
	h := Hours{
		ETHOpen:  at(tradingDate.AddDate(0, 0, tmpl.EthOpenDayOffset), tmpl.ETHOpen),
		ETHClose: at(tradingDate, tmpl.ETHClose),
		RTHOpen:  at(tradingDate, tmpl.RTHOpen),
		RTHClose: at(tradingDate, tmpl.RTHClose),
	}
	if tmpl.HaltOpen != (Clock{}) || tmpl.HaltClose != (Clock{}) {
		h.HaltOpen = at(tradingDate, tmpl.HaltOpen)
		h.HaltClose = at(tradingDate, tmpl.HaltClose)
	}
	return h
}
