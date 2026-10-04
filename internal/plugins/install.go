package plugins

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"
)

type Installed struct {
	Name   string
	Dir    string
	Vetted bool
	Skills []string
}

const (
	markerFile   = ".noctra-plugin"
	manifestDir  = ".claude-plugin"
	skillsDir    = "skills"
	skillEntry   = "SKILL.md"
	manifestName = "noctra-"
)

const SetupTimeout = 3 * time.Minute

var licenceFiles = []string{"LICENSE", "LICENSE.md", "LICENSE.txt", "LICENCE", "LICENCE.md"}

func Dir(root string, p Plugin) string {
	return filepath.Join(root, p.Name+"@"+p.Commit[:12])
}

func IsInstalled(root string, p Plugin) bool {
	_, err := os.Stat(filepath.Join(Dir(root, p), markerFile))
	return err == nil
}

func InstallAll(ctx context.Context, root string, plugins []Plugin) ([]Installed, []error) {
	var installed []Installed
	var errs []error
	for _, p := range plugins {
		inst, err := Install(ctx, root, p)
		if err != nil {
			errs = append(errs, fmt.Errorf("plugin %s (%s@%s): %w", p.Name, p.Repo, p.Commit[:12], err))
			continue
		}
		installed = append(installed, inst)
	}
	return installed, errs
}

func Install(ctx context.Context, root string, p Plugin) (Installed, error) {
	dir := Dir(root, p)
	if IsInstalled(root, p) {
		skills, err := listSkills(dir)
		if err == nil && coversSkills(skills, p.Skills) {
			return Installed{Name: p.Name, Dir: dir, Vetted: p.Vetted, Skills: skills}, nil
		}
		if err := os.RemoveAll(dir); err != nil {
			return Installed{}, err
		}
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return Installed{}, err
	}

	clone, err := os.MkdirTemp(root, ".clone-")
	if err != nil {
		return Installed{}, err
	}
	defer func() { _ = os.RemoveAll(clone) }()
	if err := fetchCommit(ctx, clone, p); err != nil {
		return Installed{}, err
	}

	build, err := os.MkdirTemp(root, ".build-")
	if err != nil {
		return Installed{}, err
	}
	defer func() { _ = os.RemoveAll(build) }()

	skills, err := copySkills(clone, build, p.Skills)
	if err != nil {
		return Installed{}, err
	}
	if len(skills) == 0 {
		return Installed{}, errors.New("no skills found")
	}
	copyLicence(clone, build)
	if err := writeManifest(build, p); err != nil {
		return Installed{}, err
	}
	if err := os.WriteFile(filepath.Join(build, markerFile), []byte(p.Repo+"@"+p.Commit+"\n"), 0o644); err != nil {
		return Installed{}, err
	}
	if err := os.Rename(build, dir); err != nil {
		return Installed{}, err
	}
	return Installed{Name: p.Name, Dir: dir, Vetted: p.Vetted, Skills: skills}, nil
}

func cloneURL(repo string) string {
	if strings.Contains(repo, "://") || filepath.IsAbs(repo) {
		return repo
	}
	return "https://github.com/" + repo + ".git"
}

func fetchCommit(ctx context.Context, dir string, p Plugin) error {
	steps := [][]string{
		{"init", "--quiet"},
		{"fetch", "--quiet", "--depth", "1", cloneURL(p.Repo), p.Commit},
		{"checkout", "--quiet", "FETCH_HEAD"},
	}
	for _, args := range steps {
		if out, err := git(ctx, dir, args...); err != nil {
			return fmt.Errorf("git %s: %w: %s", args[0], err, strings.TrimSpace(out))
		}
	}
	head, err := git(ctx, dir, "rev-parse", "HEAD")
	if err != nil {
		return fmt.Errorf("git rev-parse: %w", err)
	}
	if got := strings.TrimSpace(head); got != p.Commit {
		return fmt.Errorf("fetched commit %s, want %s", got, p.Commit)
	}
	return nil
}

const (
	gitWaitDelay     = 5 * time.Second
	gitStallBytesSec = "1000"
	gitStallSeconds  = "30"
)

func git(ctx context.Context, dir string, args ...string) (string, error) {
	full := append([]string{"-c", "http.lowSpeedLimit=" + gitStallBytesSec, "-c", "http.lowSpeedTime=" + gitStallSeconds}, args...)
	cmd := exec.CommandContext(ctx, "git", full...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	cmd.WaitDelay = gitWaitDelay
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func copySkills(clone, build string, wanted []Skill) ([]string, error) {
	if len(wanted) == 0 {
		discovered, err := discoverSkills(clone)
		if err != nil {
			return nil, err
		}
		wanted = discovered
	}

	var names []string
	for _, s := range wanted {
		src := filepath.Join(clone, filepath.FromSlash(s.Path))
		if !isFile(filepath.Join(src, skillEntry)) {
			return nil, fmt.Errorf("%s has no %s", s.Path, skillEntry)
		}
		name := s.DirName()
		if !skillNameRe.MatchString(name) {
			return nil, fmt.Errorf("skill at %q needs a name (got %q)", s.Path, name)
		}
		if slices.Contains(names, name) {
			return nil, fmt.Errorf("two skills named %q", name)
		}
		dst := filepath.Join(build, skillsDir, name)
		var err error
		if len(s.Only) > 0 {
			err = copyOnly(src, dst, s.Only)
		} else {
			err = copyTree(src, dst, s.Exclude)
		}
		if err != nil {
			return nil, err
		}
		names = append(names, name)
	}
	slices.Sort(names)
	return names, nil
}

func discoverSkills(clone string) ([]Skill, error) {
	entries, err := os.ReadDir(filepath.Join(clone, skillsDir))
	if err != nil {
		return nil, fmt.Errorf("no %s/ directory: %w", skillsDir, err)
	}
	var skills []Skill
	for _, e := range entries {
		if e.IsDir() && isFile(filepath.Join(clone, skillsDir, e.Name(), skillEntry)) {
			skills = append(skills, Skill{Path: skillsDir + "/" + e.Name()})
		}
	}
	return skills, nil
}

func copyTree(src, dst string, exclude []string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if excluded(filepath.ToSlash(rel), exclude) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		target := filepath.Join(dst, rel)
		switch {
		case d.Type()&fs.ModeSymlink != 0:
			return nil
		case d.IsDir():
			return os.MkdirAll(target, 0o755)
		case d.Type().IsRegular():
			return copyFile(path, target)
		default:
			return nil
		}
	})
}

var skillNameRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

func copyOnly(src, dst string, files []string) error {
	if !slices.Contains(files, skillEntry) {
		files = append([]string{skillEntry}, files...)
	}
	for _, rel := range files {
		from := filepath.Join(src, filepath.FromSlash(rel))
		info, err := os.Lstat(from)
		if err != nil {
			return fmt.Errorf("%s: %w", rel, err)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("%s is not a regular file", rel)
		}
		to := filepath.Join(dst, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(to), 0o755); err != nil {
			return err
		}
		if err := copyFile(from, to); err != nil {
			return err
		}
	}
	return nil
}

func excluded(rel string, exclude []string) bool {
	for _, e := range exclude {
		if rel == e || strings.HasPrefix(rel, e+"/") {
			return true
		}
	}
	return false
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	info, err := in.Stat()
	if err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode().Perm()|0o600)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

func copyLicence(clone, build string) {
	for _, name := range licenceFiles {
		if src := filepath.Join(clone, name); isFile(src) {
			_ = copyFile(src, filepath.Join(build, name))
			return
		}
	}
}

func writeManifest(build string, p Plugin) error {
	manifest, err := json.MarshalIndent(map[string]string{
		"name":        manifestName + p.Name,
		"version":     p.Commit[:12],
		"description": fmt.Sprintf("Skills from %s@%s, pinned by Noctra", p.Repo, p.Commit[:12]),
	}, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(build, manifestDir), 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(build, manifestDir, "plugin.json"), append(manifest, '\n'), 0o644)
}

func listSkills(dir string) ([]string, error) {
	entries, err := os.ReadDir(filepath.Join(dir, skillsDir))
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() && isFile(filepath.Join(dir, skillsDir, e.Name(), skillEntry)) {
			names = append(names, e.Name())
		}
	}
	return names, nil
}

func coversSkills(have []string, wanted []Skill) bool {
	if len(wanted) == 0 {
		return len(have) > 0
	}
	if len(have) != len(wanted) {
		return false
	}
	for _, s := range wanted {
		if !slices.Contains(have, s.DirName()) {
			return false
		}
	}
	return true
}

func isFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}
