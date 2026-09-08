package main

import (
	"bufio"
	"bytes"
	"net/http"
	"strings"
	"testing"
)

func TestWSAcceptRFC6455(t *testing.T) {
	got := wsAccept("dGhlIHNhbXBsZSBub25jZQ==")
	want := "s3pPLMBiTxaQ9kYGzzhZRbK+xOo="
	if got != want {
		t.Fatalf("accept = %q, want %q", got, want)
	}
}

func TestWSFrameRoundTrip(t *testing.T) {
	var buf bytes.Buffer
	if err := wsWrite(&buf, wsText, []byte("hello"), false); err != nil {
		t.Fatal(err)
	}
	op, payload, err := wsRead(bufio.NewReader(&buf))
	if err != nil || op != wsText || string(payload) != "hello" {
		t.Fatalf("unmasked read op=%d p=%q err=%v", op, payload, err)
	}

	buf.Reset()
	if err := wsWriteMasked(&buf, wsText, []byte(`{"type":"play"}`)); err != nil {
		t.Fatal(err)
	}
	op, payload, err = wsRead(bufio.NewReader(&buf))
	if err != nil || string(payload) != `{"type":"play"}` {
		t.Fatalf("masked read p=%q err=%v", payload, err)
	}
}

func TestHeaderHasUpgrade(t *testing.T) {
	h := http.Header{"Connection": {"keep-alive, Upgrade"}}
	if !headerHas(h, "Connection", "Upgrade") {
		t.Fatal("expected Upgrade token")
	}
	if headerHas(h, "Connection", "websocket") {
		t.Fatal("websocket is not a Connection token here")
	}
}

func TestWSWriteRejectsMask(t *testing.T) {
	var buf bytes.Buffer
	if err := wsWrite(&buf, wsText, []byte("x"), true); err == nil || !strings.Contains(err.Error(), "masked") {
		t.Fatalf("want mask error, got %v", err)
	}
}
