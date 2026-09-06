package main

// Databento record flags, from the DBN specification.
const (
	flagLast         uint8 = 0x80 // last message in the venue event for this instrument
	flagTOB          uint8 = 0x40 // top-of-book message, not an individual order
	flagSnapshot     uint8 = 0x20 // sourced from a replay or snapshot server
	flagMBP          uint8 = 0x10 // aggregated from individual orders
	flagBadTsRecv    uint8 = 0x08 // ts_recv is inaccurate
	flagMaybeBadBook uint8 = 0x04 // an unrecoverable channel gap was detected
)

var flagTable = []struct {
	mask uint8
	name string
}{
	{flagLast, "LAST"},
	{flagTOB, "TOB"},
	{flagSnapshot, "SNAPSHOT"},
	{flagMBP, "MBP"},
	{flagBadTsRecv, "BAD_TS_RECV"},
	{flagMaybeBadBook, "MAYBE_BAD_BOOK"},
}

// flagNames returns the set bits by name, highest bit first, or nil when
// no known bit is set. Order is fixed so output is deterministic.
func flagNames(raw uint8) []string {
	var out []string
	for _, f := range flagTable {
		if raw&f.mask != 0 {
			out = append(out, f.name)
		}
	}
	return out
}

// unknownFlagBits returns bits that are set but not in flagTable. A
// non-zero result means the spec gained a bit we do not model yet.
func unknownFlagBits(raw uint8) uint8 {
	var known uint8
	for _, f := range flagTable {
		known |= f.mask
	}
	return raw & ^known
}
