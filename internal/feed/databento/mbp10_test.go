package databento

import (
	"io"
	"strings"
	"testing"

	"github.com/morteza-sakifard/TickEngine/internal/core"
	"github.com/morteza-sakifard/TickEngine/internal/marketdata"
	"github.com/morteza-sakifard/TickEngine/internal/orderbook"
)

func emptyTail(from int) string {
	var b strings.Builder
	for i := from; i < mbp10Levels; i++ {
		b.WriteString(",0.000000000,0.000000000,0,0,0,0")
	}
	return b.String()
}

func mbp10Row(action, flags, seq, l0, l1 string) string {
	return "2025-09-22T00:00:00.000000000Z,2025-09-22T00:00:00.000000000Z,1,1,294973," +
		action + ",N,0,6700.000000000,1," + flags + ",0," + seq + "," + l0 + "," + l1 + emptyTail(2) + ",ESZ5"
}

func TestMBP10DecodeAndBook(t *testing.T) {
	l0 := "6700.000000000,6700.250000000,5,4,2,2"
	l1 := "6699.750000000,6700.500000000,3,8,1,3"
	l1pulled := "6699.750000000,6700.500000000,3,1,1,1"
	csv := mbp10Header() + "\n" +
		mbp10Row("A", "32", "10", l0, l1) + "\n" +
		mbp10Row("C", "128", "11", l0, l1pulled) + "\n"

	d, err := NewMBP10(io.NopCloser(strings.NewReader(csv)), core.ESZ5())
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	var ev marketdata.Event
	if err := d.Next(&ev); err != nil {
		t.Fatal(err)
	}
	if ev.Kind != marketdata.KindQuote || ev.Flags&marketdata.FlagSnapshot == 0 {
		t.Fatalf("first row = %+v", ev)
	}
	if ev.Quote.BidPx != 26800 || ev.Depth.Bids[1].Px != 26799 || ev.Depth.Asks[1].Qty != 8 {
		t.Fatalf("depth[0]=%+v depth[1] bid=%+v ask=%+v", ev.Depth.Bids[0], ev.Depth.Bids[1], ev.Depth.Asks[1])
	}

	var book orderbook.L2
	book.Apply(&ev)
	if err := book.AssertNotCrossed(); err != nil {
		t.Fatal(err)
	}
	asks := book.Asks(10)
	if len(asks) != 2 || asks[1].Qty != 8 {
		t.Fatalf("asks before pull = %+v", asks)
	}

	if err := d.Next(&ev); err != nil {
		t.Fatal(err)
	}
	book.Apply(&ev)
	asks = book.Asks(10)
	if len(asks) != 2 || asks[1].Qty != 1 {
		t.Fatalf("wall should pull to 1, got %+v", asks)
	}
	if err := d.Next(&ev); err != io.EOF {
		t.Fatalf("want EOF, got %v", err)
	}
}

func TestMBP10WrongHeader(t *testing.T) {
	_, err := NewMBP10(io.NopCloser(strings.NewReader(expectedHeader+"\n")), core.ESZ5())
	if err == nil {
		t.Fatal("MBP-1 header must not decode as MBP-10")
	}
}

func TestMBP10TradeThenQuote(t *testing.T) {
	l0 := "6700.000000000,6700.250000000,5,4,2,2"
	l1 := "6699.750000000,6700.500000000,3,8,1,3"
	csv := mbp10Header() + "\n" + mbp10Row("T", "128", "7", l0, l1) + "\n"
	d, err := NewMBP10(io.NopCloser(strings.NewReader(csv)), core.ESZ5())
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	var ev marketdata.Event
	if err := d.Next(&ev); err != nil || ev.Kind != marketdata.KindTrade {
		t.Fatalf("trade: %v %+v", err, ev)
	}
	if err := d.Next(&ev); err != nil || ev.Kind != marketdata.KindQuote {
		t.Fatalf("quote: %v %+v", err, ev)
	}
	if ev.Depth.Asks[1].Qty != 8 {
		t.Fatalf("pending quote lost depth: %+v", ev.Depth)
	}
}
