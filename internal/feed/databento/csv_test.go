package databento

import (
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/morteza-sakifard/market-data-lab/internal/core"
	"github.com/morteza-sakifard/market-data-lab/internal/marketdata"
)

// -update regenerates testdata/golden/mbp1_sample.events.txt from the
// decoder's real output instead of comparing against it. Run once with
// -update, read the file with your own eyes, commit it, then every
// future run without -update proves the decoder still agrees with it.
var updateGolden = flag.Bool("update", false, "write the golden file instead of comparing to it")

const (
	fixturePath = "../../../testdata/mbp1_sample.csv"
	goldenPath  = "../../../testdata/golden/mbp1_sample.events.txt"
)

func openFixture(t *testing.T) *os.File {
	t.Helper()
	f, err := os.Open(fixturePath)
	if err != nil {
		t.Fatal(err)
	}
	return f
}

// formatEvent renders one Event deterministically, reusing
// Kind/Flags/Side.String() from step 2 and 3 instead of a bespoke
// format that would just duplicate them.
func formatEvent(e marketdata.Event) string {
	var b strings.Builder
	fmt.Fprintf(&b, "kind=%s instrument=%d ts_event=%d ts_recv=%d ts_in_delta=%d sequence=%d flags=%s",
		e.Kind, e.Instrument, e.TsEvent, e.TsRecv, e.TsInDelta, e.Sequence, e.Flags)
	switch e.Kind {
	case marketdata.KindTrade:
		fmt.Fprintf(&b, " trade{px=%d qty=%d aggressor=%s}", e.Trade.Px, e.Trade.Qty, e.Trade.Aggressor)
	case marketdata.KindQuote:
		fmt.Fprintf(&b, " quote{bid_px=%d bid_qty=%d bid_ct=%d ask_px=%d ask_qty=%d ask_ct=%d}",
			e.Quote.BidPx, e.Quote.BidQty, e.Quote.BidCt, e.Quote.AskPx, e.Quote.AskQty, e.Quote.AskCt)
	}
	return b.String()
}

func decodeAll(t *testing.T) []marketdata.Event {
	t.Helper()
	d, err := NewDecoder(openFixture(t), core.ESZ5())
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	var events []marketdata.Event
	var evt marketdata.Event
	for {
		err := d.Next(&evt)
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		events = append(events, evt)
	}
	return events
}

func TestDecodeGolden(t *testing.T) {
	events := decodeAll(t)

	var got strings.Builder
	for i, e := range events {
		fmt.Fprintf(&got, "%d %s\n", i, formatEvent(e))
	}

	if *updateGolden {
		if err := os.MkdirAll(filepath.Dir(goldenPath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(goldenPath, []byte(got.String()), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Logf("wrote %s (%d events) — rerun without -update to verify", goldenPath, len(events))
		return
	}

	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("%v (first run: go test ./internal/feed/databento -run TestDecodeGolden -update)", err)
	}
	if got.String() != string(want) {
		t.Errorf("decoded events do not match %s; if this change is expected, rerun with -update\n got:\n%s\nwant:\n%s",
			goldenPath, got.String(), string(want))
	}
}

// TestDecodeRowToEventMapping checks the acceptance criterion directly
// (action=T rows split into Trade-then-Quote, everything else is
// Quote-only) instead of relying only on a diff against the golden
// blob to say what broke.
func TestDecodeRowToEventMapping(t *testing.T) {
	events := decodeAll(t)

	wantKinds := []marketdata.Kind{
		marketdata.KindQuote, marketdata.KindQuote, // rows 1, 2 (A)
		marketdata.KindTrade, marketdata.KindQuote, // row 3 (T)
		marketdata.KindTrade, marketdata.KindQuote, // row 4 (T)
		marketdata.KindQuote,                       // row 5 (C)
		marketdata.KindQuote,                       // row 6 (A)
		marketdata.KindTrade, marketdata.KindQuote, // row 7 (T)
		marketdata.KindQuote, // row 8 (A)
	}
	if len(events) != len(wantKinds) {
		t.Fatalf("got %d events, want %d (8 rows, 3 of them action=T contributing 2 each)",
			len(events), len(wantKinds))
	}
	for i, want := range wantKinds {
		if events[i].Kind != want {
			t.Errorf("events[%d].Kind = %s, want %s", i, events[i].Kind, want)
		}
	}

	// Every Trade must be immediately followed by the Quote it implies,
	// sharing the same envelope: they come from one CSV row.
	for i, e := range events {
		if e.Kind != marketdata.KindTrade {
			continue
		}
		q := events[i+1]
		if q.Kind != marketdata.KindQuote || q.Sequence != e.Sequence || q.TsEvent != e.TsEvent {
			t.Errorf("events[%d] is a Trade not immediately followed by its Quote with a matching envelope", i)
		}
	}
}

func TestDecodeEdgeCases(t *testing.T) {
	events := decodeAll(t)

	t.Run("crossed book passes through unmodified", func(t *testing.T) {
		// Row 6 -> events[7]: bid 6715.25 > ask 6715.00. Counting a
		// crossed book is step 1's census's job; this decoder's job is
		// only to carry it faithfully, not to reject or "fix" it.
		q := events[7].Quote
		if q.BidPx <= q.AskPx {
			t.Fatalf("events[7].Quote = %+v, want a crossed book (BidPx > AskPx)", q)
		}
	})

	t.Run("UNDEF ask price survives as a Ticks-level sentinel", func(t *testing.T) {
		// Row 8 -> events[10]: ask_px_00 is Databento's UNDEF_PRICE.
		q := events[10].Quote
		if q.AskPx != core.Ticks(math.MaxInt64) || q.AskQty != 0 {
			t.Errorf("events[10].Quote = %+v, want AskPx = math.MaxInt64, AskQty = 0", q)
		}
	})

	t.Run("side N trade has no aggressor", func(t *testing.T) {
		// Row 7 (T, side=N) -> events[8].
		if got := events[8].Trade.Aggressor; got != core.SideNone {
			t.Errorf("events[8].Trade.Aggressor = %s, want none", got)
		}
	})
}

func TestNewDecoderRejectsWrongHeader(t *testing.T) {
	_, err := NewDecoder(io.NopCloser(strings.NewReader("not,the,right,header\n")), core.ESZ5())
	if err == nil {
		t.Fatal("expected an error for a mismatched header")
	}
}

func TestNextRejectsInstrumentMismatch(t *testing.T) {
	wrong := core.ESZ5()
	wrong.ID++ // anything other than the fixture's real instrument_id

	d, err := NewDecoder(openFixture(t), wrong)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	var evt marketdata.Event
	if err := d.Next(&evt); err == nil {
		t.Fatal("expected an error when a row's instrument_id does not match the configured Instrument")
	}
}

// TestNextAllocs is the acceptance criterion itself: Next must perform
// zero allocations per call. See the "leaking parameter" decision in
// docs/steps/04-feed-databento.reference.md for the Go-level mechanism
// this guards against.
func TestNextAllocs(t *testing.T) {
	d, err := NewDecoder(openFixture(t), core.ESZ5())
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	var evt marketdata.Event
	f := func() {
		if err := d.Next(&evt); err != nil {
			t.Fatal(err)
		}
	}

	// The 8-row fixture yields exactly 11 events (TestDecodeRowToEventMapping).
	// 1 warm-up call plus 10 measured calls stays inside that budget and
	// still exercises both the fresh-row-decode path and the buffered
	// pending-Quote path, without ever hitting io.EOF mid-measurement.
	if allocs := testing.AllocsPerRun(10, f); allocs > 0 {
		t.Errorf("Next allocated %v times per call, want 0", allocs)
	}
}
