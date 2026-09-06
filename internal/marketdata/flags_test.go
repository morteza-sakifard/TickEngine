package marketdata

import "testing"

func TestFlagsString(t *testing.T) {
	tests := []struct {
		name string
		f    Flags
		want string
	}{
		{name: "none", f: 0, want: ""},
		{name: "single bit", f: FlagLast, want: "LAST"},
		{name: "two bits print high to low regardless of OR order", f: FlagSnapshot | FlagLast, want: "LAST|SNAPSHOT"},
		{
			name: "row 1 of the real file (see docs/steps/01-data-census.md)",
			f:    FlagLast | FlagSnapshot | FlagBadTsRecv,
			want: "LAST|SNAPSHOT|BAD_TS_RECV",
		},
		{
			name: "all six known bits",
			f:    FlagLast | FlagTOB | FlagSnapshot | FlagMBP | FlagBadTsRecv | FlagMaybeBadBook,
			want: "LAST|TOB|SNAPSHOT|MBP|BAD_TS_RECV|MAYBE_BAD_BOOK",
		},
		{name: "reserved low bit is not swallowed", f: Flags(0x01), want: "0x01"},
		{name: "known bit plus a reserved bit", f: FlagLast | Flags(0x02), want: "LAST|0x02"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.f.String(); got != tt.want {
				t.Errorf("Flags(0x%02X).String() = %q, want %q", uint8(tt.f), got, tt.want)
			}
		})
	}
}
