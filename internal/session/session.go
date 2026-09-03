package session

import "time"

type Session int

const (
	Closed Session = iota
	RTH
	ETH
)

func (s Session) String() string {
	switch s {
	case RTH:
		return "RTH"
	case ETH:
		return "ETH"
	default:
		return "Closed"
	}
}

type Hours struct {
	Closed              bool
	RTHOpen, RTHClose   time.Time
	ETHOpen, ETHClose   time.Time
	HaltOpen, HaltClose time.Time // zero value means no halt that day
}

type Schedule interface {
	HoursFor(tradingDate time.Time) Hours
}

type Calendar struct {
	Location *time.Location
	Schedule Schedule
}

func (c Calendar) Classify(t time.Time) (tradingDate time.Time, sess Session) {
	local := t.In(c.Location)
	today := civilMidnight(local)

	candidates := [2]time.Time{today, today.AddDate(0, 0, 1)}
	for _, candidate := range candidates {
		h := c.Schedule.HoursFor(candidate)
		if h.Closed {
			continue
		}
		if !h.HaltOpen.IsZero() && !local.Before(h.HaltOpen) && local.Before(h.HaltClose) {
			continue
		}
		if !local.Before(h.RTHOpen) && local.Before(h.RTHClose) {
			return candidate, RTH
		}
		if !local.Before(h.ETHOpen) && local.Before(h.ETHClose) {
			return candidate, ETH
		}
	}
	return time.Time{}, Closed
}

func (c Calendar) TradingDate(t time.Time) time.Time {
	tradingDate, _ := c.Classify(t)
	return tradingDate
}

func civilMidnight(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}
