package strategy

import "github.com/morteza-sakifard/market-data-lab/internal/marketdata"

// Cache is the current market as of the last Observe. It stores
// values, not pointers: Next overwrites the Event buffer, and a
// strategy that kept *Event would see the next row's fields under
// the previous Sequence.
type Cache struct {
	last      marketdata.Event
	lastTrade marketdata.Trade
	lastQuote marketdata.Quote
	hasLast   bool
	hasTrade  bool
	hasQuote  bool
}

func (c *Cache) Last() (marketdata.Event, bool) {
	if c == nil || !c.hasLast {
		return marketdata.Event{}, false
	}
	return c.last, true
}

func (c *Cache) LastTrade() (marketdata.Trade, bool) {
	if c == nil || !c.hasTrade {
		return marketdata.Trade{}, false
	}
	return c.lastTrade, true
}

func (c *Cache) Quote() (marketdata.Quote, bool) {
	if c == nil || !c.hasQuote {
		return marketdata.Quote{}, false
	}
	return c.lastQuote, true
}

func (c *Cache) onEvent(ev *marketdata.Event) {
	if c == nil || ev == nil {
		return
	}
	c.last = *ev
	c.hasLast = true
	switch ev.Kind {
	case marketdata.KindTrade:
		c.lastTrade = ev.Trade
		c.hasTrade = true
	case marketdata.KindQuote:
		c.lastQuote = ev.Quote
		c.hasQuote = true
	}
}
