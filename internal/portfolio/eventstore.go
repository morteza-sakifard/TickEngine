package portfolio

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/morteza-sakifard/TickEngine/internal/core"
	"github.com/morteza-sakifard/TickEngine/internal/execution"
)

const storeVersion = "v1"

type RecKind uint8

const (
	RecOrder RecKind = iota + 1
	RecFill
	RecPosition
)

func (k RecKind) String() string {
	switch k {
	case RecOrder:
		return "order"
	case RecFill:
		return "fill"
	case RecPosition:
		return "position"
	default:
		return "unknown"
	}
}

// Record is one state change. Market prints do not belong here —
// they already live on the tape. Seq is assigned by the store.
type Record struct {
	Seq    uint64
	Ts     int64
	Kind   RecKind
	Order  execution.Order
	Status execution.Status
	Fill   execution.Fill
	Pos    Position
}

// Store is an append-only log. Persistence is WriteTo / Load on a
// handle the runner owns. This package does not open files.
type Store struct {
	seq  uint64
	recs []Record
}

func NewStore() *Store {
	return &Store{}
}

func (s *Store) Len() int {
	if s == nil {
		return 0
	}
	return len(s.recs)
}

func (s *Store) Records() []Record {
	if s == nil || len(s.recs) == 0 {
		return nil
	}
	out := make([]Record, len(s.recs))
	copy(out, s.recs)
	return out
}

func (s *Store) AppendOrder(ts int64, o execution.Order, st execution.Status) error {
	return s.append(Record{Ts: ts, Kind: RecOrder, Order: o, Status: st})
}

func (s *Store) AppendFill(ts int64, f execution.Fill) error {
	if f.Qty <= 0 {
		return fmt.Errorf("portfolio: fill qty must be positive")
	}
	return s.append(Record{Ts: ts, Kind: RecFill, Fill: f})
}

func (s *Store) AppendPosition(ts int64, p Position) error {
	return s.append(Record{Ts: ts, Kind: RecPosition, Pos: p})
}

func (s *Store) append(r Record) error {
	if s == nil {
		return fmt.Errorf("portfolio: nil store")
	}
	if r.Kind != RecOrder && r.Kind != RecFill && r.Kind != RecPosition {
		return fmt.Errorf("portfolio: unknown record kind %d", r.Kind)
	}
	s.seq++
	r.Seq = s.seq
	s.recs = append(s.recs, r)
	return nil
}

// Replay applies fills in log order. RecPosition is a checkpoint
// for tests, not a second source of qty.
func (s *Store) Replay(inst core.Instrument) Position {
	var p Position
	if s == nil {
		return p
	}
	for _, r := range s.recs {
		if r.Kind == RecFill {
			p.Apply(inst, r.Fill)
		}
	}
	return p
}

func (s *Store) LastPosition() (Position, bool) {
	if s == nil {
		return Position{}, false
	}
	for i := len(s.recs) - 1; i >= 0; i-- {
		if s.recs[i].Kind == RecPosition {
			return s.recs[i].Pos, true
		}
	}
	return Position{}, false
}

func (s *Store) Fills() []execution.Fill {
	if s == nil {
		return nil
	}
	var out []execution.Fill
	for _, r := range s.recs {
		if r.Kind == RecFill {
			out = append(out, r.Fill)
		}
	}
	return out
}

// WorkingOrders are accepts that have not been filled, canceled,
// or rejected. Walk is the append order, never a map range.
func (s *Store) WorkingOrders() []execution.Order {
	if s == nil {
		return nil
	}
	var order []execution.Order
	idx := make(map[execution.OrderID]int)
	dead := make(map[execution.OrderID]struct{})
	for _, r := range s.recs {
		switch r.Kind {
		case RecOrder:
			id := r.Order.ID
			if id == 0 {
				continue
			}
			if r.Status == execution.StatusCanceled || r.Status == execution.StatusRejected {
				dead[id] = struct{}{}
				continue
			}
			if _, ok := idx[id]; !ok {
				idx[id] = len(order)
				order = append(order, r.Order)
			}
		case RecFill:
			if r.Fill.OrderID != 0 {
				dead[r.Fill.OrderID] = struct{}{}
			}
		}
	}
	var out []execution.Order
	for _, o := range order {
		if _, ok := dead[o.ID]; !ok {
			out = append(out, o)
		}
	}
	return out
}

func (s *Store) LastID() execution.OrderID {
	if s == nil {
		return 0
	}
	var max execution.OrderID
	for _, r := range s.recs {
		if r.Order.ID > max {
			max = r.Order.ID
		}
		if r.Fill.OrderID > max {
			max = r.Fill.OrderID
		}
	}
	return max
}

func (s *Store) LastTs() int64 {
	if s == nil || len(s.recs) == 0 {
		return 0
	}
	return s.recs[len(s.recs)-1].Ts
}

func (s *Store) WriteTo(w io.Writer) error {
	if s == nil {
		return fmt.Errorf("portfolio: nil store")
	}
	if w == nil {
		return fmt.Errorf("portfolio: nil writer")
	}
	if _, err := fmt.Fprintf(w, "%s\n", storeVersion); err != nil {
		return err
	}
	for _, r := range s.recs {
		if err := writeRec(w, r); err != nil {
			return err
		}
	}
	return nil
}

func writeRec(w io.Writer, r Record) error {
	switch r.Kind {
	case RecOrder:
		_, err := fmt.Fprintf(w, "o %d %d %d %d %d %d %d %d %d\n",
			r.Seq, r.Ts, r.Order.ID, r.Order.Instrument,
			r.Order.Side, r.Order.Qty, r.Order.Kind, r.Order.Px, r.Status)
		return err
	case RecFill:
		_, err := fmt.Fprintf(w, "f %d %d %d %d %d %d %d\n",
			r.Seq, r.Ts, r.Fill.OrderID, r.Fill.Side, r.Fill.Px, r.Fill.Qty, r.Fill.FeeCents)
		return err
	case RecPosition:
		_, err := fmt.Fprintf(w, "p %d %d %d %d %d\n",
			r.Seq, r.Ts, r.Pos.Qty, r.Pos.AvgPx, r.Pos.Realized)
		return err
	default:
		return fmt.Errorf("portfolio: unknown record kind %d", r.Kind)
	}
}

// Load replaces the log from a v1 dump. The handle is already open.
func (s *Store) Load(r io.Reader) error {
	if s == nil {
		return fmt.Errorf("portfolio: nil store")
	}
	if r == nil {
		return fmt.Errorf("portfolio: nil reader")
	}
	sc := bufio.NewScanner(r)
	if !sc.Scan() {
		if err := sc.Err(); err != nil {
			return err
		}
		return fmt.Errorf("portfolio: empty store")
	}
	if strings.TrimSpace(sc.Text()) != storeVersion {
		return fmt.Errorf("portfolio: unsupported store version")
	}
	var recs []Record
	var seq uint64
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		rec, err := parseRec(line)
		if err != nil {
			return err
		}
		if rec.Seq <= seq {
			return fmt.Errorf("portfolio: store seq must increase")
		}
		seq = rec.Seq
		recs = append(recs, rec)
	}
	if err := sc.Err(); err != nil {
		return err
	}
	s.seq = seq
	s.recs = recs
	return nil
}

func parseRec(line string) (Record, error) {
	f := strings.Fields(line)
	if len(f) == 0 {
		return Record{}, fmt.Errorf("portfolio: empty record")
	}
	switch f[0] {
	case "o":
		if len(f) != 10 {
			return Record{}, fmt.Errorf("portfolio: order record wants 9 fields")
		}
		n, err := ints(f[1:])
		if err != nil {
			return Record{}, err
		}
		return Record{
			Seq:  uint64(n[0]),
			Ts:   n[1],
			Kind: RecOrder,
			Order: execution.Order{
				ID:         execution.OrderID(n[2]),
				Instrument: core.InstrumentID(n[3]),
				Side:       core.Side(n[4]),
				Qty:        core.Qty(n[5]),
				Kind:       execution.Kind(n[6]),
				Px:         core.Ticks(n[7]),
			},
			Status: execution.Status(n[8]),
		}, nil
	case "f":
		if len(f) != 8 {
			return Record{}, fmt.Errorf("portfolio: fill record wants 7 fields")
		}
		n, err := ints(f[1:])
		if err != nil {
			return Record{}, err
		}
		return Record{
			Seq:  uint64(n[0]),
			Ts:   n[1],
			Kind: RecFill,
			Fill: execution.Fill{
				OrderID:  execution.OrderID(n[2]),
				Side:     core.Side(n[3]),
				Px:       core.Ticks(n[4]),
				Qty:      core.Qty(n[5]),
				FeeCents: n[6],
				Ts:       n[1],
			},
		}, nil
	case "p":
		if len(f) != 6 {
			return Record{}, fmt.Errorf("portfolio: position record wants 5 fields")
		}
		n, err := ints(f[1:])
		if err != nil {
			return Record{}, err
		}
		return Record{
			Seq:  uint64(n[0]),
			Ts:   n[1],
			Kind: RecPosition,
			Pos: Position{
				Qty:      core.Qty(n[2]),
				AvgPx:    core.Ticks(n[3]),
				Realized: n[4],
			},
		}, nil
	default:
		return Record{}, fmt.Errorf("portfolio: unknown record %q", f[0])
	}
}

func ints(fields []string) ([]int64, error) {
	out := make([]int64, len(fields))
	for i, s := range fields {
		n, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("portfolio: not an int %q", s)
		}
		out[i] = n
	}
	return out, nil
}
