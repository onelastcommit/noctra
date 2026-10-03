package plugins

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func requireGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
}

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func sourceRepo(t *testing.T) (dir, commit string) {
	t.Helper()
	requireGit(t)
	dir = t.TempDir()
	runGit(t, dir, "init", "-b", "main", "--quiet")
	runGit(t, dir, "config", "user.email", "t@t")
	runGit(t, dir, "config", "user.name", "T")
	runGit(t, dir, "config", "commit.gpgsign", "false")
	runGit(t, dir, "config", "uploadpack.allowReachableSHA1InWant", "true")
	writeFile(t, filepath.Join(dir, "LICENSE"), "MIT")
	writeFile(t, filepath.Join(dir, "skills/tdd/SKILL.md"), "---\nname: tdd\n---\n")
	writeFile(t, filepath.Join(dir, "skills/tdd/notes.md"), "notes")
	writeFile(t, filepath.Join(dir, "skills/tdd/scripts/run.sh"), "echo hi")
	writeFile(t, filepath.Join(dir, "skills/debug/SKILL.md"), "---\nname: debug\n---\n")
	writeFile(t, filepath.Join(dir, "skills/not-a-skill/readme.md"), "x")
	writeFile(t, filepath.Join(dir, "hooks/hooks.json"), "{}")
	if err := os.Symlink("/etc/passwd", filepath.Join(dir, "skills/tdd/leak")); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", "-A")
	runGit(t, dir, "commit", "-m", "init", "--quiet")
	return dir, runGit(t, dir, "rev-parse", "HEAD")
}

func TestInstall_BuildsTrimmedPinnedPlugin(t *testing.T) {
	src, commit := sourceRepo(t)
	root := t.TempDir()
	p := Plugin{
		Name:   "demo",
		Repo:   "file://" + src,
		Commit: commit,
		Skills: []Skill{{Path: "skills/tdd", Exclude: []string{"scripts"}}},
		Vetted: true,
	}

	inst, err := Install(context.Background(), root, p)
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if inst.Dir != Dir(root, p) || !inst.Vetted || !slices.Equal(inst.Skills, []string{"tdd"}) {
		t.Fatalf("Installed = %+v", inst)
	}

	mustExist := []string{"skills/tdd/SKILL.md", "skills/tdd/notes.md", "LICENSE", ".claude-plugin/plugin.json", markerFile}
	for _, rel := range mustExist {
		if _, err := os.Stat(filepath.Join(inst.Dir, rel)); err != nil {
			t.Errorf("missing %s: %v", rel, err)
		}
	}
	mustNotExist := []string{"skills/tdd/scripts", "skills/tdd/leak", "skills/debug", "hooks"}
	for _, rel := range mustNotExist {
		if _, err := os.Lstat(filepath.Join(inst.Dir, rel)); !os.IsNotExist(err) {
			t.Errorf("%s should have been left out (err=%v)", rel, err)
		}
	}

	var manifest map[string]string
	raw, _ := os.ReadFile(filepath.Join(inst.Dir, ".claude-plugin/plugin.json"))
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest["name"] != "noctra-demo" || manifest["version"] != commit[:12] {
		t.Errorf("manifest = %v", manifest)
	}
	if len(manifest) != 3 {
		t.Errorf("manifest should declare only name/version/description, got %v", manifest)
	}

	if !IsInstalled(root, p) {
		t.Error("IsInstalled = false after Install")
	}
	leftovers, _ := filepath.Glob(filepath.Join(root, ".*"))
	if len(leftovers) != 0 {
		t.Errorf("temporary directories left behind: %v", leftovers)
	}
}

func TestInstall_ReusesMatchingInstallAndRebuildsOnSkillChange(t *testing.T) {
	src, commit := sourceRepo(t)
	root := t.TempDir()
	p := Plugin{Name: "demo", Repo: "file://" + src, Commit: commit, Skills: []Skill{{Path: "skills/tdd"}}}

	if _, err := Install(context.Background(), root, p); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(Dir(root, p), "skills/tdd/SKILL.md")
	writeFile(t, marker, "cached")

	if _, err := Install(context.Background(), root, p); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(marker); string(b) != "cached" {
		t.Fatal("matching install should be reused, not refetched")
	}

	p.Skills = append(p.Skills, Skill{Path: "skills/debug"})
	inst, err := Install(context.Background(), root, p)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(inst.Skills, []string{"debug", "tdd"}) {
		t.Fatalf("skills after change = %v", inst.Skills)
	}

	p.Skills = p.Skills[:1]
	inst, err = Install(context.Background(), root, p)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(inst.Skills, []string{"tdd"}) {
		t.Fatalf("dropping a skill should rebuild, got %v", inst.Skills)
	}
}

func TestInstall_DiscoversSkillsForExtras(t *testing.T) {
	src, commit := sourceRepo(t)
	inst, err := Install(context.Background(), t.TempDir(), Plugin{Name: "extra", Repo: "file://" + src, Commit: commit})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(inst.Skills, []string{"debug", "tdd"}) {
		t.Fatalf("discovered %v, want debug and tdd only", inst.Skills)
	}
}

func TestInstall_Failures(t *testing.T) {
	src, commit := sourceRepo(t)
	ctx := context.Background()

	cases := map[string]Plugin{
		"unknown commit": {Name: "demo", Repo: "file://" + src, Commit: strings.Repeat("0", 40), Skills: []Skill{{Path: "skills/tdd"}}},
		"missing skill":  {Name: "demo", Repo: "file://" + src, Commit: commit, Skills: []Skill{{Path: "skills/nope"}}},
		"not a skill":    {Name: "demo", Repo: "file://" + src, Commit: commit, Skills: []Skill{{Path: "skills/not-a-skill"}}},
	}
	for name, p := range cases {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			if _, err := Install(ctx, root, p); err == nil {
				t.Fatal("want an error")
			}
			if IsInstalled(root, p) {
				t.Fatal("a failed install must not leave an installed plugin")
			}
		})
	}
}

func TestInstallAll_SkipsFailuresAndKeepsTheRest(t *testing.T) {
	src, commit := sourceRepo(t)
	good := Plugin{Name: "good", Repo: "file://" + src, Commit: commit, Skills: []Skill{{Path: "skills/tdd"}}}
	bad := Plugin{Name: "bad", Repo: "file://" + src, Commit: commit, Skills: []Skill{{Path: "skills/nope"}}}

	installed, errs := InstallAll(context.Background(), t.TempDir(), []Plugin{bad, good})
	if len(installed) != 1 || installed[0].Name != "good" {
		t.Fatalf("installed = %+v", installed)
	}
	if len(errs) != 1 || !strings.Contains(errs[0].Error(), "bad") {
		t.Fatalf("errs = %v", errs)
	}
}

func TestCloneURL(t *testing.T) {
	cases := map[string]string{
		"obra/superpowers": "https://github.com/obra/superpowers.git",
		"file:///tmp/x":    "file:///tmp/x",
		"/tmp/x":           "/tmp/x",
	}
	for in, want := range cases {
		if got := cloneURL(in); got != want {
			t.Errorf("cloneURL(%q) = %q, want %q", in, got, want)
		}
	}
}
