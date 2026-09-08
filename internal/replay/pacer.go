package replay

import (
	"bufio"
	"os"
	"time"
)

// Pacer stretches or compresses the wall gap between events. It does
// not change which events fire or what the ReplayClock reads — only
// how long the process waits. Speed 0 means no wait (as fast as the
// loop can go). Speed 1 is one wall second per TsRecv second. Speed
// 100 is a hundred times faster.
//
// Step waits for Hold after each Next except the first, not after
// the last event. Hold is a line from stdin when nil; tests replace it.
type Pacer struct {
	Speed float64
	Step  bool
	Sleep func(time.Duration)
	Hold  func()
}

// Between sleeps for deltaNS / Speed wall time. A non-positive
// Speed or delta is a no-op.
func (p *Pacer) Between(deltaNS int64) {
	if p == nil || p.Speed <= 0 || deltaNS <= 0 {
		return
	}
	d := time.Duration(float64(deltaNS) / p.Speed)
	if d <= 0 {
		return
	}
	sleep := p.Sleep
	if sleep == nil {
		sleep = time.Sleep
	}
	sleep(d)
}

// Await blocks when Step is set. Engine calls it after a successful
// Next for every event except the first, so the last print does not
// wait for an extra Enter.
func (p *Pacer) Await() {
	if p == nil || !p.Step {
		return
	}
	hold := p.Hold
	if hold == nil {
		hold = defaultHold
	}
	hold()
}

func defaultHold() {
	_, _ = bufio.NewReader(os.Stdin).ReadString('\n')
}
