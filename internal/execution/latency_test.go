package execution

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/morteza-sakifard/TickEngine/internal/core"
	"github.com/morteza-sakifard/TickEngine/internal/feed/databento"
	"github.com/morteza-sakifard/TickEngine/internal/marketdata"
)

func TestFeedNsAndRecvLag(t *testing.T) {
	ev := marketdata.Event{TsEvent: 100, TsRecv: 130, TsInDelta: 25}
	if FeedNs(ev) != 25 {
		t.Fatalf("FeedNs = %d, want ts_in_delta 25", FeedNs(ev))
	}
	if RecvLag(ev) != 30 {
		t.Fatalf("RecvLag = %d, want 30", RecvLag(ev))
	}
}

func TestFeedDistFromFixture(t *testing.T) {
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
	var d FeedDist
	for {
		var ev marketdata.Event
		err := dec.Next(&ev)
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		d.Add(ev)
	}
	if d.N() == 0 {
		t.Fatal("fixture produced no feed samples")
	}
	if d.Min() > d.Max() {
		t.Fatalf("min %d > max %d", d.Min(), d.Max())
	}
	p50 := d.Percentile(50)
	if p50 < d.Min() || p50 > d.Max() {
		t.Fatalf("p50 %d outside [%d, %d]", p50, d.Min(), d.Max())
	}
}

func TestZeroLatencyMarketSameAsStep20(t *testing.T) {
	v := NewVenue(Fees{})
	if err := v.SetLatency(Latency{}); err != nil {
		t.Fatal(err)
	}
	v.SetQuote(marketdata.Quote{BidPx: 100, AskPx: 101})
	if _, err := v.Enqueue(Order{Side: core.SideBid, Qty: 1}); err != nil {
		t.Fatal(err)
	}
	evs := v.Settle(50)
	if len(evs) != 1 || evs[0].Fill.Px != 101 || evs[0].Fill.Ts != 50 {
		t.Fatalf("zero latency buy = %+v, want immediate ask 101", evs)
	}
}

func TestEntryDelayHitsLaterQuote(t *testing.T) {
	v := NewVenue(Fees{})
	if err := v.SetLatency(Latency{Entry: 15}); err != nil {
		t.Fatal(err)
	}
	v.Sync(10)
	v.SetQuote(marketdata.Quote{BidPx: 1, AskPx: 5})
	if _, err := v.Enqueue(Order{Side: core.SideBid, Qty: 1}); err != nil {
		t.Fatal(err)
	}
	if evs := v.Settle(10); evs != nil {
		t.Fatalf("filled before entry elapsed: %+v", evs)
	}
	v.SetQuote(marketdata.Quote{BidPx: 1, AskPx: 9})
	evs := v.Settle(26)
	if len(evs) != 1 || evs[0].Fill.Px != 9 {
		t.Fatalf("entry-delayed fill = %+v, want later ask 9", evs)
	}
}

func TestResponseDelayHoldsFill(t *testing.T) {
	v := NewVenue(Fees{})
	if err := v.SetLatency(Latency{Response: 50}); err != nil {
		t.Fatal(err)
	}
	v.SetQuote(marketdata.Quote{BidPx: 1, AskPx: 2})
	if _, err := v.Enqueue(Order{Side: core.SideBid, Qty: 1}); err != nil {
		t.Fatal(err)
	}
	if evs := v.Settle(100); evs != nil {
		t.Fatalf("response delay leaked fill at 100: %+v", evs)
	}
	if evs := v.Settle(149); evs != nil {
		t.Fatalf("fill at 149, want 150: %+v", evs)
	}
	evs := v.Settle(150)
	if len(evs) != 1 || evs[0].Fill.Px != 2 {
		t.Fatalf("want fill at 150, got %+v", evs)
	}
}

func TestFlushReleasesTail(t *testing.T) {
	v := NewVenue(Fees{})
	if err := v.SetLatency(Latency{Entry: 1_000, Response: 1_000}); err != nil {
		t.Fatal(err)
	}
	v.Sync(1)
	v.SetQuote(marketdata.Quote{BidPx: 1, AskPx: 2})
	if _, err := v.Enqueue(Order{Side: core.SideBid, Qty: 1}); err != nil {
		t.Fatal(err)
	}
	evs := v.Flush()
	if len(evs) != 1 {
		t.Fatalf("Flush at end of tape = %+v", evs)
	}
}

func TestSetLatencyRejectsNegative(t *testing.T) {
	v := NewVenue(Fees{})
	if err := v.SetLatency(Latency{Entry: -1}); err == nil {
		t.Fatal("want error for negative entry")
	}
}

func TestCancelDuringEntryDelay(t *testing.T) {
	v := NewVenue(Fees{})
	if err := v.SetLatency(Latency{Entry: 10}); err != nil {
		t.Fatal(err)
	}
	v.Sync(1)
	v.SetQuote(marketdata.Quote{BidPx: 1, AskPx: 2})
	id, err := v.Enqueue(Order{Side: core.SideBid, Qty: 1})
	if err != nil {
		t.Fatal(err)
	}
	if err := v.Cancel(id); err != nil {
		t.Fatal(err)
	}
	if evs := v.Settle(20); evs != nil {
		t.Fatalf("canceled delayed order filled: %+v", evs)
	}
}
