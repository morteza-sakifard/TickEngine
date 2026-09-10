package orderflow

import (
	"testing"
	"time"

	"github.com/morteza-sakifard/TickEngine/internal/core"
)

func addVP(vp *VolumeProfile, px, qty int64) {
	e := tr(px, qty, core.SideBid, time.Time{})
	vp.OnTrade(&e)
}

func TestVolumeProfileEmpty(t *testing.T) {
	var vp VolumeProfile
	if got := vp.Levels(); len(got) != 0 {
		t.Fatalf("Levels = %v, want empty", got)
	}
	val, poc, vah := vp.ValueArea()
	if val != 0 || poc != 0 || vah != 0 {
		t.Fatalf("ValueArea = %d,%d,%d, want 0,0,0", val, poc, vah)
	}
}

func TestVolumeProfileLevelsSumToTotal(t *testing.T) {
	var vp VolumeProfile
	addVP(&vp, 100, 5)
	addVP(&vp, 101, 3)
	addVP(&vp, 100, 2)
	none := tr(100, 4, core.SideNone, time.Time{})
	vp.OnTrade(&none)
	if vp.Total() != 14 {
		t.Fatalf("Total = %d, want 14", vp.Total())
	}
	var sum core.Qty
	for _, lv := range vp.Levels() {
		sum += lv.Volume
	}
	if sum != vp.Total() {
		t.Fatalf("sum(Levels) = %d, Total = %d", sum, vp.Total())
	}
	got := vp.Levels()
	if len(got) != 2 || got[0].Price != 100 || got[0].Volume != 11 || got[1].Price != 101 || got[1].Volume != 3 {
		t.Fatalf("Levels = %+v", got)
	}
}

func TestPOCTieTakesLowerPrice(t *testing.T) {
	var vp VolumeProfile
	addVP(&vp, 102, 5)
	addVP(&vp, 100, 5)
	if got := vp.POC(); got != 100 {
		t.Fatalf("POC = %d, want 100 (lowest price among ties)", got)
	}
}

func TestValueAreaHandWalk(t *testing.T) {
	// px 1..9 volumes 1,2,3,5,8,5,3,2,1. Total 30. ceil(70%) = 21.
	// POC=5. Tie at 4 and 6 → expand down. Then 6. Then tie 3 and 7 → down.
	// VAL=3 POC=5 VAH=6. VA vol = 3+5+8+5 = 21.
	var vp VolumeProfile
	for i, q := range []int64{1, 2, 3, 5, 8, 5, 3, 2, 1} {
		addVP(&vp, int64(i+1), q)
	}
	val, poc, vah := vp.ValueArea()
	if poc != 5 || val != 3 || vah != 6 {
		t.Fatalf("VAL,POC,VAH = %d,%d,%d, want 3,5,6", val, poc, vah)
	}
	if val > poc || poc > vah {
		t.Fatalf("VAL ≤ POC ≤ VAH violated: %d %d %d", val, poc, vah)
	}
	var vaVol core.Qty
	for _, lv := range vp.Levels() {
		if lv.Price >= val && lv.Price <= vah {
			vaVol += lv.Volume
		}
	}
	if vaVol != 21 {
		t.Fatalf("VA volume = %d, want 21", vaVol)
	}
}

func TestValueAreaSeventyPercentBound(t *testing.T) {
	var vp VolumeProfile
	for i, q := range []int64{1, 2, 3, 5, 8, 5, 3, 2, 1} {
		addVP(&vp, int64(10+i), q)
	}
	val, poc, vah := vp.ValueArea()
	if val > poc || poc > vah {
		t.Fatalf("VAL ≤ POC ≤ VAH violated: %d %d %d", val, poc, vah)
	}
	var vaVol, maxInVA core.Qty
	for _, lv := range vp.Levels() {
		if lv.Price >= val && lv.Price <= vah {
			vaVol += lv.Volume
			if lv.Volume > maxInVA {
				maxInVA = lv.Volume
			}
		}
	}
	target := (int64(vp.Total())*70 + 99) / 100
	if int64(vaVol) < target {
		t.Fatalf("VA volume %d < ceil(70%%) %d", vaVol, target)
	}
	if int64(vaVol)-target > int64(maxInVA) {
		t.Fatalf("VA overshoot %d > one level (%d)", int64(vaVol)-target, maxInVA)
	}
}

func TestVolumeProfileReset(t *testing.T) {
	var vp VolumeProfile
	addVP(&vp, 26800, 4)
	vp.Reset()
	var empty VolumeProfile
	if vp.Total() != empty.Total() || vp.POC() != empty.POC() || len(vp.Levels()) != 0 {
		t.Fatal("Reset did not restore the zero value")
	}
	addVP(&vp, 26800, 4)
	fresh := VolumeProfile{}
	addVP(&fresh, 26800, 4)
	if vp.Total() != fresh.Total() || vp.POC() != fresh.POC() {
		t.Fatal("after Reset+OnTrade, profile differs from a fresh one")
	}
}

func TestValueAreaSingleLevel(t *testing.T) {
	var vp VolumeProfile
	addVP(&vp, 26800, 3)
	val, poc, vah := vp.ValueArea()
	if val != 26800 || poc != 26800 || vah != 26800 {
		t.Fatalf("single level VA = %d,%d,%d, want all 26800", val, poc, vah)
	}
}
