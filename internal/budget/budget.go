package budget

import (
	"fmt"
	"sync"
	"time"
)

type Tracker struct {
	mu  sync.Mutex
	cfg Config

	sessionTokens  int64
	sessionCostUSD float64
	dailyTokens    int64
	dailyCostUSD   float64
	dayStart       time.Time

	paused      bool
	pausedUntil time.Time
	pauseReason string

	now func() time.Time
}

func New(cfg Config) *Tracker {
	return &Tracker{
		cfg:      cfg,
		dayStart: todayUTC(time.Now()),
		now:      time.Now,
	}
}

func (t *Tracker) Record(tokens int64, costUSD float64) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.maybeResetDaily()
	t.sessionTokens += tokens
	t.sessionCostUSD += costUSD
	t.dailyTokens += tokens
	t.dailyCostUSD += costUSD
}

func (t *Tracker) Exceeded() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.maybeResetDaily()
	return t.exceeded()
}

func (t *Tracker) exceeded() bool {
	if t.cfg.MaxDailyTokens > 0 && t.dailyTokens >= t.cfg.MaxDailyTokens {
		return true
	}
	if t.cfg.MaxDailyUSD > 0 && t.dailyCostUSD >= t.cfg.MaxDailyUSD {
		return true
	}
	return false
}

func (t *Tracker) ExceededReason() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.maybeResetDaily()
	if t.cfg.MaxDailyTokens > 0 && t.dailyTokens >= t.cfg.MaxDailyTokens {
		return fmt.Sprintf("daily token cap (%s/%s)",
			formatTokens(t.dailyTokens), formatTokens(t.cfg.MaxDailyTokens))
	}
	if t.cfg.MaxDailyUSD > 0 && t.dailyCostUSD >= t.cfg.MaxDailyUSD {
		return fmt.Sprintf("daily cost cap ($%.2f/$%.2f)",
			t.dailyCostUSD, t.cfg.MaxDailyUSD)
	}
	return ""
}

func (t *Tracker) Pause(reason string, until time.Time) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.paused = true
	t.pauseReason = reason
	t.pausedUntil = until
}

func (t *Tracker) IsPaused() (paused bool, until time.Time, reason string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.maybeResetDaily()
	t.maybeAutoResume()
	if !t.paused {
		return false, time.Time{}, ""
	}
	return true, t.pausedUntil, t.pauseReason
}

func (t *Tracker) maybeAutoResume() {
	if !t.paused {
		return
	}
	if !t.pausedUntil.IsZero() && t.now().After(t.pausedUntil) {
		t.paused = false
		t.pauseReason = ""
		t.pausedUntil = time.Time{}
	}
}

func (t *Tracker) Resume() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.paused = false
	t.pauseReason = ""
	t.pausedUntil = time.Time{}
}

func (t *Tracker) Stats() Stats {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.maybeResetDaily()
	t.maybeAutoResume()
	return Stats{
		SessionTokens:  t.sessionTokens,
		SessionCostUSD: t.sessionCostUSD,
		DailyTokens:    t.dailyTokens,
		DailyCostUSD:   t.dailyCostUSD,
		MaxDailyTokens: t.cfg.MaxDailyTokens,
		MaxDailyUSD:    t.cfg.MaxDailyUSD,
		Paused:         t.paused,
		PausedUntil:    t.pausedUntil,
		PauseReason:    t.pauseReason,
	}
}

func (t *Tracker) maybeResetDaily() {
	today := todayUTC(t.now())
	if today.After(t.dayStart) {
		t.dailyTokens = 0
		t.dailyCostUSD = 0
		t.dayStart = today
	}
}

func NextUTCMidnight() time.Time {
	now := time.Now().UTC()
	return time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, time.UTC)
}

func todayUTC(t time.Time) time.Time {
	u := t.UTC()
	return time.Date(u.Year(), u.Month(), u.Day(), 0, 0, 0, 0, time.UTC)
}

func formatTokens(n int64) string {
	switch {
	case n >= 1_000_000:
		if n%1_000_000 == 0 {
			return fmt.Sprintf("%dM", n/1_000_000)
		}
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	case n >= 1_000:
		if n%1_000 == 0 {
			return fmt.Sprintf("%dK", n/1_000)
		}
		return fmt.Sprintf("%.1fK", float64(n)/1_000)
	default:
		return fmt.Sprintf("%d", n)
	}
}

func FormatTokens(n int64) string { return formatTokens(n) }
