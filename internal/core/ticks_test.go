package core

import "testing"

func TestSideString(t *testing.T) {
	if SideBid.String() != "bid" || SideAsk.String() != "ask" || SideNone.String() != "none" {
		t.Fatalf("Side.String mismatch")
	}
}
