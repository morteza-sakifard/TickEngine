package databento

import (
	"io"
	"strings"
	"testing"

	"github.com/morteza-sakifard/TickEngine/internal/core"
	"github.com/morteza-sakifard/TickEngine/internal/marketdata"
	"github.com/morteza-sakifard/TickEngine/internal/orderbook"
)

func mboRow(action, side, px, size, oid, flags, seq string) string {
	return "2025-09-22T00:00:00.000000000Z,2025-09-22T00:00:00.000000000Z,160,1,294973," +
		action + "," + side + "," + px + "," + size + ",0," + oid + "," + flags + ",0," + seq + ",ESZ5"
}

func TestMBODecodeBookAndTrade(t *testing.T) {
	csv := mboHeader + "\n" +
		mboRow("R", "N", "0.000000000", "0", "0", "32", "0") + "\n" +
		mboRow("A", "B", "6700.000000000", "10", "1", "128", "1") + "\n" +
		mboRow("A", "A", "6700.250000000", "4", "2", "128", "2") + "\n" +
		mboRow("F", "B", "6700.000000000", "1", "1", "128", "3") + "\n" +
		mboRow("T", "A", "6700.000000000", "1", "9", "128", "3") + "\n"

	d, err := NewMBO(io.NopCloser(strings.NewReader(csv)), core.ESZ5())
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	var ev marketdata.Event
	if err := d.Next(&ev); err != nil || ev.Kind != marketdata.KindBook || ev.Book.Action != marketdata.BookClear {
		t.Fatalf("clear: %v %+v", err, ev)
	}
	if ev.Flags&marketdata.FlagSnapshot == 0 {
		t.Fatal("clear should carry SNAPSHOT")
	}
	if err := d.Next(&ev); err != nil || ev.Book.Action != marketdata.BookAdd || ev.Book.OrderID != 1 || ev.Book.Px != 26800 || ev.Book.Qty != 10 {
		t.Fatalf("add bid: %v %+v", err, ev.Book)
	}
	if err := d.Next(&ev); err != nil || ev.Book.Side != core.SideAsk || ev.Book.Px != 26801 {
		t.Fatalf("add ask: %v %+v", err, ev.Book)
	}
	if err := d.Next(&ev); err != nil || ev.Kind != marketdata.KindTrade || ev.Trade.Px != 26800 || ev.Trade.Qty != 1 || ev.Trade.Aggressor != core.SideAsk {
		t.Fatalf("trade (fill skipped): %v %+v", err, ev)
	}
	if err := d.Next(&ev); err != io.EOF {
		t.Fatalf("want EOF, got %v", err)
	}
}

func TestMBOWrongHeader(t *testing.T) {
	_, err := NewMBO(io.NopCloser(strings.NewReader(expectedHeader+"\n")), core.ESZ5())
	if err == nil {
		t.Fatal("MBP-1 header must not decode as MBO")
	}
}

func TestMBOBuildsL3(t *testing.T) {
	csv := mboHeader + "\n" +
		mboRow("A", "B", "6700.000000000", "5", "1", "128", "1") + "\n" +
		mboRow("A", "B", "6700.000000000", "3", "2", "128", "2") + "\n" +
		mboRow("A", "A", "6700.250000000", "4", "3", "128", "3") + "\n"
	d, err := NewMBO(io.NopCloser(strings.NewReader(csv)), core.ESZ5())
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	var book orderbook.L3
	var ev marketdata.Event
	for {
		err := d.Next(&ev)
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		book.Apply(&ev)
	}
	if book.BestBid().Qty != 8 || book.BestBid().Count != 2 || book.BestAsk().Qty != 4 {
		t.Fatalf("book %+v / %+v", book.BestBid(), book.BestAsk())
	}
}
