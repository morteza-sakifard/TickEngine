package databento

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/morteza-sakifard/market-data-lab/internal/core"
	"github.com/morteza-sakifard/market-data-lab/internal/marketdata"
	"github.com/morteza-sakifard/market-data-lab/internal/orderbook"
)

func TestParseSchema(t *testing.T) {
	got, err := ParseSchema("")
	if err != nil || got != SchemaMBP1 {
		t.Fatalf("empty: %v %v", got, err)
	}
	got, err = ParseSchema("mbp-10")
	if err != nil || got != SchemaMBP10 {
		t.Fatalf("mbp-10: %v %v", got, err)
	}
	got, err = ParseSchema("MBO")
	if err != nil || got != SchemaMBO {
		t.Fatalf("mbo: %v %v", got, err)
	}
	if _, err := ParseSchema("definition"); err == nil {
		t.Fatal("unknown schema")
	}
}

func TestDetectSchema(t *testing.T) {
	s, err := DetectSchema(strings.Split(expectedHeader, ","))
	if err != nil || s != SchemaMBP1 {
		t.Fatalf("mbp-1 header: %v %v", s, err)
	}
	s, err = DetectSchema(strings.Split(mbp10Header(), ","))
	if err != nil || s != SchemaMBP10 {
		t.Fatalf("mbp-10 header: %v %v", s, err)
	}
	s, err = DetectSchema(strings.Split(mboHeader, ","))
	if err != nil || s != SchemaMBO {
		t.Fatalf("mbo header: %v %v", s, err)
	}
}

func TestOpenFixtures(t *testing.T) {
	mbp10 := openNamed(t, "../../../testdata/mbp10_sample.csv")
	defer mbp10.Close()
	src, err := Open(mbp10, core.ESZ5(), SchemaMBP10)
	if err != nil {
		t.Fatal(err)
	}
	var ev marketdata.Event
	if err := src.Next(&ev); err != nil {
		t.Fatal(err)
	}
	if ev.Kind != marketdata.KindQuote || ev.Quote.BidPx != 26859 || ev.Depth.Bids[1].Px != 26858 {
		t.Fatalf("mbp10 first quote = kind=%s bid=%d d1=%d", ev.Kind, ev.Quote.BidPx, ev.Depth.Bids[1].Px)
	}

	mbo := openNamed(t, "../../../testdata/mbo_sample.csv")
	defer mbo.Close()
	src, err = Open(mbo, core.ESZ5(), SchemaMBO)
	if err != nil {
		t.Fatal(err)
	}
	if err := src.Next(&ev); err != nil {
		t.Fatal(err)
	}
	if ev.Kind != marketdata.KindBook || ev.Book.Action != marketdata.BookClear {
		t.Fatalf("mbo first = %+v, want clear", ev)
	}
}

func TestOpenFixtureGoldens(t *testing.T) {
	cases := []struct {
		file, golden string
		schema       Schema
	}{
		{"../../../testdata/mbp10_sample.csv", "../../../testdata/golden/mbp10_sample.events.txt", SchemaMBP10},
		{"../../../testdata/mbo_sample.csv", "../../../testdata/golden/mbo_sample.events.txt", SchemaMBO},
	}
	for _, tc := range cases {
		f := openNamed(t, tc.file)
		src, err := Open(f, core.ESZ5(), tc.schema)
		if err != nil {
			t.Fatal(err)
		}
		var got strings.Builder
		var ev marketdata.Event
		i := 0
		for {
			err := src.Next(&ev)
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatal(err)
			}
			fmtLine := formatEvent(ev)
			got.WriteString(fmtLine)
			got.WriteByte('\n')
			i++
		}
		src.Close()
		if *updateGolden {
			if err := os.MkdirAll(filepath.Dir(tc.golden), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(tc.golden, []byte(got.String()), 0o644); err != nil {
				t.Fatal(err)
			}
			t.Logf("wrote %s (%d events)", tc.golden, i)
			continue
		}
		want, err := os.ReadFile(tc.golden)
		if err != nil {
			t.Fatalf("%v (rerun with -update)", err)
		}
		if got.String() != string(want) {
			t.Errorf("%s mismatch\ngot:\n%s\nwant:\n%s", tc.golden, got.String(), want)
		}
	}
}

func TestOpenRealCMEHeaders(t *testing.T) {
	root := filepath.Join("..", "..", "..", "data")
	for _, tc := range []struct {
		name   string
		schema Schema
	}{
		{"databento_glbx.mdp3_mbp_1.csv", SchemaMBP1},
		{"databento_glbx.mdp3_mbp_10.csv", SchemaMBP10},
		{"databento_glbx.mdp3_mbo.csv", SchemaMBO},
	} {
		p := filepath.Join(root, tc.name)
		f, err := os.Open(p)
		if err != nil {
			t.Skipf("no corpus file %s", p)
		}
		src, err := Open(f, core.ESZ5(), tc.schema)
		if err != nil {
			f.Close()
			t.Fatalf("%s: %v", tc.name, err)
		}
		var ev marketdata.Event
		if err := src.Next(&ev); err != nil {
			src.Close()
			t.Fatalf("%s next: %v", tc.name, err)
		}
		if ev.Instrument != core.ESZ5().ID {
			t.Fatalf("%s instrument %d", tc.name, ev.Instrument)
		}
		src.Close()
	}
}

func TestMBOFixtureBuildsL3(t *testing.T) {
	f := openNamed(t, "../../../testdata/mbo_sample.csv")
	defer f.Close()
	src, err := Open(f, core.ESZ5(), SchemaMBO)
	if err != nil {
		t.Fatal(err)
	}
	defer src.Close()
	var book orderbook.L3
	var ev marketdata.Event
	n := 0
	for {
		err := src.Next(&ev)
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		book.Apply(&ev)
		n++
	}
	if n < 2 {
		t.Fatalf("events = %d, want Clear plus Adds", n)
	}
	if book.BestBid().Qty == 0 {
		t.Fatal("L3 bid empty after snapshot Adds")
	}
}

func openNamed(t *testing.T, path string) *os.File {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	return f
}
