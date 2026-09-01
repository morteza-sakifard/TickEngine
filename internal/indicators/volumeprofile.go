package indicators

import (
	"sort"

	"github.com/morteza-sakifard/market-data-lab/internal/trade"
)

// PriceLevel is one row of a volume profile: total traded volume at a price.
type PriceLevel struct {
	Price  float64
	Volume int64
}

// VolumeProfile aggregates traded volume by price level:
// VolumeProfile[price] = sum of Size for all trades at that price.
type VolumeProfile struct {
	volumes map[float64]int64
}

// Add incorporates one trade into the profile.
func (vp *VolumeProfile) Add(t trade.Trade) {
	if vp.volumes == nil {
		vp.volumes = make(map[float64]int64)
	}
	vp.volumes[t.Price] += int64(t.Size)
}

// Levels returns the profile as a slice sorted by ascending price.
func (vp *VolumeProfile) Levels() []PriceLevel {
	levels := make([]PriceLevel, 0, len(vp.volumes))
	for price, vol := range vp.volumes {
		levels = append(levels, PriceLevel{Price: price, Volume: vol})
	}
	sort.Slice(levels, func(i, j int) bool { return levels[i].Price < levels[j].Price })
	return levels
}

// POC returns the price level with the highest traded volume
// (Point of Control). The zero value if no trades were added.
func (vp *VolumeProfile) POC() PriceLevel {
	var poc PriceLevel
	for price, vol := range vp.volumes {
		if vol > poc.Volume {
			poc = PriceLevel{Price: price, Volume: vol}
		}
	}
	return poc
}
