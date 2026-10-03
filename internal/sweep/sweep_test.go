package sweep

import (
	"testing"
)

func TestCatalog_ContainsRegisteredTasks(t *testing.T) {
	tasks := Catalog()
	if len(tasks) < 2 {
		t.Fatalf("expected at least 2 registered tasks, got %d", len(tasks))
	}

	names := map[string]bool{}
	for _, task := range tasks {
		names[task.Name] = true
	}
	for _, want := range []string{"lint-cleanup", "dead-code"} {
		if !names[want] {
			t.Errorf("missing expected task %q", want)
		}
	}
}

func TestCatalog_ContainsMaintenanceTasks(t *testing.T) {
	names := map[string]bool{}
	for _, task := range Catalog() {
		names[task.Name] = true
	}
	for _, want := range []string{"deps-update", "test-coverage", "doc-drift", "modernize", "bug-scan"} {
		if !names[want] {
			t.Errorf("missing expected task %q", want)
		}
	}
}

func TestCatalog_UniqueNames(t *testing.T) {
	seen := map[string]bool{}
	for _, task := range Catalog() {
		if seen[task.Name] {
			t.Errorf("duplicate task Name %q", task.Name)
		}
		seen[task.Name] = true
	}
}

func TestCatalog_TaskFields(t *testing.T) {
	for _, task := range Catalog() {
		if task.Name == "" {
			t.Error("task has empty Name")
		}
		if task.Description == "" {
			t.Errorf("task %q has empty Description", task.Name)
		}
		if task.Cooldown <= 0 {
			t.Errorf("task %q has non-positive Cooldown: %v", task.Name, task.Cooldown)
		}
		if task.BranchSuffix == "" {
			t.Errorf("task %q has empty BranchSuffix", task.Name)
		}
		if task.CommitPrefix == "" {
			t.Errorf("task %q has empty CommitPrefix", task.Name)
		}
		if task.Prompt == nil {
			t.Errorf("task %q has nil Prompt function", task.Name)
		}
		if task.Prompt != nil {
			p := task.Prompt("/tmp/test-repo")
			if p == "" {
				t.Errorf("task %q produced empty prompt", task.Name)
			}
		}
	}
}

func TestCatalog_UniqueBranchSuffixes(t *testing.T) {
	seen := map[string]string{}
	for _, task := range Catalog() {
		if prev, ok := seen[task.BranchSuffix]; ok {
			t.Errorf("duplicate BranchSuffix %q: used by both %q and %q",
				task.BranchSuffix, prev, task.Name)
		}
		seen[task.BranchSuffix] = task.Name
	}
}

func TestFilterTasks_NilReturnsAll(t *testing.T) {
	all := Catalog()
	filtered := FilterTasks(nil)
	if len(filtered) != len(all) {
		t.Errorf("FilterTasks(nil): got %d tasks, want %d", len(filtered), len(all))
	}
}

func TestFilterTasks_EmptyReturnsAll(t *testing.T) {
	all := Catalog()
	filtered := FilterTasks([]string{})
	if len(filtered) != len(all) {
		t.Errorf("FilterTasks([]): got %d tasks, want %d", len(filtered), len(all))
	}
}

func TestFilterTasks_SelectsSubset(t *testing.T) {
	filtered := FilterTasks([]string{"lint-cleanup"})
	if len(filtered) != 1 {
		t.Fatalf("expected 1 task, got %d", len(filtered))
	}
	if filtered[0].Name != "lint-cleanup" {
		t.Errorf("expected lint-cleanup, got %q", filtered[0].Name)
	}
}

func TestFilterTasks_UnknownNameIgnored(t *testing.T) {
	filtered := FilterTasks([]string{"nonexistent"})
	if len(filtered) != 0 {
		t.Errorf("expected 0 tasks for unknown name, got %d", len(filtered))
	}
}

func TestSweepBranchName_OmitsTheRepo(t *testing.T) {
	for suffix, want := range map[string]string{
		"lint-cleanup": "noctra/sweep-lint-cleanup",
		"Deps-Update":  "noctra/sweep-deps-update",
	} {
		if got := SweepBranchName(suffix); got != want {
			t.Errorf("SweepBranchName(%q) = %q, want %q", suffix, got, want)
		}
	}
}

func TestTaskSuffixFromBranch(t *testing.T) {
	cases := []struct {
		branch string
		want   string
		ok     bool
	}{
		{"noctra/sweep-deps-update", "deps-update", true},
		{"noctra/sweep-lint-cleanup", "lint-cleanup", true},
		{"noctra/sweep-onelastcommit-onenote-mcp-deps-update", "", false},
		{"noctra/sweep-unknown-task", "", false},
		{"noctra/eng-42", "", false},
		{"sweep-deps-update", "", false},
	}
	for _, c := range cases {
		got, ok := TaskSuffixFromBranch(c.branch)
		if got != c.want || ok != c.ok {
			t.Errorf("TaskSuffixFromBranch(%q) = %q, %v; want %q, %v", c.branch, got, ok, c.want, c.ok)
		}
	}
}

func TestSweepIdentifier(t *testing.T) {
	tests := []struct {
		repoSlug string
		suffix   string
		want     string
	}{
		{"my-repo", "lint-cleanup", "SWEEP-MY-REPO-LINT-CLEANUP"},
		{"owner/my-repo", "lint-cleanup", "SWEEP-OWNER-MY-REPO-LINT-CLEANUP"},
	}
	for _, tt := range tests {
		got := SweepIdentifier(tt.repoSlug, tt.suffix)
		if got != tt.want {
			t.Errorf("SweepIdentifier(%q, %q) = %q, want %q", tt.repoSlug, tt.suffix, got, tt.want)
		}
	}
}

func TestParseSweepIdentifier(t *testing.T) {
	tests := []struct {
		id       string
		repoSlug string
		suffix   string
		ok       bool
	}{
		{"SWEEP-MY-REPO-LINT-CLEANUP", "my-repo", "lint-cleanup", true},
		{"SWEEP-AHMADALMEZAAL-TRADE-MATE-DEAD-CODE", "ahmadalmezaal-trade-mate", "dead-code", true},
		{"ENG-42", "", "", false},
		{"SWEEP-MY-REPO-UNKNOWN", "", "", false},
	}
	for _, tt := range tests {
		repoSlug, suffix, ok := ParseSweepIdentifier(tt.id)
		if repoSlug != tt.repoSlug || suffix != tt.suffix || ok != tt.ok {
			t.Errorf("ParseSweepIdentifier(%q) = (%q, %q, %v), want (%q, %q, %v)",
				tt.id, repoSlug, suffix, ok, tt.repoSlug, tt.suffix, tt.ok)
		}
	}
}

func TestParseSweepIdentifier_RoundTrip(t *testing.T) {
	repoSlug, suffix, ok := ParseSweepIdentifier(SweepIdentifier("my-repo", "lint-cleanup"))
	if !ok || repoSlug != "my-repo" || suffix != "lint-cleanup" {
		t.Errorf("round trip = (%q, %q, %v)", repoSlug, suffix, ok)
	}
}
