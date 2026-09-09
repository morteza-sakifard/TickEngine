package portfolio

import (
	"bytes"
	"strings"
	"testing"

	"github.com/morteza-sakifard/market-data-lab/internal/core"
	"github.com/morteza-sakifard/market-data-lab/internal/execution"
)

func TestStoreReplayMatchesLastPosition(t *testing.T) {
	inst := core.ESZ5()
	s := NewStore()
	buy := execution.Fill{OrderID: 1, Ts: 10, Px: 26800, Qty: 1, Side: core.SideBid}
	sell := execution.Fill{OrderID: 2, Ts: 20, Px: 26801, Qty: 1, Side: core.SideAsk}
	if err := s.AppendOrder(10, execution.Order{ID: 1, Side: core.SideBid, Qty: 1}, 0); err != nil {
		t.Fatal(err)
	}
	if err := s.AppendFill(10, buy); err != nil {
		t.Fatal(err)
	}
	var p Position
	p.Apply(inst, buy)
	if err := s.AppendPosition(10, p); err != nil {
		t.Fatal(err)
	}
	if err := s.AppendOrder(20, execution.Order{ID: 2, Side: core.SideAsk, Qty: 1}, 0); err != nil {
		t.Fatal(err)
	}
	if err := s.AppendFill(20, sell); err != nil {
		t.Fatal(err)
	}
	p.Apply(inst, sell)
	if err := s.AppendPosition(20, p); err != nil {
		t.Fatal(err)
	}
	got := s.Replay(inst)
	last, ok := s.LastPosition()
	if !ok || got != last || got.Qty != 0 || got.Realized != CentsPerTick(inst) {
		t.Fatalf("replay=%+v last=%+v ok=%v", got, last, ok)
	}
	if len(s.WorkingOrders()) != 0 {
		t.Fatalf("filled orders still working: %+v", s.WorkingOrders())
	}
}

func TestStoreWorkingAfterCancel(t *testing.T) {
	s := NewStore()
	o := execution.Order{ID: 4, Side: core.SideBid, Qty: 1, Kind: execution.KindLimit, Px: 99}
	if err := s.AppendOrder(1, o, 0); err != nil {
		t.Fatal(err)
	}
	if got := s.WorkingOrders(); len(got) != 1 || got[0].ID != 4 {
		t.Fatalf("working = %+v", got)
	}
	if err := s.AppendOrder(2, execution.Order{ID: 4}, execution.StatusCanceled); err != nil {
		t.Fatal(err)
	}
	if got := s.WorkingOrders(); len(got) != 0 {
		t.Fatalf("canceled still working: %+v", got)
	}
}

func TestStoreRoundTripBytes(t *testing.T) {
	s := NewStore()
	if err := s.AppendOrder(5, execution.Order{
		ID: 3, Instrument: 294973, Side: core.SideAsk, Qty: 2, Kind: execution.KindLimit, Px: 100,
	}, 0); err != nil {
		t.Fatal(err)
	}
	if err := s.AppendFill(6, execution.Fill{OrderID: 3, Ts: 6, Side: core.SideAsk, Px: 100, Qty: 2, FeeCents: 12}); err != nil {
		t.Fatal(err)
	}
	if err := s.AppendPosition(6, Position{Qty: -2, AvgPx: 100, Realized: -12}); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := s.WriteTo(&buf); err != nil {
		t.Fatal(err)
	}
	dump := buf.String()
	if !strings.HasPrefix(dump, "v1\n") {
		t.Fatalf("header: %q", dump)
	}
	if strings.Contains(dump, "KindTrade") || strings.Contains(dump, "bid_px") {
		t.Fatal("store must not carry market data")
	}
	dst := NewStore()
	if err := dst.Load(strings.NewReader(dump)); err != nil {
		t.Fatal(err)
	}
	var again bytes.Buffer
	if err := dst.WriteTo(&again); err != nil {
		t.Fatal(err)
	}
	if dump != again.String() {
		t.Fatalf("round trip:\n%s\n---\n%s", dump, again.String())
	}
	if dst.LastID() != 3 || dst.LastTs() != 6 {
		t.Fatalf("LastID=%d LastTs=%d", dst.LastID(), dst.LastTs())
	}
}

func TestStoreLoadRejectsBadVersion(t *testing.T) {
	s := NewStore()
	if err := s.Load(strings.NewReader("v2\n")); err == nil {
		t.Fatal("want error for v2")
	}
}

func TestStoreAppendFillRejectsZeroQty(t *testing.T) {
	s := NewStore()
	if err := s.AppendFill(1, execution.Fill{OrderID: 1, Qty: 0}); err == nil {
		t.Fatal("zero fill")
	}
}
