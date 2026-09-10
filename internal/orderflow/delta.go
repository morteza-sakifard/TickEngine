package orderflow

import (
	"github.com/morteza-sakifard/TickEngine/internal/core"
	"github.com/morteza-sakifard/TickEngine/internal/marketdata"
)

// Delta is aggressive buy volume minus aggressive sell volume.
// SideNone is ignored: there is no aggressor to attribute.
type Delta struct {
	buy, sell core.Qty
}

func (d *Delta) OnTrade(ev *marketdata.Event) {
	if ev == nil || ev.Kind != marketdata.KindTrade {
		return
	}
	switch ev.Trade.Aggressor {
	case core.SideBid:
		d.buy += ev.Trade.Qty
	case core.SideAsk:
		d.sell += ev.Trade.Qty
	}
}

func (d *Delta) Value() core.Qty { return d.buy - d.sell }

func (d *Delta) Reset() { *d = Delta{} }
