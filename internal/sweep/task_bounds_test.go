package sweep

import (
	"regexp"
	"strings"
	"testing"
)

var runCapRe = regexp.MustCompile(`(?i)at most (twice|\d+)`)

func TestIterativeTasksBoundTheirVerifyLoop(t *testing.T) {
	iterative := []string{"dead-code", "test-coverage", "deps-update"}

	for _, name := range iterative {
		task, ok := findTask(name)
		if !ok {
			t.Fatalf("task %q is not registered", name)
		}
		prompt := task.Prompt("/tmp/repo")
		if !runCapRe.MatchString(prompt) {
			t.Errorf("task %q runs the build/test suite in a loop but its prompt declares no cap", name)
		}
		if !strings.Contains(strings.ToLower(prompt), "and report what") {
			t.Errorf("task %q has no instruction to stop and report when it cannot finish", name)
		}
	}
}

func TestEveryTaskHasABlockedEscapeHatch(t *testing.T) {
	for _, task := range Catalog() {
		if !strings.Contains(task.Prompt("/tmp/repo"), "BLOCKED:") {
			t.Errorf("task %q cannot report that there is nothing to do", task.Name)
		}
	}
}

func findTask(name string) (Task, bool) {
	for _, t := range Catalog() {
		if t.Name == name {
			return t, true
		}
	}
	return Task{}, false
}
