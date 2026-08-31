package mbp1

import (
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
	"time"
)

var requiredColumns = []string{
	"ts_recv", "ts_event",
	"action", "side",
	"price", "size",
	"sequence", "symbol",
}

type Reader struct {
	csv *csv.Reader
	col map[string]int
	row uint64
}

func NewReader(r io.Reader) (*Reader, error) {
	cr := csv.NewReader(r)
	cr.ReuseRecord = true

	header, err := cr.Read()
	if err != nil {
		return nil, fmt.Errorf("mbp1: read header: %w", err)
	}

	col := make(map[string]int, len(header))
	for i, name := range header {
		col[name] = i
	}

	for _, name := range requiredColumns {
		if _, ok := col[name]; !ok {
			return nil, fmt.Errorf("mbp1: missing required column: %s", name)
		}
	}

	return &Reader{
		csv: cr,
		col: col,
	}, nil
}

func (r *Reader) Row() uint64 {
	return r.row
}

func (r *Reader) Read() (Record, error) {
	row, err := r.csv.Read()
	if err != nil {
		return Record{}, err
	}
	r.row++

	get := func(name string) string { return row[r.col[name]] }

	rec := Record{
		Action: Action(get("action")),
		Side:   Side(get("side")),
		Symbol: get("symbol"),
	}

	var perr error
	setErr := func(field string, err error) {
		if err != nil && perr == nil {
			perr = fmt.Errorf("mbp1: row %d: field %s: %w", r.row, field, err)
		}
	}

	rec.TsRecv, err = time.Parse(time.RFC3339Nano, get("ts_recv"))
	setErr("ts_recv", err)
	rec.TsEvent, err = time.Parse(time.RFC3339Nano, get("ts_event"))
	setErr("ts_event", err)

	rec.Price, err = strconv.ParseFloat(get("price"), 64)
	setErr("price", err)
	rec.Size, err = parseUint32(get("size"))
	setErr("size", err)

	rec.Sequence, err = parseUint32(get("sequence"))
	setErr("sequence", err)

	if perr != nil {
		return Record{}, perr
	}
	return rec, nil
}

func parseUint32(s string) (uint32, error) {
	v, err := strconv.ParseUint(s, 10, 32)
	return uint32(v), err
}
