package main

import (
	"bufio"
	"bytes"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/morteza-sakifard/market-data-lab/internal/chart"
	"github.com/morteza-sakifard/market-data-lab/internal/core"
	"github.com/morteza-sakifard/market-data-lab/internal/marketdata"
	"github.com/morteza-sakifard/market-data-lab/internal/session"
)

func wsWriteMasked(w io.Writer, op byte, payload []byte) error {
	key := [4]byte{1, 2, 3, 4}
	var hdr [6]byte
	hdr[0] = 0x80 | op
	if len(payload) >= 126 {
		return io.ErrShortBuffer
	}
	hdr[1] = 0x80 | byte(len(payload))
	copy(hdr[2:6], key[:])
	if _, err := w.Write(hdr[:]); err != nil {
		return err
	}
	out := make([]byte, len(payload))
	for i, b := range payload {
		out[i] = b ^ key[i%4]
	}
	_, err := w.Write(out)
	return err
}

func dialReplay(t *testing.T, ts *httptest.Server, session string) (net.Conn, *bufio.Reader) {
	t.Helper()
	u, err := url.Parse(ts.URL + "/api/replay?session=" + session)
	if err != nil {
		t.Fatal(err)
	}
	c, err := net.Dial("tcp", u.Host)
	if err != nil {
		t.Fatal(err)
	}
	_, err = c.Write([]byte(
		"GET " + u.RequestURI() + " HTTP/1.1\r\n" +
			"Host: " + u.Host + "\r\n" +
			"Upgrade: websocket\r\n" +
			"Connection: Upgrade\r\n" +
			"Sec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==\r\n" +
			"Sec-WebSocket-Version: 13\r\n\r\n"))
	if err != nil {
		c.Close()
		t.Fatal(err)
	}
	br := bufio.NewReader(c)
	line, err := br.ReadString('\n')
	if err != nil {
		c.Close()
		t.Fatal(err)
	}
	if !bytes.Contains([]byte(line), []byte("101")) {
		c.Close()
		t.Fatalf("upgrade status %q", line)
	}
	for {
		h, err := br.ReadString('\n')
		if err != nil {
			c.Close()
			t.Fatal(err)
		}
		if h == "\r\n" {
			break
		}
	}
	return c, br
}

func readFrame(t *testing.T, br *bufio.Reader) chart.Frame {
	t.Helper()
	op, payload, err := wsRead(br)
	if err != nil {
		t.Fatal(err)
	}
	if op != wsText {
		t.Fatalf("op = %d, want text", op)
	}
	f, err := chart.ReadFrame(bytes.NewReader(payload))
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func sendCtrl(t *testing.T, c net.Conn, typ string, speed float64) {
	t.Helper()
	var buf bytes.Buffer
	if err := chart.WriteFrame(&buf, chart.Frame{Type: typ, Speed: speed}); err != nil {
		t.Fatal(err)
	}
	if err := wsWriteMasked(c, wsText, buf.Bytes()); err != nil {
		t.Fatal(err)
	}
}

func TestReplayUpgradeRejectedWithoutWS(t *testing.T) {
	h := newMux(testServer(t))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/api/replay?session=RTH", nil))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status %d, want 400", rec.Code)
	}
}

func TestReplayStreamGrowsBars(t *testing.T) {
	ts := httptest.NewServer(newMux(testServer(t)))
	defer ts.Close()

	c, br := dialReplay(t, ts, "RTH")
	defer c.Close()

	hello := readFrame(t, br)
	if hello.Type != chart.FrameHello || hello.Header == nil || hello.Header.Session != session.RTH {
		t.Fatalf("hello = %+v", hello)
	}
	if hello.Instrument == nil || hello.Instrument.Symbol != "ESZ5" {
		t.Fatal("hello missing instrument")
	}

	sendCtrl(t, c, chart.FrameSpeed, 0)
	sendCtrl(t, c, chart.FramePlay, 0)

	var bars int
	deadline := time.Now().Add(2 * time.Second)
	for bars < 1 {
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for bar/done")
		}
		_ = c.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
		f := readFrame(t, br)
		switch f.Type {
		case chart.FrameBar:
			if f.Bar == nil || f.Bar.Open != 26800 {
				t.Fatalf("bar = %+v", f.Bar)
			}
			if f.Index != 0 {
				t.Fatalf("index = %d, want 0 (one 5m bucket)", f.Index)
			}
			if f.Footprint == nil || f.Trade == nil {
				t.Fatal("bar frame missing footprint or tape print")
			}
			if f.Profile == nil || f.TPO == nil {
				t.Fatal("first bar should carry profile and TPO snapshots")
			}
			bars++
		case chart.FrameDone:
			if bars < 1 {
				t.Fatal("done before any bar")
			}
			return
		case chart.FrameError:
			t.Fatalf("error frame: %s", f.Error)
		}
	}
	for {
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for done")
		}
		_ = c.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
		f := readFrame(t, br)
		if f.Type == chart.FrameDone {
			return
		}
		if f.Type == chart.FrameError {
			t.Fatalf("error frame: %s", f.Error)
		}
	}
}

func TestReplayPauseHoldsSecondEvent(t *testing.T) {
	cal := esCal(t)
	day := time.Date(2025, time.September, 23, 0, 0, 0, 0, cal.Location)
	t0 := time.Date(2025, time.September, 23, 8, 30, 0, 0, cal.Location)
	t1 := t0.Add(time.Second)
	s := &server{
		inst:     core.ESZ5(),
		cal:      cal,
		date:     day,
		interval: 5 * time.Minute,
		defSess:  "RTH",
		trades: []marketdata.Event{
			{Kind: marketdata.KindTrade, TsEvent: t0.UnixNano(), TsRecv: 1e9, Trade: marketdata.Trade{Px: 26800, Qty: 1, Aggressor: core.SideBid}},
			{Kind: marketdata.KindTrade, TsEvent: t1.UnixNano(), TsRecv: 2e9, Trade: marketdata.Trade{Px: 26804, Qty: 1, Aggressor: core.SideAsk}},
		},
		views: map[session.Session]chart.View{},
	}
	ts := httptest.NewServer(newMux(s))
	defer ts.Close()

	c, br := dialReplay(t, ts, "RTH")
	defer c.Close()
	_ = readFrame(t, br) // hello

	sendCtrl(t, c, chart.FrameSpeed, 1)
	sendCtrl(t, c, chart.FramePlay, 0)

	_ = c.SetReadDeadline(time.Now().Add(2 * time.Second))
	first := readFrame(t, br)
	if first.Type != chart.FrameBar {
		t.Fatalf("first after play = %s", first.Type)
	}
	sendCtrl(t, c, chart.FramePause, 0)

	_ = c.SetReadDeadline(time.Now().Add(150 * time.Millisecond))
	op, payload, err := wsRead(br)
	if err == nil {
		f, _ := chart.ReadFrame(bytes.NewReader(payload))
		t.Fatalf("paused replay still sent op=%d type=%s", op, f.Type)
	}

	sendCtrl(t, c, chart.FramePlay, 0)
	_ = c.SetReadDeadline(time.Now().Add(2 * time.Second))
	second := readFrame(t, br)
	if second.Type != chart.FrameBar && second.Type != chart.FrameDone {
		t.Fatalf("after resume = %s", second.Type)
	}
}

func TestReplayETHEmptyIsErrorFrame(t *testing.T) {
	ts := httptest.NewServer(newMux(testServer(t)))
	defer ts.Close()
	c, br := dialReplay(t, ts, "ETH")
	defer c.Close()
	hello := readFrame(t, br)
	if hello.Type != chart.FrameHello {
		t.Fatalf("hello = %s", hello.Type)
	}
	errf := readFrame(t, br)
	if errf.Type != chart.FrameError || errf.Error == "" {
		t.Fatalf("want error frame, got %+v", errf)
	}
}
