package execution

import (
	"errors"
	"testing"

	"github.com/morteza-sakifard/TickEngine/internal/core"
)

func TestRiskNilAllow(t *testing.T) {
	var r *Risk
	if err := r.Allow(Order{Side: core.SideBid, Qty: 99}, 0, 0, 1); err != nil {
		t.Fatal(err)
	}
}

func TestRiskZeroLimitsAllow(t *testing.T) {
	r, err := NewRisk(Limits{})
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Allow(Order{Side: core.SideBid, Qty: 99}, 50, -1_000_000, 1); err != nil {
		t.Fatalf("zero limits must be off: %v", err)
	}
}

func TestRiskKillSwitch(t *testing.T) {
	r, err := NewRisk(Limits{})
	if err != nil {
		t.Fatal(err)
	}
	buy := Order{Side: core.SideBid, Qty: 1}
	if err := r.Allow(buy, 0, 0, 1); err != nil {
		t.Fatal(err)
	}
	r.Kill()
	if !r.Killed() {
		t.Fatal("Killed after Kill")
	}
	if err := r.Allow(buy, 0, 0, 2); !errors.Is(err, ErrRiskKilled) {
		t.Fatalf("after Kill: %v", err)
	}
	if err := r.Allow(Order{Side: core.SideAsk, Qty: 1}, 1, 0, 3); !errors.Is(err, ErrRiskKilled) {
		t.Fatalf("kill blocks flatten too: %v", err)
	}
	r.Arm()
	if r.Killed() {
		t.Fatal("Arm must clear the switch")
	}
	if err := r.Allow(buy, 0, 0, 4); err != nil {
		t.Fatal(err)
	}
}

func TestRiskMaxQty(t *testing.T) {
	r, err := NewRisk(Limits{MaxQty: 2})
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Allow(Order{Side: core.SideBid, Qty: 2}, 0, 0, 1); err != nil {
		t.Fatal(err)
	}
	if err := r.Allow(Order{Side: core.SideBid, Qty: 3}, 0, 0, 2); !errors.Is(err, ErrRiskQty) {
		t.Fatalf("qty 3: %v", err)
	}
}

func TestRiskMaxPosition(t *testing.T) {
	r, err := NewRisk(Limits{MaxAbsPosition: 2})
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Allow(Order{Side: core.SideBid, Qty: 2}, 0, 0, 1); err != nil {
		t.Fatal(err)
	}
	if err := r.Allow(Order{Side: core.SideBid, Qty: 1}, 2, 0, 2); !errors.Is(err, ErrRiskPosition) {
		t.Fatalf("long 2 + buy: %v", err)
	}
	if err := r.Allow(Order{Side: core.SideAsk, Qty: 1}, 2, 0, 3); err != nil {
		t.Fatalf("reduce long must pass: %v", err)
	}
	if err := r.Allow(Order{Side: core.SideAsk, Qty: 5}, 2, 0, 4); !errors.Is(err, ErrRiskPosition) {
		t.Fatalf("flip to short 3: %v", err)
	}
}

func TestRiskMaxDayLossBlocksNewRisk(t *testing.T) {
	r, err := NewRisk(Limits{MaxDayLossCents: 1000})
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Allow(Order{Side: core.SideBid, Qty: 1}, 0, -999, 1); err != nil {
		t.Fatalf("loss 999 is still under the cap: %v", err)
	}
	if err := r.Allow(Order{Side: core.SideBid, Qty: 1}, 0, -1000, 2); !errors.Is(err, ErrRiskDayLoss) {
		t.Fatalf("flat + new buy at cap: %v", err)
	}
	if err := r.Allow(Order{Side: core.SideBid, Qty: 1}, 2, -1000, 3); !errors.Is(err, ErrRiskDayLoss) {
		t.Fatalf("add to long at cap: %v", err)
	}
}

func TestRiskMaxDayLossAllowsFlatten(t *testing.T) {
	r, err := NewRisk(Limits{MaxDayLossCents: 1000})
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Allow(Order{Side: core.SideAsk, Qty: 2}, 2, -2500, 1); err != nil {
		t.Fatalf("flatten long must pass: %v", err)
	}
	if err := r.Allow(Order{Side: core.SideAsk, Qty: 1}, 2, -2500, 2); err != nil {
		t.Fatalf("partial reduce must pass: %v", err)
	}
	if err := r.Allow(Order{Side: core.SideAsk, Qty: 3}, 2, -2500, 3); !errors.Is(err, ErrRiskDayLoss) {
		t.Fatalf("sell 3 vs long 2 is a flip: %v", err)
	}
	if err := r.Allow(Order{Side: core.SideBid, Qty: 1}, -2, -2500, 4); err != nil {
		t.Fatalf("cover short must pass: %v", err)
	}
}

func TestRiskOrderRate(t *testing.T) {
	r, err := NewRisk(Limits{MaxOrders: 2, WindowNs: 100})
	if err != nil {
		t.Fatal(err)
	}
	buy := Order{Side: core.SideBid, Qty: 1}
	if err := r.Allow(buy, 0, 0, 50); err != nil {
		t.Fatal(err)
	}
	if err := r.Allow(buy, 0, 0, 80); err != nil {
		t.Fatal(err)
	}
	if err := r.Allow(buy, 0, 0, 90); !errors.Is(err, ErrRiskRate) {
		t.Fatalf("third in window: %v", err)
	}
	if err := r.Allow(buy, 0, 0, 150); !errors.Is(err, ErrRiskRate) {
		t.Fatalf("at t=150 first stamp (50) still in [50,150]: %v", err)
	}
	if err := r.Allow(buy, 0, 0, 151); err != nil {
		t.Fatalf("t=151 drops stamp 50: %v", err)
	}
}

func TestRiskRateSameTimestampCounts(t *testing.T) {
	r, err := NewRisk(Limits{MaxOrders: 1, WindowNs: 10})
	if err != nil {
		t.Fatal(err)
	}
	buy := Order{Side: core.SideBid, Qty: 1}
	if err := r.Allow(buy, 0, 0, 5); err != nil {
		t.Fatal(err)
	}
	if err := r.Allow(buy, 0, 0, 5); !errors.Is(err, ErrRiskRate) {
		t.Fatalf("two submits at one TsRecv: %v", err)
	}
}

func TestRiskRateRejectDoesNotConsume(t *testing.T) {
	r, err := NewRisk(Limits{MaxQty: 1, MaxOrders: 1, WindowNs: 100})
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Allow(Order{Side: core.SideBid, Qty: 9}, 0, 0, 1); !errors.Is(err, ErrRiskQty) {
		t.Fatal(err)
	}
	if err := r.Allow(Order{Side: core.SideBid, Qty: 1}, 0, 0, 1); err != nil {
		t.Fatalf("rejected qty must not burn the rate slot: %v", err)
	}
}

func TestRiskLimitsValidate(t *testing.T) {
	if _, err := NewRisk(Limits{MaxQty: -1}); err == nil {
		t.Fatal("negative MaxQty")
	}
	if _, err := NewRisk(Limits{MaxDayLossCents: -1}); err == nil {
		t.Fatal("negative day loss")
	}
	if _, err := NewRisk(Limits{MaxOrders: 3}); err == nil {
		t.Fatal("MaxOrders without WindowNs")
	}
	if _, err := NewRisk(Limits{MaxOrders: -1, WindowNs: 1}); err == nil {
		t.Fatal("negative MaxOrders")
	}
}

func TestRiskCheckOrder(t *testing.T) {
	r, err := NewRisk(Limits{MaxQty: 1, MaxAbsPosition: 1, MaxDayLossCents: 1, MaxOrders: 1, WindowNs: 10})
	if err != nil {
		t.Fatal(err)
	}
	r.Kill()
	if err := r.Allow(Order{Side: core.SideBid, Qty: 9}, 9, -99, 1); !errors.Is(err, ErrRiskKilled) {
		t.Fatalf("kill must win: %v", err)
	}
}
