package session

import (
	"testing"
	"time"
)

func esCal(t *testing.T) Calendar {
	t.Helper()
	return Calendar{Location: mustChicago(t), Schedule: ESRegularSchedule()}
}

func TestBoundariesRegularWeekday(t *testing.T) {
	cal := esCal(t)
	// Tuesday 2025-09-23, from midnight to the next midnight.
	from := ctTime(t, 2025, time.September, 23, 0, 0, 0)
	to := ctTime(t, 2025, time.September, 24, 0, 0, 0)
	got := cal.Boundaries(from, to)

	tue := ctTime(t, 2025, time.September, 23, 0, 0, 0)
	wed := ctTime(t, 2025, time.September, 24, 0, 0, 0)
	want := []Boundary{
		{At: ctTime(t, 2025, time.September, 23, 8, 30, 0), TradingDate: tue, From: ETH, To: RTH},
		{At: ctTime(t, 2025, time.September, 23, 15, 0, 0), TradingDate: tue, From: RTH, To: ETH},
		{At: ctTime(t, 2025, time.September, 23, 16, 0, 0), TradingDate: time.Time{}, From: ETH, To: Closed},
		{At: ctTime(t, 2025, time.September, 23, 17, 0, 0), TradingDate: wed, From: Closed, To: ETH},
	}
	assertBoundaries(t, got, want)
}

func TestBoundariesHalfOpenRange(t *testing.T) {
	cal := esCal(t)
	rthOpen := ctTime(t, 2025, time.September, 23, 8, 30, 0)
	rthClose := ctTime(t, 2025, time.September, 23, 15, 0, 0)

	got := cal.Boundaries(rthOpen, rthClose)
	if len(got) != 1 {
		t.Fatalf("Boundaries([08:30, 15:00)) = %d entries, want 1 (the 08:30 edge inclusive, 15:00 exclusive)", len(got))
	}
	if !got[0].At.Equal(rthOpen) || got[0].To != RTH {
		t.Errorf("got %+v, want the 08:30 ETH->RTH edge", got[0])
	}
}

func TestBoundariesEmptyOrInverted(t *testing.T) {
	cal := esCal(t)
	at := ctTime(t, 2025, time.September, 23, 10, 0, 0)
	if got := cal.Boundaries(at, at); got != nil {
		t.Errorf("empty range = %v, want nil", got)
	}
	if got := cal.Boundaries(at.Add(time.Hour), at); got != nil {
		t.Errorf("inverted range = %v, want nil", got)
	}
}

func TestBoundariesNoDuplicatesAndSorted(t *testing.T) {
	cal := esCal(t)
	from := ctTime(t, 2025, time.September, 22, 0, 0, 0)
	to := ctTime(t, 2025, time.September, 29, 0, 0, 0)
	got := cal.Boundaries(from, to)
	if len(got) == 0 {
		t.Fatal("expected boundaries across a business week")
	}
	for i := 1; i < len(got); i++ {
		if !got[i-1].At.Before(got[i].At) {
			t.Errorf("Boundaries not strictly increasing at %d: %v then %v", i, got[i-1].At, got[i].At)
		}
	}
}

func TestBoundariesMonthEndHalt(t *testing.T) {
	cal := esCal(t)
	from := ctTime(t, 2025, time.December, 31, 15, 0, 0)
	to := ctTime(t, 2026, time.January, 1, 0, 0, 0)
	got := cal.Boundaries(from, to)

	dec31 := ctTime(t, 2025, time.December, 31, 0, 0, 0)
	// Jan 1 2026 is New Year's (closed), so Dec 31 17:00 does not
	// open ETH. The next edge is Jan 1 17:00, which is outside [from, to).
	want := []Boundary{
		{At: ctTime(t, 2025, time.December, 31, 15, 15, 0), TradingDate: time.Time{}, From: RTH, To: Closed},
		{At: ctTime(t, 2025, time.December, 31, 15, 30, 0), TradingDate: dec31, From: Closed, To: RTH},
		{At: ctTime(t, 2025, time.December, 31, 16, 0, 0), TradingDate: time.Time{}, From: RTH, To: Closed},
	}
	assertBoundaries(t, got, want)
}

func TestBoundariesChristmasEve(t *testing.T) {
	cal := esCal(t)
	from := ctTime(t, 2025, time.December, 24, 0, 0, 0)
	to := ctTime(t, 2025, time.December, 26, 0, 0, 0)
	got := cal.Boundaries(from, to)

	dec24 := ctTime(t, 2025, time.December, 24, 0, 0, 0)
	dec26 := ctTime(t, 2025, time.December, 26, 0, 0, 0)
	want := []Boundary{
		{At: ctTime(t, 2025, time.December, 24, 8, 30, 0), TradingDate: dec24, From: ETH, To: RTH},
		{At: ctTime(t, 2025, time.December, 24, 12, 15, 0), TradingDate: time.Time{}, From: RTH, To: Closed},
		// Dec 25 is Christmas (fully closed). ETH for Friday Dec 26
		// opens Thursday Dec 25 17:00 CT.
		{At: ctTime(t, 2025, time.December, 25, 17, 0, 0), TradingDate: dec26, From: Closed, To: ETH},
	}
	assertBoundaries(t, got, want)
}

func TestBoundariesDSTSpringForward(t *testing.T) {
	cal := esCal(t)
	// Sunday 2025-03-09 02:00 CT is skipped. ETH for Monday opens
	// Sunday 17:00, already in CDT.
	from := ctTime(t, 2025, time.March, 9, 16, 0, 0)
	to := ctTime(t, 2025, time.March, 10, 9, 0, 0)
	got := cal.Boundaries(from, to)

	mon := ctTime(t, 2025, time.March, 10, 0, 0, 0)
	want := []Boundary{
		{At: ctTime(t, 2025, time.March, 9, 17, 0, 0), TradingDate: mon, From: Closed, To: ETH},
		{At: ctTime(t, 2025, time.March, 10, 8, 30, 0), TradingDate: mon, From: ETH, To: RTH},
	}
	assertBoundaries(t, got, want)
}

func TestBoundariesDSTFallBack(t *testing.T) {
	cal := esCal(t)
	from := ctTime(t, 2025, time.November, 2, 16, 0, 0)
	to := ctTime(t, 2025, time.November, 3, 9, 0, 0)
	got := cal.Boundaries(from, to)

	mon := ctTime(t, 2025, time.November, 3, 0, 0, 0)
	want := []Boundary{
		{At: ctTime(t, 2025, time.November, 2, 17, 0, 0), TradingDate: mon, From: Closed, To: ETH},
		{At: ctTime(t, 2025, time.November, 3, 8, 30, 0), TradingDate: mon, From: ETH, To: RTH},
	}
	assertBoundaries(t, got, want)
}

// TestBoundariesMatchClassifyWalk checks the property Boundaries claims:
// every session change Classify would report in the range, and no extras.
func TestBoundariesMatchClassifyWalk(t *testing.T) {
	cal := esCal(t)
	from := ctTime(t, 2025, time.December, 23, 0, 0, 0)
	to := ctTime(t, 2026, time.January, 3, 0, 0, 0)

	var walked []Boundary
	prevTD, prevSess := cal.Classify(from)
	for t0 := from.Add(time.Minute); t0.Before(to); t0 = t0.Add(time.Minute) {
		td, sess := cal.Classify(t0)
		if sess == prevSess && td.Equal(prevTD) {
			continue
		}
		walked = append(walked, Boundary{At: t0, TradingDate: td, From: prevSess, To: sess})
		prevTD, prevSess = td, sess
	}

	got := cal.Boundaries(from, to)
	if len(got) != len(walked) {
		t.Fatalf("Boundaries returned %d edges, minute walk found %d", len(got), len(walked))
	}
	for i := range got {
		// The walk reports the first minute *inside* the new session;
		// Boundaries reports the exact cut. They must name the same
		// From/To and TradingDate, and the walk instant must be at or
		// after the cut, less than one minute later.
		if got[i].From != walked[i].From || got[i].To != walked[i].To || !got[i].TradingDate.Equal(walked[i].TradingDate) {
			t.Errorf("edge %d: Boundaries %+v vs walk %+v", i, got[i], walked[i])
		}
		if walked[i].At.Before(got[i].At) || !walked[i].At.Before(got[i].At.Add(time.Minute)) {
			t.Errorf("edge %d: walk At %v is not in [%v, %v)", i, walked[i].At, got[i].At, got[i].At.Add(time.Minute))
		}
	}
}

func assertBoundaries(t *testing.T, got, want []Boundary) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("got %d boundaries, want %d\n got: %v\nwant: %v", len(got), len(want), got, want)
	}
	for i := range want {
		g, w := got[i], want[i]
		if !g.At.Equal(w.At) || g.From != w.From || g.To != w.To || !g.TradingDate.Equal(w.TradingDate) {
			t.Errorf("boundaries[%d] = {At:%v From:%s To:%s TD:%v}, want {At:%v From:%s To:%s TD:%v}",
				i, g.At, g.From, g.To, g.TradingDate, w.At, w.From, w.To, w.TradingDate)
		}
	}
}
