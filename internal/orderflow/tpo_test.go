package orderflow

import (
	"testing"
	"time"

	"github.com/morteza-sakifard/market-data-lab/internal/core"
	"github.com/morteza-sakifard/market-data-lab/internal/session"
)

func addTPO(tpo *TPO, px int64, at time.Time) {
	e := tr(px, 1, core.SideBid, at)
	tpo.OnTrade(&e)
}

func TestPeriodLetter(t *testing.T) {
	if PeriodLetter(0) != "A" || PeriodLetter(25) != "Z" || PeriodLetter(26) != "a" {
		t.Fatalf("letters A/Z/a = %q %q %q", PeriodLetter(0), PeriodLetter(25), PeriodLetter(26))
	}
}

func TestTPOEmptyAndZeroAnchor(t *testing.T) {
	var z TPO
	e := tr(26800, 1, core.SideBid, time.Date(2025, 9, 23, 13, 30, 0, 0, time.UTC))
	z.OnTrade(&e)
	if len(z.Levels()) != 0 {
		t.Fatal("zero anchor must not start period A from the first trade")
	}
	tpo := NewTPO(time.Time{})
	tpo.OnTrade(&e)
	if len(tpo.Levels()) != 0 {
		t.Fatal("NewTPO(zero) must ignore trades")
	}
}

func TestTPOAnchorNotFirstTrade(t *testing.T) {
	cal, err := session.ForProduct("ES")
	if err != nil {
		t.Fatal(err)
	}
	day := time.Date(2025, time.September, 23, 0, 0, 0, 0, cal.Location)
	open := cal.Schedule.HoursFor(day).RTHOpen
	tpo := NewTPO(open)
	late := open.Add(2*time.Hour + 7*time.Minute) // 10:37
	addTPO(tpo, 26800, late)
	got := tpo.Levels()
	if len(got) != 1 || len(got[0].Periods) != 1 || got[0].Periods[0] != 4 {
		t.Fatalf("10:37 vs 08:30 open = %+v, want period 4 (E), not 0 (A)", got)
	}
	if PeriodLetter(got[0].Periods[0]) != "E" {
		t.Fatalf("letter = %q, want E", PeriodLetter(got[0].Periods[0]))
	}
}

func TestTPOFullRTHHas13Periods(t *testing.T) {
	cal, err := session.ForProduct("ES")
	if err != nil {
		t.Fatal(err)
	}
	day := time.Date(2025, time.September, 23, 0, 0, 0, 0, cal.Location)
	h := cal.Schedule.HoursFor(day)
	tpo := NewTPO(h.RTHOpen)
	for i := 0; i < 13; i++ {
		at := h.RTHOpen.Add(time.Duration(i)*TPOPeriod + time.Second)
		if !at.Before(h.RTHClose) {
			t.Fatalf("period %d (%s) is not inside RTH", i, at)
		}
		addTPO(tpo, 26800+int64(i%3), at)
	}
	if got := tpo.PeriodCount(); got != 13 {
		t.Fatalf("PeriodCount = %d, want 13 (08:30–15:00)", got)
	}
	if PeriodLetter(12) != "M" {
		t.Fatalf("period 12 letter = %q, want M", PeriodLetter(12))
	}
}

func TestTPOInitialBalanceAndSinglePrints(t *testing.T) {
	open := time.Date(2025, 9, 23, 13, 30, 0, 0, time.UTC) // 08:30 CT
	tpo := NewTPO(open)
	addTPO(tpo, 100, open)                     // A
	addTPO(tpo, 102, open)                     // A
	addTPO(tpo, 99, open.Add(30*time.Minute))  // B
	addTPO(tpo, 102, open.Add(30*time.Minute)) // B
	addTPO(tpo, 110, open.Add(60*time.Minute)) // C — single, outside IB
	low, high, ok := tpo.InitialBalance()
	if !ok || low != 99 || high != 102 {
		t.Fatalf("IB = %d,%d ok=%v, want 99–102", low, high, ok)
	}
	sp := tpo.SinglePrints()
	if len(sp) != 3 { // 99 B, 100 A, 110 C; 102 is A+B
		t.Fatalf("SinglePrints = %+v, want 3 rows", sp)
	}
	if tpo.POC() != 102 {
		t.Fatalf("POC = %d, want 102 (two periods)", tpo.POC())
	}
}

func TestTPOResetKeepsAnchor(t *testing.T) {
	open := time.Date(2025, 9, 23, 13, 30, 0, 0, time.UTC)
	tpo := NewTPO(open)
	addTPO(tpo, 26800, open.Add(2*time.Hour+7*time.Minute))
	tpo.Reset()
	if tpo.PeriodCount() != 0 {
		t.Fatal("Reset did not clear prints")
	}
	addTPO(tpo, 26800, open.Add(2*time.Hour+7*time.Minute))
	if tpo.Levels()[0].Periods[0] != 4 {
		t.Fatal("Reset lost the session-open anchor")
	}
}

func TestTPOIgnoresBeforeOpen(t *testing.T) {
	open := time.Date(2025, 9, 23, 13, 30, 0, 0, time.UTC)
	tpo := NewTPO(open)
	addTPO(tpo, 26800, open.Add(-time.Minute))
	if tpo.PeriodCount() != 0 {
		t.Fatal("trade before the open must not create period −1")
	}
}
