package plugins

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"regexp"
)

//go:embed catalog.json
var catalogJSON []byte

type catalogFile struct {
	Plugins      map[string]catalogPlugin      `json:"plugins"`
	Requirements map[string]catalogRequirement `json:"requirements"`
	Packs        []catalogPack                 `json:"packs"`
}

type catalogPlugin struct {
	Repo    string `json:"repo"`
	Commit  string `json:"commit"`
	Licence string `json:"licence"`
}

type catalogRequirement struct {
	Name    string   `json:"name"`
	Command []string `json:"command"`
	Hint    string   `json:"hint"`
}

type catalogPack struct {
	Name        string           `json:"name"`
	Description string           `json:"description"`
	Optional    bool             `json:"optional"`
	Include     []catalogInclude `json:"include"`
}

type catalogInclude struct {
	Plugin string         `json:"plugin"`
	Skills []catalogSkill `json:"skills"`
}

type catalogSkill struct {
	Path     string   `json:"path"`
	Name     string   `json:"name"`
	Only     []string `json:"only"`
	Exclude  []string `json:"exclude"`
	Requires []string `json:"requires"`
}

var commitRe = regexp.MustCompile(`^[0-9a-f]{40}$`)

func mustLoadCatalog(raw []byte) []Pack {
	packs, err := loadCatalog(raw)
	if err != nil {
		panic(fmt.Sprintf("embedded plugin catalogue is invalid: %v", err))
	}
	return packs
}

func loadCatalog(raw []byte) ([]Pack, error) {
	var file catalogFile
	if err := json.Unmarshal(raw, &file); err != nil {
		return nil, err
	}
	for name, p := range file.Plugins {
		if !commitRe.MatchString(p.Commit) {
			return nil, fmt.Errorf("plugin %s: commit %q is not a full SHA", name, p.Commit)
		}
		if p.Repo == "" || p.Licence == "" {
			return nil, fmt.Errorf("plugin %s: repo and licence are required", name)
		}
	}

	packs := make([]Pack, 0, len(file.Packs))
	for _, cp := range file.Packs {
		pack := Pack{Name: cp.Name, Description: cp.Description, Optional: cp.Optional}
		for _, inc := range cp.Include {
			meta, ok := file.Plugins[inc.Plugin]
			if !ok {
				return nil, fmt.Errorf("pack %s: unknown plugin %q", cp.Name, inc.Plugin)
			}
			plugin := Plugin{Name: inc.Plugin, Repo: meta.Repo, Commit: meta.Commit, Licence: meta.Licence}
			for _, cs := range inc.Skills {
				skill := Skill{Path: cs.Path, Name: cs.Name, Only: cs.Only, Exclude: cs.Exclude}
				for _, key := range cs.Requires {
					req, ok := file.Requirements[key]
					if !ok {
						return nil, fmt.Errorf("pack %s: plugin %s: unknown requirement %q", cp.Name, inc.Plugin, key)
					}
					skill.Requires = append(skill.Requires, Requirement(req))
				}
				plugin.Skills = append(plugin.Skills, skill)
			}
			pack.Plugins = append(pack.Plugins, plugin)
		}
		packs = append(packs, pack)
	}
	return packs, nil
}
