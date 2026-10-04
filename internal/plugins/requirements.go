package plugins

import (
	"context"
	"fmt"
	"os/exec"
	"slices"
	"strings"
	"time"
)

type Requirement struct {
	Name    string
	Command []string
	Hint    string
}

type Unmet struct {
	Plugin      string
	Skill       string
	Requirement Requirement
	Err         error
}

func (u Unmet) String() string {
	return fmt.Sprintf("%s/%s needs %s (%v); %s", u.Plugin, u.Skill, u.Requirement.Name, u.Err, u.Requirement.Hint)
}

const requirementTimeout = 90 * time.Second

func CheckRequirements(ctx context.Context, plugins []Plugin) ([]Plugin, []Unmet) {
	results := map[string]error{}
	var unmet []Unmet
	var usable []Plugin
	for _, p := range plugins {
		kept := p
		kept.Skills = nil
		for _, s := range p.Skills {
			if missing, req, err := firstUnmet(ctx, s.Requires, results); missing {
				unmet = append(unmet, Unmet{Plugin: p.Name, Skill: s.DirName(), Requirement: req, Err: err})
				continue
			}
			kept.Skills = append(kept.Skills, s)
		}
		if len(p.Skills) > 0 && len(kept.Skills) == 0 {
			continue
		}
		usable = append(usable, kept)
	}
	return usable, unmet
}

func firstUnmet(ctx context.Context, reqs []Requirement, results map[string]error) (bool, Requirement, error) {
	for _, r := range reqs {
		key := strings.Join(r.Command, "\x00")
		err, seen := results[key]
		if !seen {
			err = runRequirement(ctx, r)
			results[key] = err
		}
		if err != nil {
			return true, r, err
		}
	}
	return false, Requirement{}, nil
}

func runRequirement(ctx context.Context, r Requirement) error {
	if len(r.Command) == 0 {
		return fmt.Errorf("no check command")
	}
	ctx, cancel := context.WithTimeout(ctx, requirementTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, r.Command[0], r.Command[1:]...)
	out, err := cmd.CombinedOutput()
	if err == nil {
		return nil
	}
	if ctx.Err() != nil {
		return fmt.Errorf("check timed out")
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if last := strings.TrimSpace(lines[len(lines)-1]); last != "" {
		return fmt.Errorf("%s", last)
	}
	return err
}

func SkillNames(unmet []Unmet) []string {
	names := make([]string, 0, len(unmet))
	for _, u := range unmet {
		if !slices.Contains(names, u.Skill) {
			names = append(names, u.Skill)
		}
	}
	return names
}
