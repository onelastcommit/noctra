package pipeline

import (
	"testing"

	"github.com/onelastcommit/noctra/internal/plugins"
)

func TestPluginSummary(t *testing.T) {
	installed := []plugins.Installed{
		{Name: "superpowers", Vetted: true, Skills: []string{"a", "b"}},
		{Name: "acme-skills", Skills: []string{"c"}},
	}
	cases := []struct {
		name      string
		packs     []string
		installed []plugins.Installed
		failures  int
		skipped   []string
		want      string
	}{
		{"off", nil, nil, 0, nil, "Disabled"},
		{"none", []string{"none"}, nil, 0, nil, "Disabled"},
		{"packs", []string{"engineering"}, installed[:1], 0, nil, "engineering (1 plugins, 2 skills)"},
		{"unvetted and failures", []string{"engineering"}, installed, 2, nil, "engineering (2 plugins, 3 skills) + unvetted: acme-skills; 2 failed to fetch"},
		{"missing dependency", []string{"frontend"}, installed[:1], 0, []string{"webapp-testing"}, "frontend (1 plugins, 2 skills); left out for missing dependencies: webapp-testing"},
		{"extras only", nil, installed[1:], 0, nil, "1 plugins, 1 skills + unvetted: acme-skills"},
	}
	for _, c := range cases {
		if got := pluginSummary(c.packs, c.installed, c.failures, c.skipped); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}
