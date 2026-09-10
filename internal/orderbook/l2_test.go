package orderbook

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/morteza-sakifard/TickEngine/internal/core"
	"github.com/morteza-sakifard/TickEngine/internal/feed/databento"
	"github.com/morteza-sakifard/TickEngine/internal/marketdata"
)

func quoteEv(seq uint32, flags marketdata.Flags, bidPx, askPx core.Ticks, bidQty, askQty core.Qty) marketdata.Event {
	q := marketdata.Quote{BidPx: bidPx, AskPx: askPx, BidQty: bidQty, AskQty: askQty, BidCt: 1, AskCt: 1}
	return marketdata.Event{
		Kind: marketdata.KindQuote, Sequence: seq, Flags: flags,
		Quote: q, Depth: marketdata.DepthFromQuote(q),
	}
}

func TestSnapshotResetsBook(t *testing.T) {
	var b L2
	b.Apply(&marketdata.Event{
		Kind:     marketdata.KindQuote,
		Sequence: 5,
		Depth: marketdata.Depth{
			Bids: [10]Level{{Px: 100, Qty: 7, Count: 2}, {Px: 99, Qty: 3, Count: 1}},
			Asks: [10]Level{{Px: 101, Qty: 4, Count: 1}},
		},
	})
	if n := len(b.Bids(10)); n != 2 {
		t.Fatalf("pre-snapshot bids = %d", n)
	}
	b.Apply(&marketdata.Event{
		Kind:     marketdata.KindQuote,
		Sequence: 1,
		Flags:    marketdata.FlagSnapshot,
		Depth: marketdata.Depth{
			Bids: [10]Level{{Px: 50, Qty: 1, Count: 1}},
			Asks: [10]Level{{Px: 51, Qty: 1, Count: 1}},
		},
	})
	bids := b.Bids(10)
	if len(bids) != 1 || bids[0].Px != 50 {
		t.Fatalf("snapshot must replace, got %+v", bids)
	}
	if b.Gaps() != 0 {
		t.Fatalf("snapshot is a new baseline, gaps=%d", b.Gaps())
	}
	if err := b.AssertNotCrossed(); err != nil {
		t.Fatal(err)
	}
}

func TestSequenceGapReported(t *testing.T) {
	var b L2
	b.Apply(&marketdata.Event{Kind: marketdata.KindQuote, Sequence: 10, Quote: marketdata.Quote{BidPx: 1, AskPx: 2, BidQty: 1, AskQty: 1}, Depth: marketdata.DepthFromQuote(marketdata.Quote{BidPx: 1, AskPx: 2, BidQty: 1, AskQty: 1})})
	b.Apply(&marketdata.Event{Kind: marketdata.KindQuote, Sequence: 13, Quote: marketdata.Quote{BidPx: 1, AskPx: 2, BidQty: 1, AskQty: 1}, Depth: marketdata.DepthFromQuote(marketdata.Quote{BidPx: 1, AskPx: 2, BidQty: 1, AskQty: 1})})
	g, ok := b.LastGap()
	if !ok || b.Gaps() != 1 || g.Prev != 10 || g.Got != 13 {
		t.Fatalf("gap = %+v ok=%v n=%d", g, ok, b.Gaps())
	}
}

func TestCrossedBookDetected(t *testing.T) {
	var b L2
	b.Apply(&marketdata.Event{
		Kind:  marketdata.KindQuote,
		Quote: marketdata.Quote{BidPx: 102, AskPx: 100, BidQty: 1, AskQty: 1},
		Depth: marketdata.DepthFromQuote(marketdata.Quote{BidPx: 102, AskPx: 100, BidQty: 1, AskQty: 1}),
	})
	if !b.Crossed() {
		t.Fatal("want Crossed on bid > ask")
	}
	if err := b.AssertNotCrossed(); err == nil {
		t.Fatal("want assert error")
	}
}

func TestHealthyBookBestBidBelowAsk(t *testing.T) {
	var b L2
	ev := quoteEv(1, 0, 100, 101, 2, 3)
	b.Apply(&ev)
	if b.BestBid().Px >= b.BestAsk().Px {
		t.Fatalf("bestBid=%d bestAsk=%d", b.BestBid().Px, b.BestAsk().Px)
	}
	if b.Spread() != 1 || b.BestBid().Qty != 2 || b.BestAsk().Qty != 3 {
		t.Fatalf("touch %+v / %+v", b.BestBid(), b.BestAsk())
	}
}

func TestL2Level0MatchesMBP1Quote(t *testing.T) {
	path := filepath.Join("..", "..", "data", "test", "mbp1_sample.csv")
	f, err := os.Open(path)
	if err != nil {
		t.Skip(err)
	}
	dec, err := databento.NewDecoder(f, core.ESZ5())
	if err != nil {
		f.Close()
		t.Fatal(err)
	}
	defer dec.Close()
	var b L2
	var ev marketdata.Event
	n := 0
	for {
		err := dec.Next(&ev)
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if ev.Kind != marketdata.KindQuote {
			continue
		}
		b.Apply(&ev)
		// Slot 0, not Best*: an empty side is UNDEF + qty 0 on the
		// quote, and BestAsk skips qty 0.
		if b.bids[0].Px != ev.Quote.BidPx || b.bids[0].Qty != ev.Quote.BidQty {
			t.Fatalf("bid L2=%+v quote=%+v", b.bids[0], ev.Quote)
		}
		if b.asks[0].Px != ev.Quote.AskPx || b.asks[0].Qty != ev.Quote.AskQty {
			t.Fatalf("ask L2=%+v quote=%+v", b.asks[0], ev.Quote)
		}
		n++
	}
	if n == 0 {
		t.Fatal("no quotes")
	}
}

func TestPackageConstraints(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		b, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		s := string(b)
		for _, bad := range []string{
			"time.Now(", "os.Open", "\ngo ", " chan ", "\tchan ", "\nselect ", " select ",
			"internal/feed", "internal/strategy", "internal/chart", "internal/replay",
		} {
			if strings.Contains(s, bad) {
				t.Errorf("%s contains %q", name, strings.TrimSpace(bad))
			}
		}
	}
}
