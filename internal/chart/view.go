package chart

import (
	"time"

	"github.com/morteza-sakifard/TickEngine/internal/aggregation"
	"github.com/morteza-sakifard/TickEngine/internal/core"
	"github.com/morteza-sakifard/TickEngine/internal/session"
)

// View is everything needed to draw one page. It has no reference to
// a feed, a clock, or the replay engine — RenderSVG and WriteJSON
// are both pure functions of this value. That is why a golden-file
// test can lock the pixels and the browser can draw the same page:
// same View, same facts.
//
// Overlays and Panels are drawn when present. Empty slices keep the
// step-8 candle layout, so data/test/golden/candles_basic.svg stays
// valid.
type View struct {
	Instrument core.Instrument
	Header     Header
	Bars       []aggregation.Bar
	Overlays   []Series
	Panels     []Panel
	Profile    *ProfileView
	Footprint  *FootprintView
	TPO        *TPOView
	Marks      []Mark
	Heatmap    *HeatmapView
	DOM        *DOMView // live ladder; nil keeps the step-8 width
}

// TPOView is the market-profile letter ladder. Levels are sorted by
// price; Letters is already A…Z/a. A nil TPO leaves the step-8
// layout unchanged.
type TPOView struct {
	Levels        []TPOViewLevel
	POC           core.Ticks
	IBLow, IBHigh core.Ticks
	HasIB         bool
	PeriodCount   int
}

// TPOViewLevel is one price row of letters.
type TPOViewLevel struct {
	Price   core.Ticks
	Letters string
	Single  bool
}

// FootprintView is the per-bar bid×ask ladder. Bars is aligned with
// View.Bars by index; a shorter slice skips extra candles. A nil
// Footprint leaves the candle bodies in place so the step-8 golden
// stays valid.
type FootprintView struct {
	TicksPerRow int
	Bars        []BarFootprint
}

// BarFootprint is one column of cells, already grouped and with
// imbalance marks resolved. RenderSVG does not recompute ratios.
type BarFootprint struct {
	Levels []FootprintLevel
	Imbs   []FootprintImb
}

// FootprintLevel is one grouped price row of a bar.
type FootprintLevel struct {
	Price     core.Ticks
	Buy, Sell core.Qty
}

// FootprintImb marks one cell. Stacked is true when that cell sits
// in a run of at least three same-direction imbalances.
type FootprintImb struct {
	Price   core.Ticks
	Dir     core.Side
	Stacked bool
}

// ProfileView is the side histogram for a session volume profile.
// Levels must already be sorted by Price; RenderSVG does not range a
// map. A nil Profile, or one with no Levels, leaves the step-8
// layout unchanged.
type ProfileView struct {
	Levels        []ProfileLevel
	POC, VAH, VAL core.Ticks
}

// ProfileLevel is one horizontal bar of a ProfileView.
type ProfileLevel struct {
	Price  core.Ticks
	Volume core.Qty
}

// Header is the title strip. Empty Symbol falls back to
// Instrument.Symbol; zero TradingDate falls back to the first bar.
type Header struct {
	Symbol      string
	TradingDate time.Time
	Session     session.Session
}

// Series is a polyline aligned with View.Bars by index (VWAP and
// similar). Length may be shorter than Bars; extra bars are skipped.
type Series struct {
	Name   string
	Values []core.Ticks
}

// Panel is a secondary plot under the price pane (CVD, volume, delta).
type Panel struct {
	Name   string
	Series []Series
}

// Options is how to draw, not what to draw. Location is the calendar
// timezone for axis labels — UTC would print 13:30 for an 08:30 CT
// RTH open, which is the bug this field exists to prevent. A nil
// Location uses each bar's own Start location.
type Options struct {
	Width, Height int
	Location      *time.Location
}
