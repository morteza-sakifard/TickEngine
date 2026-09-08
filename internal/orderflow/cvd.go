package orderflow

import (
	"github.com/morteza-sakifard/market-data-lab/internal/core"
	"github.com/morteza-sakifard/market-data-lab/internal/marketdata"
)

// CVD is the running sum of Delta. Divergence versus price — price
// makes a new high while CVD does not — is the raw order-flow tell.
// This type holds only the live total; per-bar snapshots are taken
// by the renderer, not stored here.
type CVD struct {
	delta Delta
}

func (c *CVD) OnTrade(ev *marketdata.Event) { c.delta.OnTrade(ev) }

func (c *CVD) Value() core.Qty { return c.delta.Value() }

func (c *CVD) Reset() { c.delta.Reset() }
