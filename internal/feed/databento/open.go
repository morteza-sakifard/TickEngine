package databento

import (
	"fmt"
	"io"
	"strings"

	"github.com/morteza-sakifard/market-data-lab/internal/core"
	"github.com/morteza-sakifard/market-data-lab/internal/feed"
)

// Schema selects which CSV decoder Open uses. Zero means MBP-1.
type Schema uint8

const (
	SchemaMBP1 Schema = iota + 1
	SchemaMBP10
	SchemaMBO
)

func (s Schema) String() string {
	switch s {
	case SchemaMBP10:
		return "mbp-10"
	case SchemaMBO:
		return "mbo"
	default:
		return "mbp-1"
	}
}

func ParseSchema(s string) (Schema, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "mbp-1", "mbp1":
		return SchemaMBP1, nil
	case "mbp-10", "mbp10":
		return SchemaMBP10, nil
	case "mbo":
		return SchemaMBO, nil
	default:
		return 0, fmt.Errorf("databento: unknown schema %q", s)
	}
}

// DetectSchema looks at CSV header names. order_id without bid_px_00
// is MBO. bid_px_09 is MBP-10. bid_px_00 is MBP-1.
func DetectSchema(header []string) (Schema, error) {
	has := make(map[string]struct{}, len(header))
	for _, name := range header {
		has[strings.TrimSpace(name)] = struct{}{}
	}
	_, orderID := has["order_id"]
	_, bid0 := has["bid_px_00"]
	_, bid9 := has["bid_px_09"]
	if orderID && !bid0 {
		return SchemaMBO, nil
	}
	if bid9 {
		return SchemaMBP10, nil
	}
	if bid0 {
		return SchemaMBP1, nil
	}
	return 0, fmt.Errorf("databento: cannot detect schema from header")
}

// Open is the historical door. CLI and Live both call it. schema 0
// is MBP-1. On error rc is still open.
func Open(rc io.ReadCloser, inst core.Instrument, schema Schema) (feed.Source, error) {
	if rc == nil {
		return nil, fmt.Errorf("databento: nil reader")
	}
	switch schema {
	case SchemaMBP10:
		return NewMBP10(rc, inst)
	case SchemaMBO:
		return NewMBO(rc, inst)
	case 0, SchemaMBP1:
		return NewDecoder(rc, inst)
	default:
		return nil, fmt.Errorf("databento: unknown schema %d", schema)
	}
}
