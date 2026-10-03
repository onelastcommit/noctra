package authcheck

type Report struct {
	Failing   []Result
	Reminders []Result
	Recovered []Result
}

func (r Report) Empty() bool {
	return len(r.Failing) == 0 && len(r.Reminders) == 0 && len(r.Recovered) == 0
}

type Tracker struct {
	failing map[string]struct{}
}

func NewTracker() *Tracker {
	return &Tracker{failing: map[string]struct{}{}}
}

func (t *Tracker) Observe(results []Result) Report {
	var rep Report
	for _, r := range results {
		if r.Skipped {
			continue
		}
		_, wasFailing := t.failing[r.Name]
		switch {
		case r.OK && wasFailing:
			delete(t.failing, r.Name)
			rep.Recovered = append(rep.Recovered, r)
		case r.OK:
		case wasFailing:
			rep.Reminders = append(rep.Reminders, r)
		default:
			t.failing[r.Name] = struct{}{}
			rep.Failing = append(rep.Failing, r)
		}
	}
	return rep
}
