package mbp1

type TradeReader struct {
	r   *Reader
	row uint64
}

func NewTradeReader(r *Reader) *TradeReader {
	return &TradeReader{r: r}
}

func (t *TradeReader) Read() (Record, error) {
	for {
		rec, err := t.r.Read()
		if err != nil {
			return Record{}, err
		}
		if rec.Action == ActionTrade {
			t.row++
			return rec, nil
		}
	}
}

func (t *TradeReader) Row() uint64 {
	return t.row
}
