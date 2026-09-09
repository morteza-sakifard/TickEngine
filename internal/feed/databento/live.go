package databento

import (
	"fmt"
	"io"
	"sync"

	"github.com/morteza-sakifard/market-data-lab/internal/core"
	"github.com/morteza-sakifard/market-data-lab/internal/feed"
	"github.com/morteza-sakifard/market-data-lab/internal/marketdata"
)

var _ feed.Source = (*Live)(nil)

// Schema selects which historical decoder reads the bytes after
// START. The live gateway in this lab speaks CSV so the same
// NewDecoder / NewMBP10 / NewMBO path is reused. A later DBN
// reader can sit behind the same Source.
type Schema uint8

const (
	SchemaMBP1 Schema = iota + 1
	SchemaMBP10
	SchemaMBO
)

func (s Schema) name() string {
	switch s {
	case SchemaMBP10:
		return "mbp-10"
	case SchemaMBO:
		return "mbo"
	default:
		return "mbp-1"
	}
}

// LiveConfig is how to reach a gateway. Dial is required so this
// file never opens a network itself and never reads the wall clock.
// Reconnect is the live default: a dropped stream redials and asks
// for a snapshot so L2/L3 Reset instead of merging onto a stale book.
type LiveConfig struct {
	Dial             func() (io.ReadWriteCloser, error)
	Inst             core.Instrument
	Schema           Schema
	Key              string
	Symbol           string
	DisableReconnect bool
}

type liveMsg struct {
	ev  marketdata.Event
	err error
}

// Live is a feed.Source whose bytes come from a socket. The
// goroutine and the channel live only between the socket and Next;
// aggregation, orderflow, chart, and strategy still pull Events.
type Live struct {
	cfg  LiveConfig
	out  chan liveMsg
	done chan struct{}

	closeOnce sync.Once

	mu   sync.Mutex
	conn io.Closer
}

func NewLive(cfg LiveConfig) (*Live, error) {
	if cfg.Dial == nil {
		return nil, fmt.Errorf("databento: live Dial is required")
	}
	if cfg.Schema == 0 {
		cfg.Schema = SchemaMBP1
	}
	if cfg.Symbol == "" {
		cfg.Symbol = cfg.Inst.Symbol
	}
	if cfg.Key == "" {
		cfg.Key = "-"
	}
	l := &Live{
		cfg:  cfg,
		out:  make(chan liveMsg, 32),
		done: make(chan struct{}),
	}
	go l.loop()
	return l, nil
}

func (l *Live) Next(dst *marketdata.Event) error {
	if l == nil || dst == nil {
		return fmt.Errorf("databento: nil live source")
	}
	select {
	case <-l.done:
		return io.EOF
	case m, ok := <-l.out:
		if !ok {
			return io.EOF
		}
		if m.err != nil {
			return m.err
		}
		*dst = m.ev
		return nil
	}
}

func (l *Live) Close() error {
	if l == nil {
		return nil
	}
	l.closeOnce.Do(func() {
		close(l.done)
		l.mu.Lock()
		if l.conn != nil {
			_ = l.conn.Close()
			l.conn = nil
		}
		l.mu.Unlock()
	})
	return nil
}

func (l *Live) loop() {
	defer close(l.out)
	for {
		if l.stopped() {
			return
		}
		conn, err := l.cfg.Dial()
		if err != nil {
			if !l.sendErr(err) {
				return
			}
			if l.cfg.DisableReconnect {
				return
			}
			continue
		}
		l.mu.Lock()
		l.conn = conn
		l.mu.Unlock()
		err = l.session(conn)
		l.mu.Lock()
		if l.conn != nil {
			_ = l.conn.Close()
			l.conn = nil
		}
		l.mu.Unlock()
		if l.stopped() {
			return
		}
		if err == io.EOF && l.cfg.DisableReconnect {
			return
		}
	}
}

func (l *Live) session(conn io.ReadWriteCloser) error {
	if _, err := fmt.Fprintf(conn, "AUTH %s\nSUBSCRIBE %s %s\nSTART snapshot=1\n",
		l.cfg.Key, l.cfg.Schema.name(), l.cfg.Symbol); err != nil {
		return err
	}
	src, err := openSchema(conn, l.cfg.Inst, l.cfg.Schema)
	if err != nil {
		return err
	}
	armed := true
	for {
		if l.stopped() {
			return io.EOF
		}
		var ev marketdata.Event
		err := src.Next(&ev)
		if err != nil {
			return err
		}
		if armed && (ev.Kind == marketdata.KindQuote || ev.Kind == marketdata.KindBook) {
			ev.Flags |= marketdata.FlagSnapshot
			armed = false
		}
		if !l.send(ev) {
			return io.EOF
		}
	}
}

func openSchema(rc io.ReadCloser, inst core.Instrument, schema Schema) (feed.Source, error) {
	switch schema {
	case SchemaMBP10:
		return NewMBP10(rc, inst)
	case SchemaMBO:
		return NewMBO(rc, inst)
	default:
		return NewDecoder(rc, inst)
	}
}

func (l *Live) send(ev marketdata.Event) bool {
	select {
	case <-l.done:
		return false
	case l.out <- liveMsg{ev: ev}:
		return true
	}
}

func (l *Live) sendErr(err error) bool {
	select {
	case <-l.done:
		return false
	case l.out <- liveMsg{err: err}:
		return true
	}
}

func (l *Live) stopped() bool {
	select {
	case <-l.done:
		return true
	default:
		return false
	}
}
