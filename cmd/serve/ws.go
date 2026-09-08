package main

import (
	"bufio"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
)

const wsMagic = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"

const (
	wsText  = 1
	wsClose = 8
	wsPing  = 9
	wsPong  = 10
)

const wsMax = 1 << 20

// wsConn is a text-frame WebSocket after a successful upgrade.
// Writes are serialized; the engine goroutine and the control
// reader both need the socket.
type wsConn struct {
	raw net.Conn
	r   *bufio.Reader
	mu  sync.Mutex
}

func upgradeWS(w http.ResponseWriter, r *http.Request) (*wsConn, error) {
	if !headerHas(r.Header, "Connection", "Upgrade") || !strings.EqualFold(r.Header.Get("Upgrade"), "websocket") {
		return nil, fmt.Errorf("serve: not a websocket upgrade")
	}
	key := r.Header.Get("Sec-WebSocket-Key")
	if key == "" {
		return nil, fmt.Errorf("serve: missing Sec-WebSocket-Key")
	}
	hj, ok := w.(http.Hijacker)
	if !ok {
		return nil, fmt.Errorf("serve: hijack not supported")
	}
	raw, bufrw, err := hj.Hijack()
	if err != nil {
		return nil, err
	}
	accept := wsAccept(key)
	_, err = fmt.Fprintf(bufrw, "HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Accept: %s\r\n\r\n", accept)
	if err != nil {
		raw.Close()
		return nil, err
	}
	if err := bufrw.Flush(); err != nil {
		raw.Close()
		return nil, err
	}
	return &wsConn{raw: raw, r: bufrw.Reader}, nil
}

func wsAccept(key string) string {
	h := sha1.New()
	io.WriteString(h, key)
	io.WriteString(h, wsMagic)
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

func headerHas(h http.Header, key, token string) bool {
	for _, part := range strings.Split(h.Get(key), ",") {
		if strings.EqualFold(strings.TrimSpace(part), token) {
			return true
		}
	}
	return false
}

func (c *wsConn) Close() error {
	if c == nil || c.raw == nil {
		return nil
	}
	return c.raw.Close()
}

func (c *wsConn) WriteText(p []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return wsWrite(c.raw, wsText, p, false)
}

func (c *wsConn) ReadText() ([]byte, error) {
	for {
		op, payload, err := wsRead(c.r)
		if err != nil {
			return nil, err
		}
		switch op {
		case wsText:
			return payload, nil
		case wsClose:
			return nil, io.EOF
		case wsPing:
			c.mu.Lock()
			err := wsWrite(c.raw, wsPong, payload, false)
			c.mu.Unlock()
			if err != nil {
				return nil, err
			}
		}
	}
}

func wsWrite(w io.Writer, op byte, payload []byte, mask bool) error {
	if len(payload) > wsMax {
		return fmt.Errorf("serve: websocket payload too large")
	}
	var hdr [14]byte
	hdr[0] = 0x80 | op
	n := 2
	l := len(payload)
	switch {
	case l < 126:
		hdr[1] = byte(l)
	case l < 1<<16:
		hdr[1] = 126
		binary.BigEndian.PutUint16(hdr[2:4], uint16(l))
		n = 4
	default:
		hdr[1] = 127
		binary.BigEndian.PutUint64(hdr[2:10], uint64(l))
		n = 10
	}
	if mask {
		return errors.New("serve: server frames are not masked")
	}
	if _, err := w.Write(hdr[:n]); err != nil {
		return err
	}
	_, err := w.Write(payload)
	return err
}

func wsRead(r *bufio.Reader) (byte, []byte, error) {
	h0, err := r.ReadByte()
	if err != nil {
		return 0, nil, err
	}
	h1, err := r.ReadByte()
	if err != nil {
		return 0, nil, err
	}
	if h0&0x80 == 0 {
		return 0, nil, fmt.Errorf("serve: fragmented websocket frames are not supported")
	}
	op := h0 & 0x0f
	masked := h1&0x80 != 0
	l := int(h1 & 0x7f)
	switch l {
	case 126:
		var ext [2]byte
		if _, err := io.ReadFull(r, ext[:]); err != nil {
			return 0, nil, err
		}
		l = int(binary.BigEndian.Uint16(ext[:]))
	case 127:
		var ext [8]byte
		if _, err := io.ReadFull(r, ext[:]); err != nil {
			return 0, nil, err
		}
		n64 := binary.BigEndian.Uint64(ext[:])
		if n64 > wsMax {
			return 0, nil, fmt.Errorf("serve: websocket payload too large")
		}
		l = int(n64)
	}
	if l > wsMax {
		return 0, nil, fmt.Errorf("serve: websocket payload too large")
	}
	var key [4]byte
	if masked {
		if _, err := io.ReadFull(r, key[:]); err != nil {
			return 0, nil, err
		}
	}
	payload := make([]byte, l)
	if _, err := io.ReadFull(r, payload); err != nil {
		return 0, nil, err
	}
	if masked {
		for i := range payload {
			payload[i] ^= key[i%4]
		}
	}
	return op, payload, nil
}
