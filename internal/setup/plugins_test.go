package setup

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/onelastcommit/noctra/internal/plugins"
)

func localPluginRepo(t *testing.T) (dir, commit string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	dir = t.TempDir()
	run := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	run("init", "-b", "main", "--quiet")
	run("config", "user.email", "t@t")
	run("config", "user.name", "T")
	run("config", "commit.gpgsign", "false")
	run("config", "uploadpack.allowReachableSHA1InWant", "true")
	skill := filepath.Join(dir, "skills", "tdd")
	if err := os.MkdirAll(skill, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skill, "SKILL.md"), []byte("---\nname: tdd\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", "-A")
	run("commit", "-m", "init", "--quiet")
	return dir, run("rev-parse", "HEAD")
}

func TestInstallPluginSet_InstallsAndReportsEach(t *testing.T) {
	src, commit := localPluginRepo(t)
	root := t.TempDir()
	good := plugins.Plugin{Name: "good", Repo: "file://" + src, Commit: commit, Skills: []plugins.Skill{{Path: "skills/tdd"}}, Vetted: true}
	extra := plugins.Plugin{Name: "extra", Repo: "file://" + src, Commit: commit}
	broken := plugins.Plugin{Name: "broken", Repo: "file://" + src, Commit: commit, Skills: []plugins.Skill{{Path: "skills/missing"}}}

	var out bytes.Buffer
	installed, failed := installPluginSet(context.Background(), &out, root, []plugins.Plugin{good, extra, broken})

	if installed != 2 || failed != 1 {
		t.Fatalf("installed=%d failed=%d\n%s", installed, failed, out.String())
	}
	if !plugins.IsInstalled(root, good) || !plugins.IsInstalled(root, extra) {
		t.Fatal("plugins were not installed on disk")
	}
	for _, want := range []string{"✓ good", "✓ extra", "1 skills (unvetted)", "⚠️  broken", "2 plugins ready, 2 skills", "1 failed"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("output missing %q:\n%s", want, out.String())
		}
	}
}
