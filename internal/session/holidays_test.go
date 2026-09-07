package session

import (
	"encoding/csv"
	"errors"
	"io"
	"os"
	"testing"
	"time"
)

func TestClassify_ChristmasEveEarlyClose(t *testing.T) {
	cal := esCal(t)

	if _, sess := cal.Classify(ctTime(t, 2025, time.December, 24, 10, 0, 0)); sess != RTH {
		t.Errorf("Christmas Eve 10:00 session = %v, want RTH", sess)
	}
	if _, sess := cal.Classify(ctTime(t, 2025, time.December, 24, 12, 14, 59)); sess != RTH {
		t.Errorf("Christmas Eve 12:14:59 session = %v, want RTH", sess)
	}
	if _, sess := cal.Classify(ctTime(t, 2025, time.December, 24, 12, 15, 0)); sess != Closed {
		t.Errorf("Christmas Eve 12:15 session = %v, want Closed", sess)
	}
	if _, sess := cal.Classify(ctTime(t, 2025, time.December, 24, 20, 0, 0)); sess != Closed {
		t.Errorf("Christmas Eve 20:00 session = %v, want Closed (next day is Christmas)", sess)
	}
}

func TestClassify_LaborDayMondayHoliday(t *testing.T) {
	cal := esCal(t)
	// Sunday evening before a Monday holiday must not open ETH.
	if _, sess := cal.Classify(ctTime(t, 2025, time.August, 31, 20, 0, 0)); sess != Closed {
		t.Errorf("Sunday before Labor Day 20:00 session = %v, want Closed", sess)
	}
	if _, sess := cal.Classify(ctTime(t, 2025, time.September, 1, 10, 0, 0)); sess != Closed {
		t.Errorf("Labor Day 10:00 session = %v, want Closed", sess)
	}
	// Monday 17:00 opens Tuesday's ETH.
	trd, sess := cal.Classify(ctTime(t, 2025, time.September, 1, 17, 0, 0))
	if sess != ETH {
		t.Fatalf("Labor Day 17:00 session = %v, want ETH", sess)
	}
	if want := ctTime(t, 2025, time.September, 2, 0, 0, 0); !trd.Equal(want) {
		t.Errorf("Labor Day 17:00 tradingDate = %v, want %v", trd, want)
	}
}

func TestClassify_SeptemberMonthEndHalt(t *testing.T) {
	cal := esCal(t)
	if _, sess := cal.Classify(ctTime(t, 2025, time.September, 30, 15, 20, 0)); sess != Closed {
		t.Errorf("Sep 30 15:20 (halt) session = %v, want Closed", sess)
	}
	if _, sess := cal.Classify(ctTime(t, 2025, time.September, 30, 15, 45, 0)); sess != RTH {
		t.Errorf("Sep 30 15:45 session = %v, want RTH", sess)
	}
}

func TestClassify_NovemberEarlyCloseBeatsMonthEndHalt(t *testing.T) {
	cal := esCal(t)
	// Nov 28 2025 is both the Friday after Thanksgiving (12:15 close)
	// and November's last weekday. Early close wins: 15:20 is Closed
	// because the day already ended, not because of a 15:15 halt, and
	// there is no RTH resume at 15:30.
	if _, sess := cal.Classify(ctTime(t, 2025, time.November, 28, 15, 20, 0)); sess != Closed {
		t.Errorf("Nov 28 15:20 session = %v, want Closed", sess)
	}
	if _, sess := cal.Classify(ctTime(t, 2025, time.November, 28, 15, 45, 0)); sess != Closed {
		t.Errorf("Nov 28 15:45 session = %v, want Closed (no month-end RTH resume)", sess)
	}
}

// TestClosedHoursHaveNoTradesOnRealData is the acceptance criterion:
// every trade's TsEvent in the real file must Classify as RTH or ETH.
// A single print in Closed means the holiday table or weekly template
// is wrong. Set MDL_DATA to the path of the Databento CSV.
func TestClosedHoursHaveNoTradesOnRealData(t *testing.T) {
	path := os.Getenv("MDL_DATA")
	if path == "" {
		t.Skip("set MDL_DATA to run integration tests")
	}

	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	cr := csv.NewReader(f)
	cr.ReuseRecord = true
	header, err := cr.Read()
	if err != nil {
		t.Fatal(err)
	}
	col := make(map[string]int, len(header))
	for i, name := range header {
		col[name] = i
	}
	for _, name := range []string{"ts_event", "action"} {
		if _, ok := col[name]; !ok {
			t.Fatalf("missing column %q", name)
		}
	}

	cal, err := ForProduct("ES")
	if err != nil {
		t.Fatal(err)
	}

	var closed int
	var first string
	for {
		row, err := cr.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if row[col["action"]] != "T" {
			continue
		}
		ts, err := time.Parse(time.RFC3339Nano, row[col["ts_event"]])
		if err != nil {
			t.Fatal(err)
		}
		if _, sess := cal.Classify(ts); sess == Closed {
			closed++
			if first == "" {
				first = ts.In(cal.Location).Format(time.RFC3339Nano)
			}
		}
	}
	if closed > 0 {
		t.Errorf("%d trades have ts_event in Closed (first %s); the calendar is wrong or the holiday table is incomplete",
			closed, first)
	}
}
