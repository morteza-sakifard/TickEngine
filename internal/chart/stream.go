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
