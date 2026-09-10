package marketdata

import "github.com/morteza-sakifard/TickEngine/internal/core"

// Trade is a single execution. Aggressor is the side of the market
// order that triggered it, not the side of the resting order it hit:
// SideBid means a buyer lifted the ask, SideAsk means a seller hit the
// bid (see core.Side). Step 5 validates this against Databento's own
// side column and the quote in force just before the trade.
type Trade struct {
	Px        core.Ticks
	Qty       core.Qty
	Aggressor core.Side
}
