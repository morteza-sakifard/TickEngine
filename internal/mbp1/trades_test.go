package mbp1

import (
	"errors"
	"io"
	"strings"
	"testing"
)

const mixedActionCSV = `ts_recv,ts_event,action,side,price,size,sequence,symbol
2025-09-22T00:00:00.000000000Z,2025-09-22T00:00:00.000000000Z,A,N,6713.500000000,2,1,ESZ5
2025-09-22T00:00:00.010000000Z,2025-09-22T00:00:00.010000000Z,T,B,6714.750000000,1,2,ESZ5
2025-09-22T00:00:00.020000000Z,2025-09-22T00:00:00.020000000Z,C,B,6714.750000000,1,3,ESZ5
2025-09-22T00:00:00.030000000Z,2025-09-22T00:00:00.030000000Z,T,A,6714.500000000,3,4,ESZ5
`

func TestTradeReader_Read(t *testing.T) {
	r, err := NewReader(strings.NewReader(mixedActionCSV))
	if err != nil {
		t.Fatalf("NewReader: %v", err)
	}

	tr := NewTradeReader(r)

	first, err := tr.Read()
	if err != nil {
		t.Fatalf("Read (trade 1): %v", err)
	}
	if first.Action != ActionTrade || first.Side != SideBid || first.Size != 1 {
		t.Errorf("got %+v, want Action=T Side=B Size=1", first)
	}

	second, err := tr.Read()
	if err != nil {
		t.Fatalf("Read (trade 2): %v", err)
	}
	if second.Action != ActionTrade || second.Side != SideAsk || second.Size != 3 {
		t.Errorf("got %+v, want Action=T Side=A Size=3", second)
	}

	if _, err := tr.Read(); !errors.Is(err, io.EOF) {
		t.Fatalf("Read (after last trade): err = %v, want io.EOF", err)
	}
}
