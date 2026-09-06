package marketdata

import (
	"fmt"
	"strings"
)

// Flags is the Databento record flags bitfield. It is shared by every
// schema this project reads: MBP-1 today, MBP-10 and MBO later (steps
// 24 and 26). See docs/00-architecture.md L1 and cmd/census/flags.go,
// which found these bits first while profiling the raw file in step 1.
type Flags uint8

const (
	FlagLast         Flags = 1 << 7 // 0x80 last message in this packet
	FlagTOB          Flags = 1 << 6 // 0x40 top-of-book only, not an individual order
	FlagSnapshot     Flags = 1 << 5 // 0x20 part of a snapshot, not a live delta
	FlagMBP          Flags = 1 << 4 // 0x10 aggregated from MBO
	FlagBadTsRecv    Flags = 1 << 3 // 0x08 ts_recv is unreliable for this record
	FlagMaybeBadBook Flags = 1 << 2 // 0x04 the book may be inconsistent (channel gap)
)

// knownFlags is every bit this project names. Databento reserves 0x02
// and 0x01 as of this schema version.
const knownFlags = FlagLast | FlagTOB | FlagSnapshot | FlagMBP | FlagBadTsRecv | FlagMaybeBadBook

var flagNames = []struct {
	bit  Flags
	name string
}{
	{FlagLast, "LAST"},
	{FlagTOB, "TOB"},
	{FlagSnapshot, "SNAPSHOT"},
	{FlagMBP, "MBP"},
	{FlagBadTsRecv, "BAD_TS_RECV"},
	{FlagMaybeBadBook, "MAYBE_BAD_BOOK"},
}

// String renders the set bits high-to-low, joined by "|", e.g.
// "LAST|SNAPSHOT". An empty Flags renders as "" since there is no
// "NONE" bit to name. A bit this project does not know about yet is
// rendered as hex (e.g. "0x02") instead of being silently dropped: the
// same forward-compatibility concern that produced unknownFlagBits in
// cmd/census/flags.go, promoted here into the shared type instead of
// living only in the census tool.
func (f Flags) String() string {
	var b strings.Builder
	for _, fl := range flagNames {
		if f&fl.bit == 0 {
			continue
		}
		if b.Len() > 0 {
			b.WriteByte('|')
		}
		b.WriteString(fl.name)
	}
	if rest := f &^ knownFlags; rest != 0 {
		if b.Len() > 0 {
			b.WriteByte('|')
		}
		fmt.Fprintf(&b, "0x%02X", uint8(rest))
	}
	return b.String()
}
