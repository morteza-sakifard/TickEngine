package session

import "testing"

func TestSetZeroMeansAllOpenSessions(t *testing.T) {
	var z Set
	if !z.Contains(RTH) || !z.Contains(ETH) {
		t.Fatal("zero Set must contain RTH and ETH")
	}
	if z.Contains(Closed) {
		t.Fatal("zero Set must not contain Closed")
	}
	if SetRTH.Contains(ETH) {
		t.Fatal("SetRTH must not contain ETH")
	}
}
