package execution

import (
	"fmt"
	"sort"
)

// Snapshot is what the venue (or a broker) says is live right now.
// It is truth during reconcile. Local cache can be stale after a
// drop, a restart, or a manual cancel the process did not see.
type Snapshot struct {
	Working []Order
	LastID  OrderID
}

// Report is the diff. Keep and Adopt are the live set; Ghost is
// what the cache still believes and the venue does not.
type Report struct {
	Keep  []Order
	Adopt []Order
	Ghost []Order
}

// Reconcile compares cached working orders to a snapshot. Match is
// by Order.ID. Output slices are sorted by ID — no map range.
func Reconcile(local []Order, snap Snapshot) Report {
	loc := byID(local)
	rem := byID(snap.Working)
	var out Report
	for _, id := range sortedIDs(local) {
		if o, ok := rem[id]; ok {
			out.Keep = append(out.Keep, o)
			continue
		}
		out.Ghost = append(out.Ghost, loc[id])
	}
	for _, id := range sortedIDs(snap.Working) {
		if _, ok := loc[id]; !ok {
			out.Adopt = append(out.Adopt, rem[id])
		}
	}
	return out
}

func (r Report) Live() []Order {
	out := make([]Order, 0, len(r.Keep)+len(r.Adopt))
	out = append(out, r.Keep...)
	out = append(out, r.Adopt...)
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func byID(orders []Order) map[OrderID]Order {
	m := make(map[OrderID]Order, len(orders))
	for _, o := range orders {
		if o.ID == 0 {
			continue
		}
		m[o.ID] = o
	}
	return m
}

func sortedIDs(orders []Order) []OrderID {
	seen := make(map[OrderID]struct{}, len(orders))
	ids := make([]OrderID, 0, len(orders))
	for _, o := range orders {
		if o.ID == 0 {
			continue
		}
		if _, ok := seen[o.ID]; ok {
			continue
		}
		seen[o.ID] = struct{}{}
		ids = append(ids, o.ID)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

// Working is inbox + delayed + resting, sorted by ID.
func (v *Venue) Working() []Order {
	if v == nil {
		return nil
	}
	n := len(v.inbox) + len(v.delayed) + len(v.working)
	if n == 0 {
		return nil
	}
	out := make([]Order, 0, n)
	out = append(out, v.inbox...)
	for _, d := range v.delayed {
		out = append(out, d.order)
	}
	for _, r := range v.working {
		out = append(out, r.order)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func (v *Venue) LastID() OrderID {
	if v == nil {
		return 0
	}
	return v.nextID
}

// Adopt replaces the live set with the snapshot. Limits rest;
// market stays in the inbox until the next Settle. lastID keeps
// the next Enqueue from reusing an id the venue already issued.
func (v *Venue) Adopt(orders []Order, lastID OrderID) error {
	if v == nil {
		return fmt.Errorf("execution: nil venue")
	}
	for _, o := range orders {
		if o.ID == 0 {
			return fmt.Errorf("execution: adopt requires id")
		}
		if err := o.validate(); err != nil {
			return err
		}
	}
	v.inbox = nil
	v.delayed = nil
	v.working = nil
	max := lastID
	for _, o := range orders {
		if o.kind() == KindLimit {
			v.rest(o)
		} else {
			v.inbox = append(v.inbox, o)
		}
		if o.ID > max {
			max = o.ID
		}
	}
	if max > v.nextID {
		v.nextID = max
	}
	return nil
}
