package plugins

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

const (
	StagedSkillsDir     = ".agents/skills"
	stagedSkillPrefix   = "noctra-"
	StagedSkillsExclude = "/" + StagedSkillsDir + "/" + stagedSkillPrefix + "*/"
	stagedMarker        = ".noctra-staged"
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
		}
	}
	return cleanup, errors.Join(errs...)
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
