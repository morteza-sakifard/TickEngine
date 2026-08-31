package mbp1

import (
	"errors"
	"io"
	"strings"
	"testing"
	"time"
)

const sampleCSV = `ts_recv,ts_event,rtype,publisher_id,instrument_id,action,side,depth,price,size,flags,ts_in_delta,sequence,bid_px_00,ask_px_00,bid_sz_00,ask_sz_00,bid_ct_00,ask_ct_00,symbol
2025-09-22T00:00:00.000000000Z,2025-09-21T23:59:59.878773407Z,1,1,294973,A,N,0,6713.500000000,2,168,0,144737,6714.750000000,6715.000000000,9,11,7,10,ESZ5
2025-09-22T00:00:00.161086709Z,2025-09-22T00:00:00.160844375Z,1,1,294973,T,B,0,6714.750000000,1,0,12249,144773,6714.500000000,6714.750000000,17,3,14,3,ESZ5
`

func TestReader_Read(t *testing.T) {
	r, err := NewReader(strings.NewReader(sampleCSV))
	if err != nil {
		t.Fatalf("NewReader: %v", err)
	}

	first, err := r.Read()
	if err != nil {
		t.Fatalf("Read (row 1): %v", err)
	}
	wantTsEvent := time.Date(2025, 9, 21, 23, 59, 59, 878773407, time.UTC)
	if !first.TsEvent.Equal(wantTsEvent) {
		t.Errorf("TsEvent = %v, want %v", first.TsEvent, wantTsEvent)
	}
	if first.Action != ActionAdd || first.Side != SideUnknown {
		t.Errorf("Action/Side = %q/%q, want %q/%q", first.Action, first.Side, ActionAdd, SideUnknown)
	}
	if first.Price != 6713.5 || first.Symbol != "ESZ5" {
		t.Errorf("Price/Symbol = %v/%q, want 6713.5/ESZ5", first.Price, first.Symbol)
	}

	trade, err := r.Read()
	if err != nil {
		t.Fatalf("Read (row 2): %v", err)
	}
	if trade.Action != ActionTrade || trade.Side != SideBid {
		t.Errorf("Action/Side = %q/%q, want %q/%q", trade.Action, trade.Side, ActionTrade, SideBid)
	}
	if trade.Size != 1 || trade.Price != 6714.75 {
		t.Errorf("Size/Price = %d/%v, want 1/6714.75", trade.Size, trade.Price)
	}

	if _, err := r.Read(); !errors.Is(err, io.EOF) {
		t.Fatalf("Read (row 3): err = %v, want io.EOF", err)
	}
}

func TestReader_MissingColumn(t *testing.T) {
	if _, err := NewReader(strings.NewReader("ts_recv,ts_event\n")); err == nil {
		t.Fatal("expected error for missing columns, got nil")
	}
}
