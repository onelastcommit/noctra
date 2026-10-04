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

const (
	superpowersCommit = "8ca22dba9a94f28898bbce59f2537ff4d87c747d"
	agentSkillsCommit = "1401c8b8030e023baeebb31781a6653fe8e93026"
	mattpocockCommit  = "d81f3a183412e71a5b1e84ca21bc1a35eea03a60"
	ponytailCommit    = "c982cd411abb53323c4baa1baa3c2f020b8d0b08"
	impeccableCommit  = "e103efe779e2dd01274dabae83531fef00bf2563"
	tasteSkillCommit  = "ce26fc25c0e5e8cab638f883de62d9a86ee5e45b"
	gsapSkillsCommit  = "aed9cfd3277740755f6bfc1155c7aa645403b760"
	anthropicCommit   = "8a1541c4a3ffa5a20a5a91de0dcf3f0bab1d1ef4"
	vercelCommit      = "063bee94c3f4df8453406c830b0a7df0f2860278"
	trailOfBitsCommit = "82fe8226252622fa807643bdca1710901198553a"
	marketingCommit   = "dda3841f0b294e01e93b1541486beefbfab0915e"
	humanizerCommit   = "225a6f39ac85f76ee48dbad772ea4abe4ed6c9d8"
)

var playwright = Requirement{
	Name: "Python Playwright with Chromium",
	Command: []string{"python3", "-c",
		"from playwright.sync_api import sync_playwright\nwith sync_playwright() as p:\n    p.chromium.launch().close()"},
	Hint: "install it with `pip install playwright && python3 -m playwright install --with-deps chromium`",
}

var catalog = []Pack{
	{
		Name:        BasePack,
		Description: "test-first work, root-cause debugging, verification before done, minimal diffs",
		Plugins: []Plugin{
			{
				Name:    "superpowers",
				Repo:    "obra/superpowers",
				Commit:  superpowersCommit,
				Licence: "MIT",
				Skills: []Skill{
					{Path: "skills/test-driven-development"},
					{Path: "skills/systematic-debugging", Exclude: []string{"CREATION-LOG.md", "test-academic.md", "test-pressure-1.md", "test-pressure-2.md", "test-pressure-3.md"}},
					{Path: "skills/verification-before-completion"},
					{Path: "skills/receiving-code-review"},
				},
			},
			{
				Name:    "agent-skills",
				Repo:    "addyosmani/agent-skills",
				Commit:  agentSkillsCommit,
				Licence: "MIT",
				Skills:  []Skill{{Path: "skills/code-simplification"}},
			},
			{
				Name:    "ponytail",
				Repo:    "DietrichGebert/ponytail",
				Commit:  ponytailCommit,
				Licence: "MIT",
				Skills:  []Skill{{Path: "skills/ponytail"}},
			},
		},
	},
	{
		Name:        "backend",
		Description: "API and interface design, deep modules",
		Plugins: []Plugin{
			{
				Name:    "agent-skills",
				Repo:    "addyosmani/agent-skills",
				Commit:  agentSkillsCommit,
				Licence: "MIT",
				Skills:  []Skill{{Path: "skills/api-and-interface-design"}},
			},
			{
				Name:    "mattpocock-skills",
				Repo:    "mattpocock/skills",
				Commit:  mattpocockCommit,
				Licence: "MIT",
				Skills:  []Skill{{Path: "skills/engineering/codebase-design"}},
			},
		},
	},
	{
		Name:        "frontend",
		Description: "design direction and craft, anti-template UI, GSAP motion, React performance, UI checks in a real browser",
		Plugins: []Plugin{
			{
				Name:    "impeccable",
				Repo:    "pbakaus/impeccable",
				Commit:  impeccableCommit,
				Licence: "Apache-2.0",
				Skills:  []Skill{{Path: "plugin/skills/impeccable", Exclude: []string{"scripts"}}},
			},
			{
				Name:    "taste-skill",
				Repo:    "Leonxlnx/taste-skill",
				Commit:  tasteSkillCommit,
				Licence: "MIT",
				Skills:  []Skill{{Path: "skills/taste-skill"}},
			},
			{
				Name:    "gsap-skills",
				Repo:    "greensock/gsap-skills",
				Commit:  gsapSkillsCommit,
				Licence: "MIT",
				Skills: []Skill{
					{Path: "skills/gsap-core"},
					{Path: "skills/gsap-timeline"},
					{Path: "skills/gsap-scrolltrigger"},
					{Path: "skills/gsap-react"},
					{Path: "skills/gsap-performance"},
				},
			},
			{
				Name:    "anthropic-skills",
				Repo:    "anthropics/skills",
				Commit:  anthropicCommit,
				Licence: "Apache-2.0",
				Skills:  []Skill{{Path: "skills/webapp-testing", Requires: []Requirement{playwright}}},
			},
			{
				Name:    "vercel-agent-skills",
				Repo:    "vercel-labs/agent-skills",
				Commit:  vercelCommit,
				Licence: "MIT",
				Skills:  []Skill{{Path: "skills/react-best-practices", Exclude: []string{"AGENTS.md", "README.md"}}},
			},
		},
	},
	{
		Name:        "security",
		Description: "security review of the diff, dangerous APIs and unsafe defaults",
		Optional:    true,
		Plugins: []Plugin{
			{
				Name:    "trailofbits-skills",
				Repo:    "trailofbits/skills",
				Commit:  trailOfBitsCommit,
				Licence: "CC-BY-SA-4.0",
				Skills: []Skill{
					{Path: "plugins/differential-review/skills/differential-review", Exclude: []string{"agents", "assets"}},
					{Path: "plugins/sharp-edges/skills/sharp-edges", Exclude: []string{"agents", "assets"}},
				},
			},
		},
	},
	{
		Name:        "content",
		Description: "user-facing copy that converts and reads like a person wrote it",
		Optional:    true,
		Plugins: []Plugin{
			{
				Name:    "marketingskills",
				Repo:    "coreyhaines31/marketingskills",
				Commit:  marketingCommit,
				Licence: "MIT",
				Skills:  []Skill{{Path: "skills/copywriting", Exclude: []string{"evals"}}},
			},
			{
				Name:    "humanizer",
				Repo:    "blader/humanizer",
				Commit:  humanizerCommit,
				Licence: "MIT",
				Skills:  []Skill{{Path: ".", Name: "humanizer", Only: []string{"SKILL.md"}}},
			},
		},
	},
}

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
