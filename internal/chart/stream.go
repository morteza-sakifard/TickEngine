package chart

import (
	"encoding/json"
	"io"

	"github.com/morteza-sakifard/market-data-lab/internal/aggregation"
	"github.com/morteza-sakifard/market-data-lab/internal/core"
)

// Frame is one WebSocket message. The same View fields move here in
// pieces so the canvas can grow without receiving a new page. type
// is the only discriminator; this file has no socket and no
// goroutine — cmd/serve owns I/O.
const (
	FrameHello = "hello"
	FrameBar   = "bar"
	FrameDone  = "done"
	FrameError = "error"
	FramePlay  = "play"
	FramePause = "pause"
	FrameSpeed = "speed"
)

// Frame is a stream message. Hello carries Instrument and Header.
// Bar upserts Bars[Index] plus the aligned VWAP and CVD samples.
// Play / Pause / Speed travel client → server.
type Frame struct {
	Type       string           `json:"type"`
	Instrument *core.Instrument `json:"Instrument,omitempty"`
	Header     *Header          `json:"Header,omitempty"`
	Index      int              `json:"index"`
	Bar        *aggregation.Bar `json:"bar,omitempty"`
	VWAP       core.Ticks       `json:"vwap"`
	CVD        core.Ticks       `json:"cvd"`
	Speed      float64          `json:"speed,omitempty"`
	Error      string           `json:"error,omitempty"`
	Trade      *TapePrint       `json:"trade,omitempty"`
	Footprint  *BarFootprint    `json:"Footprint,omitempty"`
	Profile    *ProfileView     `json:"Profile,omitempty"`
	TPO        *TPOView         `json:"TPO,omitempty"`
}

// TapePrint is one Time & Sales row. Side is core.Side (1=bid buy,
// 2=ask sell) — the same discriminant the SVG footprint uses.
type TapePrint struct {
	TsEvent int64      `json:"TsEvent"`
	Px      core.Ticks `json:"Px"`
	Qty     core.Qty   `json:"Qty"`
	Side    core.Side  `json:"Side"`
}

// WriteFrame writes one JSON value. Two calls with the same Frame
// are byte-identical.
func WriteFrame(w io.Writer, f Frame) error {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	return enc.Encode(f)
}

// ReadFrame is the inverse of WriteFrame.
func ReadFrame(r io.Reader) (Frame, error) {
	var f Frame
	err := json.NewDecoder(r).Decode(&f)
	return f, err
}
