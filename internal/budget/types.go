package budget

import "time"

type Config struct {
	MaxDailyTokens int64
	MaxDailyUSD    float64
}

type Stats struct {
	SessionTokens  int64
	SessionCostUSD float64
	DailyTokens    int64
	DailyCostUSD   float64
	MaxDailyTokens int64
	MaxDailyUSD    float64
	Paused         bool
	PausedUntil    time.Time
	PauseReason    string
}

func (s Stats) HasCaps() bool {
	return s.MaxDailyTokens > 0 || s.MaxDailyUSD > 0
}
