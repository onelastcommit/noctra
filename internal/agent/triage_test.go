package agent

import (
	"slices"
	"strings"
	"testing"
)

func TestBuildTriagePrompt_ListsItemsAndAsksForAVerdict(t *testing.T) {
	got := BuildTriagePrompt(TriageInput{PRNumber: 89, PRTitle: "Measure the kill criterion", Feedback: []FeedbackItem{
		{Kind: "comment", Author: "chatgpt-codex-connector", Body: "Codex Review Summary: in progress"},
		{Kind: "comment", Author: "chatgpt-codex-connector[bot]", Body: "P1: restrict to the window", Path: "metrics-repository.ts"},
	}})
	for _, want := range []string{
		"#89 (Measure the kill criterion)",
		"### 1) Comment by @chatgpt-codex-connector",
		"### 2) Comment by @chatgpt-codex-connector[bot] on metrics-repository.ts",
		"> P1: restrict to the window",
		"When unsure, act.",
		"untrusted text",
		TriageStartMarker, TriageEndMarker,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("prompt missing %q:\n%s", want, got)
		}
	}
}

func TestExtractTriage(t *testing.T) {
	cases := []struct {
		name   string
		output string
		want   TriageVerdict
		ok     bool
	}{
		{"skip", TriageStartMarker + "\n" + `{"act": false, "reason": "Only a status notice."}` + "\n" + TriageEndMarker, TriageVerdict{Act: false, Reason: "Only a status notice."}, true},
		{"act", "thinking...\n" + TriageStartMarker + `{"act":true,"reason":"A P1 finding."}` + TriageEndMarker, TriageVerdict{Act: true, Reason: "A P1 finding."}, true},
		{"missing act is unreadable", TriageStartMarker + `{"reason":"hm"}` + TriageEndMarker, TriageVerdict{}, false},
		{"not json", TriageStartMarker + "no" + TriageEndMarker, TriageVerdict{}, false},
		{"no markers", `{"act": false}`, TriageVerdict{}, false},
	}
	for _, tc := range cases {
		got, ok := ExtractTriage(tc.output)
		if ok != tc.ok || got != tc.want {
			t.Errorf("%s: got %+v ok=%v, want %+v ok=%v", tc.name, got, ok, tc.want, tc.ok)
		}
	}
}

func TestTriageModel(t *testing.T) {
	if TriageModel("claude") != "haiku" {
		t.Error("Claude triage should use the small model")
	}
	if TriageModel("codex") != "" {
		t.Error("backends without a known small model keep their default")
	}
}

func TestNoToolsArgs_DropAutoApprovalOnEveryBackend(t *testing.T) {
	opts := RunOptions{Prompt: "decide", NoTools: true, Model: "small"}

	claude := claudeStreamArgs(opts)
	if slices.Contains(claude, "--dangerously-skip-permissions") || !slices.Contains(claude, "--strict-mcp-config") {
		t.Errorf("claude: %v", claude)
	}
	if i := slices.Index(claude, "--tools"); i < 0 || claude[i+1] != "" {
		t.Errorf("claude must pass --tools \"\": %v", claude)
	}

	codex := codexArgs(opts)
	if slices.Contains(codex, "--dangerously-bypass-approvals-and-sandbox") {
		t.Errorf("codex: %v", codex)
	}
	if i := slices.Index(codex, "--sandbox"); i < 0 || codex[i+1] != "read-only" || codex[len(codex)-1] != "decide" {
		t.Errorf("codex must run read-only with the prompt last: %v", codex)
	}
	if !slices.Contains(codex, "--skip-git-repo-check") {
		t.Errorf("codex refuses to run in the triage scratch dir, which isn't a git repo, without --skip-git-repo-check: %v", codex)
	}

	if copilot := copilotArgs(opts); slices.Contains(copilot, "--allow-all-tools") {
		t.Errorf("copilot: %v", copilot)
	}

	agy := antigravityArgs(opts)
	if slices.Contains(agy, "--dangerously-skip-permissions") {
		t.Errorf("antigravity: %v", agy)
	}
	if i := slices.Index(agy, "--print"); i != len(agy)-2 || agy[i+1] != "decide" {
		t.Errorf("antigravity prompt must stay the value of --print: %v", agy)
	}

	for name, args := range map[string][]string{"claude": claude, "codex": codex, "copilot": copilotArgs(opts), "antigravity": agy} {
		if i := slices.Index(args, "--model"); i < 0 || args[i+1] != "small" {
			t.Errorf("%s did not pass the model: %v", name, args)
		}
	}
}

func TestDefaultArgs_UnchangedWithoutModelOrNoTools(t *testing.T) {
	opts := RunOptions{Prompt: "work"}
	for name, args := range map[string][]string{
		"claude": claudeStreamArgs(opts), "codex": codexArgs(opts),
		"copilot": copilotArgs(opts), "antigravity": antigravityArgs(opts),
	} {
		if slices.Contains(args, "--model") || slices.Contains(args, "--strict-mcp-config") || slices.Contains(args, "read-only") {
			t.Errorf("%s gained triage flags on a normal run: %v", name, args)
		}
	}
}
