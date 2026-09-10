package aggregation

import (
	"fmt"
	"time"

	"github.com/morteza-sakifard/TickEngine/internal/core"
	"github.com/morteza-sakifard/TickEngine/internal/marketdata"
	"github.com/morteza-sakifard/TickEngine/internal/session"
)

type Anchor uint8

const (
	AnchorSessionOpen   Anchor = iota // open of the session Classify assigns
	AnchorRTHOpen                     // RTH open of that trading date
	AnchorMidnightLocal               // civil midnight in Location
	AnchorUTCEpoch                    // Unix-epoch floor; matches Truncate for 5m
)

type Kind uint8

const (
	KindTime Kind = iota + 1
	KindTick
	KindVolume
	KindRange
)

type BarSpec struct {
	Kind     Kind
	Interval time.Duration // KindTime
	Count    int64         // KindTick, KindVolume
	Band     core.Ticks    // KindRange
	Anchor   Anchor        // KindTime; zero is AnchorSessionOpen
	Location *time.Location
	Sessions session.Set
}

func (s BarSpec) loc(cal session.Calendar) *time.Location {
	if s.Location != nil {
		return s.Location
	}
	return cal.Location
}

// Aggregator turns a stream of Events into Bars. Add ignores quotes
// and any trade Classify puts in a session Sessions does not contain.
// Bars is a read-only snapshot and must not close the open bar.
type Aggregator interface {
	Add(ev *marketdata.Event)
	Bars() []Bar
	Flush()
}

func New(spec BarSpec, cal session.Calendar) (Aggregator, error) {
	if cal.Location == nil {
		return nil, fmt.Errorf("aggregation: calendar location is nil")
	}
	if spec.Location == nil {
		spec.Location = cal.Location
	}
	switch spec.Kind {
	case KindTime:
		if spec.Interval <= 0 {
			return nil, fmt.Errorf("aggregation: KindTime requires Interval > 0")
		}
		return &timeBuilder{spec: spec, cal: cal}, nil
	case KindTick:
		if spec.Count <= 0 {
			return nil, fmt.Errorf("aggregation: KindTick requires Count > 0")
		}
		return &tickBuilder{spec: spec, cal: cal}, nil
	case KindVolume:
		if spec.Count <= 0 {
			return nil, fmt.Errorf("aggregation: KindVolume requires Count > 0")
		}
		return &volumeBuilder{spec: spec, cal: cal}, nil
	case KindRange:
		if spec.Band <= 0 {
			return nil, fmt.Errorf("aggregation: KindRange requires Band > 0")
		}
		return &rangeBuilder{spec: spec, cal: cal}, nil
	default:
		return nil, fmt.Errorf("aggregation: unknown Kind %d", spec.Kind)
	}
}
