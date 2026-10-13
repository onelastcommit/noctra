package plugins

import (
	"fmt"
	"path/filepath"
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

type Installed struct {
	Name   string
	Dir    string
	Vetted bool
	Skills []string
}

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
