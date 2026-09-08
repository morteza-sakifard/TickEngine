package portfolio

import (
	"math"
	"strconv"
	"strings"
)

// Metrics is what the blotter compresses into. Sharpe and drawdown
// are summaries; they lie when three lucky tickets made the PF.
// Read the blotter before you trust these.
//
// Sharpe is unannualized: mean of per-fill equity changes over
// their sample stdev. Annualizing would invent a year we do not
// have. float64 is allowed here (conventions: statistical metrics).
type Metrics struct {
	Trades       int
	Wins         int
	Losses       int
	WinRate      float64
	AvgWin       int64
	AvgLoss      int64 // magnitude, cents
	Expectancy   int64
	ProfitFactor float64
	MaxDrawdown  int64
	Sharpe       float64
	NetPnL       int64
}

func computeMetrics(entries []Entry, trades []int64) Metrics {
	var m Metrics
	m.Trades = len(trades)
	var winSum, lossSum int64
	for _, p := range trades {
		if p > 0 {
			m.Wins++
			winSum += p
		} else if p < 0 {
			m.Losses++
			lossSum += -p
		}
	}
	if m.Trades > 0 {
		m.WinRate = float64(m.Wins) / float64(m.Trades)
		var tot int64
		for _, p := range trades {
			tot += p
		}
		m.Expectancy = tot / int64(m.Trades)
	}
	if m.Wins > 0 {
		m.AvgWin = winSum / int64(m.Wins)
	}
	if m.Losses > 0 {
		m.AvgLoss = lossSum / int64(m.Losses)
	}
	if lossSum == 0 {
		if winSum > 0 {
			m.ProfitFactor = math.Inf(1)
		}
	} else {
		m.ProfitFactor = float64(winSum) / float64(lossSum)
	}

	equity := make([]int64, 0, len(entries))
	rets := make([]float64, 0, len(entries))
	var prev int64
	var peak int64
	for i, e := range entries {
		equity = append(equity, e.Realized)
		if i == 0 {
			rets = append(rets, float64(e.Realized))
		} else {
			rets = append(rets, float64(e.Realized-prev))
		}
		prev = e.Realized
		if e.Realized > peak {
			peak = e.Realized
		}
		if dd := peak - e.Realized; dd > m.MaxDrawdown {
			m.MaxDrawdown = dd
		}
	}
	if n := len(equity); n > 0 {
		m.NetPnL = equity[n-1]
	}
	m.Sharpe = sharpe(rets)
	return m
}

func sharpe(r []float64) float64 {
	if len(r) < 2 {
		return 0
	}
	var sum float64
	for _, x := range r {
		sum += x
	}
	mean := sum / float64(len(r))
	var ss float64
	for _, x := range r {
		d := x - mean
		ss += d * d
	}
	sd := math.Sqrt(ss / float64(len(r)-1))
	if sd == 0 {
		return 0
	}
	return mean / sd
}

func (m Metrics) Text() string {
	return "trades=" + strconv.Itoa(m.Trades) +
		" wins=" + strconv.Itoa(m.Wins) +
		" losses=" + strconv.Itoa(m.Losses) +
		" win_rate=" + fmtFloat(m.WinRate) +
		" avg_win=" + strconv.FormatInt(m.AvgWin, 10) +
		" avg_loss=" + strconv.FormatInt(m.AvgLoss, 10) +
		" exp=" + strconv.FormatInt(m.Expectancy, 10) +
		" pf=" + fmtFloat(m.ProfitFactor) +
		" max_dd=" + strconv.FormatInt(m.MaxDrawdown, 10) +
		" sharpe=" + fmtFloat(m.Sharpe) +
		" net=" + strconv.FormatInt(m.NetPnL, 10) + "\n"
}

func fmtFloat(v float64) string {
	if math.IsInf(v, 1) {
		return "inf"
	}
	if math.IsInf(v, -1) {
		return "-inf"
	}
	if math.IsNaN(v) {
		return "nan"
	}
	s := strconv.FormatFloat(v, 'f', 6, 64)
	return strings.TrimRight(strings.TrimRight(s, "0"), ".")
}
