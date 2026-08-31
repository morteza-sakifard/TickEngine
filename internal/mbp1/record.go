package mbp1

import "time"

type Action string

const (
	ActionAdd    Action = "A"
	ActionModify Action = "M"
	ActionCancel Action = "C"
	ActionClear  Action = "R"
	ActionTrade  Action = "T"
	ActionFill   Action = "F"
	ActionNone   Action = "N"
)

type Side string

const (
	SideBid     Side = "B"
	SideAsk     Side = "A"
	SideUnknown Side = "N"
)

// Record is one parsed row of a Databento MBP-1 CSV file. Field names and
// types follow https://databento.com/docs/schemas-and-data-formats/mbp-1
type Record struct {
	TsRecv  time.Time
	TsEvent time.Time

	Action Action
	Side   Side

	Price float64
	Size  uint32

	Sequence uint32
	Symbol   string
}
