package plugins

import (
	"fmt"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

type Skill struct {
	Path     string
	Name     string
	Only     []string
	Exclude  []string
	Requires []Requirement
}

func (s Skill) DirName() string {
	if s.Name != "" {
		return s.Name
	}
	return filepath.Base(filepath.FromSlash(s.Path))
}

type Plugin struct {
	Name    string
	Repo    string
	Commit  string
	Licence string
	Skills  []Skill
	Vetted  bool
}

type Pack struct {
	Name        string
	Description string
	Optional    bool
	Plugins     []Plugin
}

const (
	BasePack = "engineering"
	NoPacks  = "none"
)

var catalog = mustLoadCatalog(catalogJSON)

func Catalog() []Pack {
	return catalog
}

func PackNames() []string {
	names := make([]string, 0, len(catalog))
	for _, p := range catalog {
		names = append(names, p.Name)
	}
	return names
}

func OptionalPacks() []Pack {
	var out []Pack
	for _, p := range catalog {
		if p.Optional {
			out = append(out, p)
		}
	}
	return out
}

func IsOptional(name string) bool {
	p, ok := findPack(name)
	return ok && p.Optional
}

func findPack(name string) (Pack, bool) {
	for _, p := range catalog {
		if p.Name == name {
			return p, true
		}
	}
	return Pack{}, false
}

func Enabled(packs []string) bool {
	return len(packs) > 0 && !slices.Contains(packs, NoPacks)
}

func Resolve(packs, extras []string) ([]Plugin, error) {
	var selected []string
	if Enabled(packs) {
		selected = append(selected, BasePack)
		for _, name := range packs {
			if !slices.Contains(selected, name) {
				selected = append(selected, name)
			}
		}
	}

	var out []Plugin
	for _, name := range selected {
		pack, ok := findPack(name)
		if !ok {
			return nil, fmt.Errorf("unknown plugin pack %q (want %s or %s)", name, strings.Join(PackNames(), ", "), NoPacks)
		}
		for _, p := range pack.Plugins {
			p.Vetted = true
			out = merge(out, p)
		}
	}

	for _, raw := range extras {
		p, err := ParseExtra(raw)
		if err != nil {
			return nil, err
		}
		out = merge(out, p)
	}
	return out, nil
}

func merge(plugins []Plugin, p Plugin) []Plugin {
	for i := range plugins {
		if plugins[i].Name != p.Name {
			continue
		}
		for _, s := range p.Skills {
			if !slices.ContainsFunc(plugins[i].Skills, func(have Skill) bool { return have.Path == s.Path }) {
				plugins[i].Skills = append(plugins[i].Skills, s)
			}
		}
		return plugins
	}
	p.Skills = slices.Clone(p.Skills)
	return append(plugins, p)
}

var extraRe = regexp.MustCompile(`^([A-Za-z0-9_.-]+)/([A-Za-z0-9_.-]+)@([0-9a-f]{40})$`)

var unsafeNameRe = regexp.MustCompile(`[^a-z0-9-]+`)

func ParseExtra(raw string) (Plugin, error) {
	m := extraRe.FindStringSubmatch(strings.TrimSpace(raw))
	if m == nil {
		return Plugin{}, fmt.Errorf("AGENT_PLUGINS_EXTRA entry %q must be owner/repo@<40-character commit SHA>", raw)
	}
	name := unsafeNameRe.ReplaceAllString(strings.ToLower(m[1]+"-"+m[2]), "-")
	return Plugin{
		Name:   strings.Trim(name, "-"),
		Repo:   m[1] + "/" + m[2],
		Commit: m[3],
	}, nil
}
