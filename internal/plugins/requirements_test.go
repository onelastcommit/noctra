package plugins

import (
	"context"
	"os"
	"slices"
	"strings"
	"testing"
)

var (
	satisfied = Requirement{Name: "always there", Command: []string{"true"}, Hint: "nothing to do"}
	missing   = Requirement{Name: "never there", Command: []string{"sh", "-c", "echo 'No module named playwright' >&2; exit 1"}, Hint: "pip install playwright"}
	absent    = Requirement{Name: "absent binary", Command: []string{"noctra-definitely-not-a-command"}, Hint: "install it"}
)

func TestCheckRequirements(t *testing.T) {
	plugins := []Plugin{
		{Name: "anthropic-skills", Skills: []Skill{
			{Path: "skills/webapp-testing", Requires: []Requirement{missing}},
			{Path: "skills/other", Requires: []Requirement{satisfied}},
		}},
		{Name: "only-gated", Skills: []Skill{{Path: "skills/browser", Requires: []Requirement{absent}}}},
		{Name: "plain", Skills: []Skill{{Path: "skills/tdd"}}},
		{Name: "extra"},
	}

	usable, unmet := CheckRequirements(context.Background(), plugins)

	if !slices.Equal(names(usable), []string{"anthropic-skills", "plain", "extra"}) {
		t.Fatalf("usable = %v; a plugin whose every skill is gated out should drop entirely", names(usable))
	}
	if paths := skillPaths(usable[0]); !slices.Equal(paths, []string{"skills/other"}) {
		t.Fatalf("anthropic-skills kept %v", paths)
	}
	if !slices.Equal(SkillNames(unmet), []string{"webapp-testing", "browser"}) {
		t.Fatalf("unmet = %v", SkillNames(unmet))
	}
	if !strings.Contains(unmet[0].Err.Error(), "No module named playwright") {
		t.Errorf("error should carry the check's last output line, got %v", unmet[0].Err)
	}
	if !strings.Contains(unmet[0].String(), "pip install playwright") {
		t.Errorf("String() should include the hint: %s", unmet[0])
	}
	if len(plugins[0].Skills) != 2 {
		t.Error("CheckRequirements must not mutate its input")
	}
}

func TestCheckRequirements_RunsEachCommandOnce(t *testing.T) {
	counter := t.TempDir() + "/count"
	once := Requirement{Name: "counted", Command: []string{"sh", "-c", "echo x >> " + counter}}
	plugins := []Plugin{
		{Name: "a", Skills: []Skill{{Path: "skills/one", Requires: []Requirement{once}}, {Path: "skills/two", Requires: []Requirement{once}}}},
	}
	if _, unmet := CheckRequirements(context.Background(), plugins); len(unmet) != 0 {
		t.Fatalf("unmet = %v", unmet)
	}
	if got := countLines(t, counter); got != 1 {
		t.Fatalf("check ran %d times, want 1", got)
	}
}

func countLines(t *testing.T, path string) int {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return strings.Count(string(raw), "\n")
}
