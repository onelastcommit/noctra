package plugins

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func installedPlugin(t *testing.T) string {
	t.Helper()
	src, commit := sourceRepo(t)
	inst, err := Install(context.Background(), t.TempDir(), Plugin{Name: "demo", Repo: "file://" + src, Commit: commit})
	if err != nil {
		t.Fatal(err)
	}
	return inst.Dir
}

func TestStage_CopiesSkillsAndCleansUp(t *testing.T) {
	pluginDir := installedPlugin(t)
	workdir := t.TempDir()

	cleanup, err := Stage(workdir, []string{pluginDir})
	if err != nil {
		t.Fatalf("Stage: %v", err)
	}
	for _, skill := range []string{"tdd", "debug"} {
		staged := filepath.Join(workdir, StagedSkillsDir, "noctra-demo-"+skill, "SKILL.md")
		if _, err := os.Stat(staged); err != nil {
			t.Errorf("%s not staged: %v", skill, err)
		}
	}

	cleanup()
	if _, err := os.Stat(filepath.Join(workdir, ".agents")); !os.IsNotExist(err) {
		t.Errorf(".agents should be removed when Stage created it (err=%v)", err)
	}
}

func TestStage_LeavesTheRepositorysOwnSkillsAlone(t *testing.T) {
	pluginDir := installedPlugin(t)
	workdir := t.TempDir()
	own := filepath.Join(workdir, StagedSkillsDir, "house-style", "SKILL.md")
	writeFile(t, own, "ours")

	cleanup, err := Stage(workdir, []string{pluginDir})
	if err != nil {
		t.Fatal(err)
	}
	cleanup()

	if b, err := os.ReadFile(own); err != nil || string(b) != "ours" {
		t.Fatalf("repository skill changed: %q, %v", b, err)
	}
	if _, err := os.Stat(filepath.Join(workdir, StagedSkillsDir, "noctra-demo-tdd")); !os.IsNotExist(err) {
		t.Errorf("staged skill left behind (err=%v)", err)
	}
}

func TestStage_NoPluginsIsANoOp(t *testing.T) {
	workdir := t.TempDir()
	cleanup, err := Stage(workdir, nil)
	if err != nil {
		t.Fatal(err)
	}
	cleanup()
	if _, err := os.Stat(filepath.Join(workdir, ".agents")); !os.IsNotExist(err) {
		t.Errorf("Stage(nil) created .agents (err=%v)", err)
	}
}

func TestStagedSkillName(t *testing.T) {
	if got := StagedSkillName("/x/superpowers@8ca22dba9a94", "test-driven-development"); got != "noctra-superpowers-test-driven-development" {
		t.Fatalf("got %q", got)
	}
}

func TestExcludeStaged_HidesStagedSkillsFromGit(t *testing.T) {
	requireGit(t)
	repo := t.TempDir()
	runGit(t, repo, "init", "-b", "main", "--quiet")
	writeFile(t, filepath.Join(repo, ".git/info/exclude"), "*.log")

	ctx := context.Background()
	for range 2 {
		if err := ExcludeStaged(ctx, repo); err != nil {
			t.Fatal(err)
		}
	}
	raw, _ := os.ReadFile(filepath.Join(repo, ".git/info/exclude"))
	if string(raw) != "*.log\n"+StagedSkillsExclude+"\n" {
		t.Fatalf("exclude file = %q", raw)
	}

	writeFile(t, filepath.Join(repo, StagedSkillsDir, "noctra-demo-tdd", "SKILL.md"), "x")
	writeFile(t, filepath.Join(repo, StagedSkillsDir, "house-style", "SKILL.md"), "x")
	status := runGit(t, repo, "status", "--porcelain", "--untracked-files=all")
	if status != "?? .agents/skills/house-style/SKILL.md" {
		t.Fatalf("git status = %q, want only the repository's own skill", status)
	}
}
