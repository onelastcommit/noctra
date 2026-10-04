package plugins

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

const (
	StagedSkillsDir     = ".agents/skills"
	stagedSkillPrefix   = "noctra-"
	StagedSkillsExclude = "/" + StagedSkillsDir + "/" + stagedSkillPrefix + "*/"
	stagedMarker        = ".noctra-staged"
	stageCheckTimeout   = time.Minute
)

func StagedSkillName(pluginDir, skill string) string {
	plugin, _, _ := strings.Cut(filepath.Base(pluginDir), "@")
	return stagedSkillPrefix + plugin + "-" + skill
}

func Stage(workdir string, pluginDirs []string) (func(), error) {
	if len(pluginDirs) == 0 {
		return func() {}, nil
	}

	target := filepath.Join(workdir, filepath.FromSlash(StagedSkillsDir))
	created := missingAncestors(workdir, target)
	if err := os.MkdirAll(target, 0o755); err != nil {
		return func() {}, err
	}

	var staged []string
	cleanup := func() {
		for _, dir := range staged {
			_ = os.RemoveAll(dir)
		}
		for _, dir := range created {
			_ = os.Remove(dir)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), stageCheckTimeout)
	defer cancel()
	checkVisible := insideWorkTree(ctx, workdir)

	var errs []error
	for _, pluginDir := range pluginDirs {
		skills, err := listSkills(pluginDir)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		for _, skill := range skills {
			dst := filepath.Join(target, StagedSkillName(pluginDir, skill))
			if exists(dst) {
				if !isFile(filepath.Join(dst, stagedMarker)) {
					errs = append(errs, fmt.Errorf("%s already exists and isn't Noctra's; leaving it alone and skipping that skill", dst))
					continue
				}
				if err := os.RemoveAll(dst); err != nil {
					errs = append(errs, err)
					continue
				}
			}
			staged = append(staged, dst)
			if err := copyTree(filepath.Join(pluginDir, skillsDir, skill), dst, nil); err != nil {
				errs = append(errs, err)
				continue
			}
			if err := os.WriteFile(filepath.Join(dst, stagedMarker), nil, 0o644); err != nil {
				errs = append(errs, err)
			}
			if checkVisible {
				if err := ensureHiddenFromGit(ctx, workdir, dst); err != nil {
					_ = os.RemoveAll(dst)
					staged = staged[:len(staged)-1]
					errs = append(errs, err)
				}
			}
		}
	}
	return cleanup, errors.Join(errs...)
}

func insideWorkTree(ctx context.Context, dir string) bool {
	out, err := git(ctx, dir, "rev-parse", "--is-inside-work-tree")
	return err == nil && strings.TrimSpace(out) == "true"
}

func ensureHiddenFromGit(ctx context.Context, workdir, dst string) error {
	rel, err := filepath.Rel(workdir, dst)
	if err != nil {
		return err
	}
	out, err := git(ctx, workdir, "ls-files", "--others", "--exclude-standard", "--", filepath.ToSlash(rel))
	if err != nil {
		return fmt.Errorf("could not confirm git ignores %s, so it wasn't staged: %w", rel, err)
	}
	if strings.TrimSpace(out) != "" {
		return fmt.Errorf("%s would be visible to git (a .gitignore re-includes it), so it wasn't staged", rel)
	}
	return nil
}

func exists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

func missingAncestors(base, path string) []string {
	var missing []string
	for dir := path; dir != base && strings.HasPrefix(dir, base); dir = filepath.Dir(dir) {
		if _, err := os.Stat(dir); err == nil {
			break
		}
		missing = append(missing, dir)
	}
	slices.Sort(missing)
	slices.Reverse(missing)
	return missing
}
