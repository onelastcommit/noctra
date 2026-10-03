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
		want      string
	}{
		{"off", nil, nil, 0, "Disabled"},
		{"none", []string{"none"}, nil, 0, "Disabled"},
		{"packs", []string{"engineering"}, installed[:1], 0, "engineering (1 plugins, 2 skills)"},
		{"unvetted and failures", []string{"engineering"}, installed, 2, "engineering (2 plugins, 3 skills) + unvetted: acme-skills — 2 failed to fetch"},
		{"extras only", nil, installed[1:], 0, "1 plugins, 1 skills + unvetted: acme-skills"},
	}
	for _, c := range cases {
		if got := pluginSummary(c.packs, c.installed, c.failures); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}
