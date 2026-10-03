package authcheck

import "time"

type Report struct {
	Failing   []Result
	Reminders []Result
	Recovered []Result
}

func (r Report) Empty() bool {
	return len(r.Failing) == 0 && len(r.Reminders) == 0 && len(r.Recovered) == 0
}

type failure struct {
	lastAlert time.Time
}

type Tracker struct {
	RemindEvery time.Duration
	failing     map[string]failure
}

func NewTracker(remindEvery time.Duration) *Tracker {
	return &Tracker{RemindEvery: remindEvery, failing: map[string]failure{}}
}

func (t *Tracker) Observe(now time.Time, results []Result) Report {
	var rep Report
	for _, r := range results {
		if r.Skipped {
			continue
		}
		prev, wasFailing := t.failing[r.Name]
		switch {
		case r.OK && wasFailing:
			delete(t.failing, r.Name)
			rep.Recovered = append(rep.Recovered, r)
		case r.OK:
		case !wasFailing:
			t.failing[r.Name] = failure{lastAlert: now}
			rep.Failing = append(rep.Failing, r)
		case t.RemindEvery > 0 && now.Sub(prev.lastAlert) >= t.RemindEvery:
			t.failing[r.Name] = failure{lastAlert: now}
			rep.Reminders = append(rep.Reminders, r)
		}
	}
	return rep
}
