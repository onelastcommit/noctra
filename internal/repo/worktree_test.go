package repo

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestCreateAndCleanupWorktree(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	repo := t.TempDir()
	mustGit := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, string(out))
		}
	}

	mustGit("init", "-b", "main", "--quiet")
	mustGit("config", "user.email", "t@t")
	mustGit("config", "user.name", "T")
	mustGit("config", "commit.gpgsign", "false")
	mustGit("remote", "add", "origin", repo)

	if err := os.WriteFile(filepath.Join(repo, "README.md"), []byte("init"), 0o600); err != nil {
		t.Fatal(err)
	}
	mustGit("add", "-A")
	mustGit("commit", "-m", "init", "--quiet")
	mustGit("fetch", "origin", "--quiet")

	base := t.TempDir()
	ctx := context.Background()

	wt, err := CreateWorktree(ctx, base, "ENG-200", repo, "main")
	if err != nil {
		t.Fatalf("CreateWorktree: %v", err)
	}
	if wt.Branch != "noctra/eng-200" {
		t.Errorf("branch: got %q, want %q", wt.Branch, "noctra/eng-200")
	}
	if _, err := os.Stat(wt.Path); err != nil {
		t.Fatalf("worktree dir missing: %v", err)
	}

	CleanupWorktree(ctx, repo, base, "ENG-200")
	if _, err := os.Stat(wt.Path); !os.IsNotExist(err) {
		t.Errorf("worktree dir should have been removed (err=%v)", err)
	}
}

func TestCreateWorktree_BadRepoPath(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	_, err := CreateWorktree(context.Background(), t.TempDir(), "ENG-999", "/does/not/exist", "main")
	if err == nil {
		t.Fatal("expected CreateWorktree to fail on a bad repo path")
	}
}

func TestResumeWorktree_PicksUpExistingBranchCommits(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	repo := t.TempDir()
	mustGit := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, string(out))
		}
	}

	mustGit("init", "-b", "main", "--quiet")
	mustGit("config", "user.email", "t@t")
	mustGit("config", "user.name", "T")
	mustGit("config", "commit.gpgsign", "false")
	mustGit("config", "receive.denyCurrentBranch", "ignore")
	mustGit("remote", "add", "origin", repo)

	if err := os.WriteFile(filepath.Join(repo, "README.md"), []byte("init"), 0o600); err != nil {
		t.Fatal(err)
	}
	mustGit("add", "-A")
	mustGit("commit", "-m", "init", "--quiet")
	mustGit("fetch", "origin", "--quiet")

	base := t.TempDir()
	ctx := context.Background()

	wt1, err := CreateWorktree(ctx, base, "ENG-300", repo, "main")
	if err != nil {
		t.Fatalf("CreateWorktree: %v", err)
	}

	markerPath := filepath.Join(wt1.Path, "from-attempt-1.txt")
	if err := os.WriteFile(markerPath, []byte("hello"), 0o600); err != nil {
		t.Fatal(err)
	}
	runInWt := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = wt1.Path
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v in worktree: %v\n%s", args, err, string(out))
		}
	}
	runInWt("add", "-A")
	runInWt("commit", "-m", "attempt-1", "--quiet")
	runInWt("push", "-u", "origin", wt1.Branch, "--quiet")

	CleanupWorktree(ctx, repo, base, "ENG-300")

	wt2, err := ResumeWorktree(ctx, base, "ENG-300", repo)
	if err != nil {
		t.Fatalf("ResumeWorktree: %v", err)
	}
	if wt2.Branch != "noctra/eng-300" {
		t.Errorf("branch: got %q", wt2.Branch)
	}
	if _, err := os.Stat(filepath.Join(wt2.Path, "from-attempt-1.txt")); err != nil {
		t.Errorf("resumed worktree is missing the prior attempt's marker file: %v", err)
	}

	CleanupWorktree(ctx, repo, base, "ENG-300")
}

func TestResumeWorktreeWithBranch_UsesGivenBranchNotIdentifier(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	repo := t.TempDir()
	mustGit := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, string(out))
		}
	}

	mustGit("init", "-b", "main", "--quiet")
	mustGit("config", "user.email", "t@t")
	mustGit("config", "user.name", "T")
	mustGit("config", "commit.gpgsign", "false")
	mustGit("config", "receive.denyCurrentBranch", "ignore")
	mustGit("remote", "add", "origin", repo)

	if err := os.WriteFile(filepath.Join(repo, "README.md"), []byte("init"), 0o600); err != nil {
		t.Fatal(err)
	}
	mustGit("add", "-A")
	mustGit("commit", "-m", "init", "--quiet")
	mustGit("fetch", "origin", "--quiet")

	base := t.TempDir()
	ctx := context.Background()
	identifier := "SWEEP-ACME-WIDGETS-BUG-SCAN"
	branch := "noctra/sweep-bug-scan"

	wt1, err := CreateWorktreeWithBranch(ctx, base, identifier, repo, "main", branch)
	if err != nil {
		t.Fatalf("CreateWorktreeWithBranch: %v", err)
	}
	if err := os.WriteFile(filepath.Join(wt1.Path, "sweep.txt"), []byte("fix"), 0o600); err != nil {
		t.Fatal(err)
	}
	runInWt := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = wt1.Path
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v in worktree: %v\n%s", args, err, string(out))
		}
	}
	runInWt("add", "-A")
	runInWt("commit", "-m", "sweep", "--quiet")
	runInWt("push", "-u", "origin", branch, "--quiet")
	CleanupWorktree(ctx, repo, base, identifier)

	if _, err := ResumeWorktree(ctx, base, identifier, repo); err == nil {
		t.Fatal("ResumeWorktree derives the branch from the identifier and should not find the sweep branch")
	}

	wt2, err := ResumeWorktreeWithBranch(ctx, base, identifier, repo, branch)
	if err != nil {
		t.Fatalf("ResumeWorktreeWithBranch: %v", err)
	}
	if wt2.Branch != branch {
		t.Errorf("branch: got %q, want %q", wt2.Branch, branch)
	}
	if _, err := os.Stat(filepath.Join(wt2.Path, "sweep.txt")); err != nil {
		t.Errorf("resumed worktree is missing the sweep commit: %v", err)
	}

	CleanupWorktree(ctx, repo, base, identifier)
}

func TestResumeWorktree_OverStaleWorktree(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	repo := t.TempDir()
	mustGit := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, string(out))
		}
	}

	mustGit("init", "-b", "main", "--quiet")
	mustGit("config", "user.email", "t@t")
	mustGit("config", "user.name", "T")
	mustGit("config", "commit.gpgsign", "false")
	mustGit("config", "receive.denyCurrentBranch", "ignore")
	mustGit("remote", "add", "origin", repo)
	if err := os.WriteFile(filepath.Join(repo, "README.md"), []byte("init"), 0o600); err != nil {
		t.Fatal(err)
	}
	mustGit("add", "-A")
	mustGit("commit", "-m", "init", "--quiet")
	mustGit("fetch", "origin", "--quiet")

	base := t.TempDir()
	ctx := context.Background()

	wt1, err := CreateWorktree(ctx, base, "ENG-400", repo, "main")
	if err != nil {
		t.Fatalf("CreateWorktree: %v", err)
	}
	if err := os.WriteFile(filepath.Join(wt1.Path, "marker.txt"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	runInWt := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = wt1.Path
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v in worktree: %v\n%s", args, err, string(out))
		}
	}
	runInWt("add", "-A")
	runInWt("commit", "-m", "attempt-1", "--quiet")
	runInWt("push", "-u", "origin", wt1.Branch, "--quiet")

	wt2, err := ResumeWorktree(ctx, base, "ENG-400", repo)
	if err != nil {
		t.Fatalf("ResumeWorktree over a stale worktree: %v", err)
	}
	if _, err := os.Stat(filepath.Join(wt2.Path, "marker.txt")); err != nil {
		t.Errorf("resumed worktree missing prior marker: %v", err)
	}

	CleanupWorktree(ctx, repo, base, "ENG-400")
}

func TestResumeWorktree_FailsIfBranchNotOnRemote(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	repo := t.TempDir()
	mustGit := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, string(out))
		}
	}

	mustGit("init", "-b", "main", "--quiet")
	mustGit("config", "user.email", "t@t")
	mustGit("config", "user.name", "T")
	mustGit("config", "commit.gpgsign", "false")
	mustGit("remote", "add", "origin", repo)
	if err := os.WriteFile(filepath.Join(repo, "README.md"), []byte("init"), 0o600); err != nil {
		t.Fatal(err)
	}
	mustGit("add", "-A")
	mustGit("commit", "-m", "init", "--quiet")
	mustGit("fetch", "origin", "--quiet")

	if _, err := ResumeWorktree(context.Background(), t.TempDir(), "ENG-NEVER-PUSHED", repo); err == nil {
		t.Fatal("expected ResumeWorktree to fail when the branch isn't on origin")
	}
}

func newFixtureRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	mustGit := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, string(out))
		}
	}
	mustGit("init", "-b", "main", "--quiet")
	mustGit("config", "user.email", "t@t")
	mustGit("config", "user.name", "T")
	mustGit("config", "commit.gpgsign", "false")
	mustGit("remote", "add", "origin", dir)
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("init"), 0o600); err != nil {
		t.Fatal(err)
	}
	mustGit("add", "-A")
	mustGit("commit", "-m", "init", "--quiet")
	mustGit("fetch", "origin", "--quiet")
	return dir
}

func TestCreateWorktreeWithBranch_ConcurrentSameRepo(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	repoDir := newFixtureRepo(t)
	base := t.TempDir()
	ctx := context.Background()

	const n = 8
	errs := make(chan error, n)
	var start sync.WaitGroup
	start.Add(1)
	for i := range n {
		go func(i int) {
			start.Wait()
			id := fmt.Sprintf("SWEEP-FIXTURE-TASK-%d", i)
			_, err := CreateWorktreeWithBranch(ctx, base, id, repoDir, "main", "noctra/sweep-task-"+strconv.Itoa(i))
			errs <- err
		}(i)
	}
	start.Done()

	for range n {
		if err := <-errs; err != nil {
			t.Fatalf("concurrent CreateWorktreeWithBranch failed: %v", err)
		}
	}
}

func TestLockRepo_DistinctReposDoNotBlock(t *testing.T) {
	unlockA := lockRepo("/repos/a")
	defer unlockA()

	done := make(chan struct{})
	go func() {
		lockRepo("/repos/b")()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("lockRepo serialized two distinct repos")
	}
}

func TestLockRepo_SamePathSerializes(t *testing.T) {
	unlock := lockRepo("/repos/a")

	acquired := make(chan struct{})
	go func() {
		lockRepo("/repos/a/../a")()
		close(acquired)
	}()

	select {
	case <-acquired:
		t.Fatal("lockRepo let two holders into the same repo")
	case <-time.After(100 * time.Millisecond):
	}

	unlock()
	select {
	case <-acquired:
	case <-time.After(5 * time.Second):
		t.Fatal("lockRepo did not release")
	}
}

func gitIn(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, string(out))
	}
}

func TestCreateOrResumeWorktree_ResumesOrphanedBranch(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	repoDir := newFixtureRepo(t)

	gitIn(t, repoDir, "checkout", "-q", "-b", "noctra/eng-414")
	if err := os.WriteFile(filepath.Join(repoDir, "orphan.md"), []byte("prior attempt"), 0o600); err != nil {
		t.Fatal(err)
	}
	gitIn(t, repoDir, "add", "-A")
	gitIn(t, repoDir, "commit", "-m", "ENG-414: prior attempt", "--quiet")
	gitIn(t, repoDir, "checkout", "-q", "main")

	base := t.TempDir()
	wt, resumed, err := CreateOrResumeWorktree(context.Background(), base, "ENG-414", repoDir, "main")
	if err != nil {
		t.Fatalf("CreateOrResumeWorktree: %v", err)
	}
	if !resumed {
		t.Error("expected the existing remote branch to be resumed, not recreated from main")
	}
	if _, err := os.Stat(filepath.Join(wt.Path, "orphan.md")); err != nil {
		t.Errorf("resumed worktree lost the prior attempt's commit: %v", err)
	}
}

func TestCreateOrResumeWorktree_FreshWhenNoRemoteBranch(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	repoDir := newFixtureRepo(t)
	base := t.TempDir()

	wt, resumed, err := CreateOrResumeWorktree(context.Background(), base, "ENG-999", repoDir, "main")
	if err != nil {
		t.Fatalf("CreateOrResumeWorktree: %v", err)
	}
	if resumed {
		t.Error("no remote branch exists, so nothing should have been resumed")
	}
	if wt.Branch != "noctra/eng-999" {
		t.Errorf("branch: got %q, want noctra/eng-999", wt.Branch)
	}
}

func TestRemoteBranchExists(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	repoDir := newFixtureRepo(t)
	if !RemoteBranchExists(context.Background(), repoDir, "main") {
		t.Error("main should exist on origin")
	}
	if RemoteBranchExists(context.Background(), repoDir, "noctra/never-pushed") {
		t.Error("an unpushed branch must not report as existing")
	}
}

func TestCreateWorktreeWithBranch_OverUnregisteredLeftoverDir(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	repoDir := newFixtureRepo(t)
	base := t.TempDir()
	ctx := context.Background()

	leftover := filepath.Join(base, "SWEEP-FIXTURE-BUG-SCAN")
	if err := os.MkdirAll(filepath.Join(leftover, "node_modules", "react-native", "src"), 0o755); err != nil {
		t.Fatal(err)
	}

	wt, err := CreateWorktreeWithBranch(ctx, base, "SWEEP-FIXTURE-BUG-SCAN", repoDir, "main", "noctra/sweep-fixture-bug-scan")
	if err != nil {
		t.Fatalf("CreateWorktreeWithBranch over a leftover directory: %v", err)
	}
	if _, err := os.Stat(filepath.Join(wt.Path, "README.md")); err != nil {
		t.Errorf("worktree was not checked out: %v", err)
	}
	if _, err := os.Stat(filepath.Join(wt.Path, "node_modules")); !os.IsNotExist(err) {
		t.Errorf("leftover contents survived: %v", err)
	}
}

func TestCreateWorktree_HidesStagedAgentSkillsFromGit(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	repo := t.TempDir()
	mustGit := func(dir string, args ...string) string {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, string(out))
		}
		return strings.TrimSpace(string(out))
	}
	mustGit(repo, "init", "-b", "main", "--quiet")
	mustGit(repo, "config", "user.email", "t@t")
	mustGit(repo, "config", "user.name", "T")
	mustGit(repo, "config", "commit.gpgsign", "false")
	mustGit(repo, "remote", "add", "origin", repo)
	if err := os.WriteFile(filepath.Join(repo, "README.md"), []byte("init"), 0o600); err != nil {
		t.Fatal(err)
	}
	mustGit(repo, "add", "-A")
	mustGit(repo, "commit", "-m", "init", "--quiet")
	mustGit(repo, "fetch", "origin", "--quiet")

	wt, err := CreateWorktree(context.Background(), t.TempDir(), "ENG-1", repo, "main")
	if err != nil {
		t.Fatalf("CreateWorktree: %v", err)
	}
	staged := filepath.Join(wt.Path, ".agents", "skills", "noctra-superpowers-tdd")
	if err := os.MkdirAll(staged, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(staged, "SKILL.md"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if status := mustGit(wt.Path, "status", "--porcelain", "--untracked-files=all"); status != "" {
		t.Fatalf("staged skills visible to git in the worktree: %q", status)
	}
}
