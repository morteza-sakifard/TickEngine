package marketdata

import (
	"reflect"
	"testing"
	"time"
	"unsafe"
)

// TestEventSize never fails; it only records the number. A future
// change that grows Event should be visible here instead of silently
// doubling allocation cost across a file with several million events.
// Compare this measured number against the 48-byte guess in
// docs/00-architecture.md 5.8 before trusting that guess for a storage
// decision.
func TestEventSize(t *testing.T) {
	t.Logf("unsafe.Sizeof(Event{}) = %d bytes", unsafe.Sizeof(Event{}))
	t.Logf("unsafe.Sizeof(Trade{}) = %d bytes", unsafe.Sizeof(Trade{}))
	t.Logf("unsafe.Sizeof(Quote{}) = %d bytes", unsafe.Sizeof(Quote{}))
}

func TestEventNoTimeTimeField(t *testing.T) {
	assertNoTimeTime(t, reflect.TypeOf(Event{}), "Event")
}

// assertNoTimeTime walks a struct type recursively (Event embeds Trade
// and Quote by value) and fails if any field, at any depth, is
// time.Time. Event's own timestamps must stay int64 Unix nanoseconds:
// see docs/00-architecture.md 5.2 for why a live time.Time is a heap
// pointer riding along on every event.
func assertNoTimeTime(t *testing.T, typ reflect.Type, path string) {
	t.Helper()
	if typ == reflect.TypeOf(time.Time{}) {
		t.Fatalf("%s is time.Time; marketdata events must store int64 unix-nanos instead", path)
	}
	if typ.Kind() != reflect.Struct {
		return
	}
	for i := 0; i < typ.NumField(); i++ {
		f := typ.Field(i)
		assertNoTimeTime(t, f.Type, path+"."+f.Name)
	}
}

func TestEventTimeAccessors(t *testing.T) {
	e := Event{
		TsEvent: 1_700_000_000_123_456_789,
		TsRecv:  1_700_000_000_123_500_000,
	}
	if got, want := e.EventTime().UnixNano(), e.TsEvent; got != want {
		t.Errorf("EventTime().UnixNano() = %d, want %d", got, want)
	}
	if got, want := e.RecvTime().UnixNano(), e.TsRecv; got != want {
		t.Errorf("RecvTime().UnixNano() = %d, want %d", got, want)
	}
	if loc := e.EventTime().Location(); loc != time.UTC {
		t.Errorf("EventTime().Location() = %v, want UTC", loc)
	}
}

func TestKindString(t *testing.T) {
	tests := []struct {
		k    Kind
		want string
	}{
		{KindTrade, "trade"},
		{KindQuote, "quote"},
		{KindStatus, "status"},
		{Kind(0), "unknown"},
	}
	for _, tt := range tests {
		if got := tt.k.String(); got != tt.want {
			t.Errorf("Kind(%d).String() = %q, want %q", uint8(tt.k), got, tt.want)
		}
	}
}
