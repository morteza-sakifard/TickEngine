package marketdata

import "github.com/morteza-sakifard/TickEngine/internal/core"

// BookAction is one MBO verb. Fill does not change the reconstructed
// book — Databento's F rows are passive-order detail on a trade.
// The book moves on Add, Cancel, Modify, and Clear.
type BookAction uint8

const (
	BookAdd BookAction = iota + 1
	BookCancel
	BookModify
	BookClear
	BookFill
)

func (a BookAction) String() string {
	switch a {
	case BookAdd:
		return "add"
	case BookCancel:
		return "cancel"
	case BookModify:
		return "modify"
	case BookClear:
		return "clear"
	case BookFill:
		return "fill"
	default:
		return "unknown"
	}
}

// Book is one order-level change. Valid when Kind == KindBook.
// OrderID is the venue id, not our simulated Order.ID.
type Book struct {
	Action  BookAction
	OrderID uint64
	Side    core.Side
	Px      core.Ticks
	Qty     core.Qty
}
