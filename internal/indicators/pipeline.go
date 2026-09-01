package indicators

import "github.com/morteza-sakifard/market-data-lab/internal/trade"

// Pipeline runs every indicator over a stream of trades in a single pass.
type Pipeline struct {
	VWAP          VWAP
	Delta         Delta
	CVD           CVD
	VolumeProfile VolumeProfile
	Footprint     Footprint
	TPO           TPO
}

// Add incorporates one trade into every indicator in the pipeline.
func (p *Pipeline) Add(t trade.Trade) {
	p.VWAP.Add(t)
	p.Delta.Add(t)
	p.CVD.Add(t)
	p.VolumeProfile.Add(t)
	p.Footprint.Add(t)
	p.TPO.Add(t)
}
