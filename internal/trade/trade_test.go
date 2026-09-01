package trade

import (
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/morteza-sakifard/market-data-lab/internal/mbp1"
)

const sampleCSV = `ts_recv,ts_event,action,side,price,size,sequence,symbol
2025-09-22T00:00:00.161086709Z,2025-09-22T00:00:00.160844375Z,T,B,6714.750000000,1,144773,ESZ5
2025-09-22T00:00:00.161336811Z,2025-09-22T00:00:00.161083905Z,T,A,6714.500000000,3,144776,ESZ5
`

func TestReader_Read(t *testing.T) {
	mr, err := mbp1.NewReader(strings.NewReader(sampleCSV))
	if err != nil {
		t.Fatalf("mbp1.NewReader: %v", err)
	}
	r := NewReader(mbp1.NewTradeReader(mr))

	buy, err := r.Read()
	if err != nil {
		t.Fatalf("Read (1): %v", err)
	}
	if buy.Side != Buy || buy.Price != 6714.75 || buy.Size != 1 {
		t.Errorf("got %+v, want Side=Buy Price=6714.75 Size=1", buy)
	}

	sell, err := r.Read()
	if err != nil {
		t.Fatalf("Read (2): %v", err)
	}
	if sell.Side != Sell || sell.Price != 6714.5 || sell.Size != 3 {
		t.Errorf("got %+v, want Side=Sell Price=6714.5 Size=3", sell)
	}

	if _, err := r.Read(); !errors.Is(err, io.EOF) {
		t.Fatalf("Read (3): err = %v, want io.EOF", err)
	}
}
func TestFromRecord_UnknownSide(t *testing.T) {
	rec := mbp1.Record{Action: mbp1.ActionTrade, Side: mbp1.SideUnknown}
	if _, err := FromRecord(rec); err == nil {
		t.Fatal("expected error for unknown side, got nil")
	}
}
func TestFromRecord_NotATrade(t *testing.T) {
	rec := mbp1.Record{Action: mbp1.ActionAdd, Side: mbp1.SideBid}
	if _, err := FromRecord(rec); err == nil {
		t.Fatal("expected error for non-trade action, got nil")
	}
}
