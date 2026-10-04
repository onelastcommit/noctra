package plugins

import (
	"regexp"
	"slices"
	"strings"
	"testing"
)

var shaRe = regexp.MustCompile(`^[0-9a-f]{40}$`)

func TestCatalog_EveryPluginIsPinnedAndLicensed(t *testing.T) {
	for _, pack := range Catalog() {
		if pack.Description == "" {
			t.Errorf("pack %s has no description", pack.Name)
		}
		for _, p := range pack.Plugins {
			if !shaRe.MatchString(p.Commit) {
				t.Errorf("%s/%s: commit %q is not a full SHA", pack.Name, p.Name, p.Commit)
			}
			if p.Licence == "" {
				t.Errorf("%s/%s: no licence recorded", pack.Name, p.Name)
			}
			if len(p.Skills) == 0 {
				t.Errorf("%s/%s: lists no skills", pack.Name, p.Name)
			}
			if strings.Count(p.Repo, "/") != 1 {
				t.Errorf("%s/%s: repo %q is not owner/name", pack.Name, p.Name, p.Repo)
			}
		}
	}
}

func TestCatalog_SharedPluginsUseOneCommit(t *testing.T) {
	commits := map[string]string{}
	for _, pack := range Catalog() {
		for _, p := range pack.Plugins {
			if c, ok := commits[p.Name]; ok && c != p.Commit {
				t.Errorf("plugin %s is pinned to %s and %s", p.Name, c, p.Commit)
			}
			commits[p.Name] = p.Commit
		}
	}
}

func TestCatalog_BasePackExists(t *testing.T) {
	if !slices.Contains(PackNames(), BasePack) {
		t.Fatalf("catalogue has no %q pack", BasePack)
	}
}

func skillPaths(p Plugin) []string {
	var out []string
	for _, s := range p.Skills {
		out = append(out, s.Path)
	}
	return out
}

func names(ps []Plugin) []string {
	var out []string
	for _, p := range ps {
		out = append(out, p.Name)
	}
	return out
}

func TestResolve(t *testing.T) {
	t.Run("nothing selected", func(t *testing.T) {
		got, err := Resolve(nil, nil)
		if err != nil || len(got) != 0 {
			t.Fatalf("Resolve(nil) = %v, %v", got, err)
		}
	})

	t.Run("none disables packs", func(t *testing.T) {
		got, err := Resolve([]string{NoPacks}, nil)
		if err != nil || len(got) != 0 {
			t.Fatalf("Resolve(none) = %v, %v", got, err)
		}
	})

	t.Run("any pack adds the base pack", func(t *testing.T) {
		got, err := Resolve([]string{"frontend"}, nil)
		if err != nil {
			t.Fatal(err)
		}
		if !slices.Contains(names(got), "superpowers") || !slices.Contains(names(got), "impeccable") {
			t.Fatalf("got %v, want base + frontend plugins", names(got))
		}
		for _, p := range got {
			if !p.Vetted {
				t.Errorf("%s should be vetted", p.Name)
			}
		}
	})

	t.Run("a plugin in two packs merges its skills", func(t *testing.T) {
		got, err := Resolve([]string{"backend"}, nil)
		if err != nil {
			t.Fatal(err)
		}
		var agentSkills []Plugin
		for _, p := range got {
			if p.Name == "agent-skills" {
				agentSkills = append(agentSkills, p)
			}
		}
		if len(agentSkills) != 1 {
			t.Fatalf("agent-skills appears %d times", len(agentSkills))
		}
		paths := skillPaths(agentSkills[0])
		if !slices.Contains(paths, "skills/code-simplification") || !slices.Contains(paths, "skills/api-and-interface-design") {
			t.Fatalf("merged skills = %v", paths)
		}
	})

	t.Run("merging never mutates the catalogue", func(t *testing.T) {
		if _, err := Resolve([]string{"backend"}, nil); err != nil {
			t.Fatal(err)
		}
		base, _ := findPack(BasePack)
		for _, p := range base.Plugins {
			if p.Name == "agent-skills" && len(p.Skills) != 1 {
				t.Fatalf("catalogue entry grew to %v", skillPaths(p))
			}
		}
	})

	t.Run("unknown pack", func(t *testing.T) {
		if _, err := Resolve([]string{"mobile"}, nil); err == nil {
			t.Fatal("want an error for an unknown pack")
		}
	})

	t.Run("extras are unvetted and work without packs", func(t *testing.T) {
		got, err := Resolve(nil, []string{"Acme/Cool.Skills@" + strings.Repeat("a", 40)})
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 1 || got[0].Vetted || got[0].Name != "acme-cool-skills" {
			t.Fatalf("got %+v", got)
		}
	})
}

func TestParseExtra_RequiresFullSHA(t *testing.T) {
	for _, raw := range []string{
		"acme/skills",
		"acme/skills@main",
		"acme/skills@v1.2.3",
		"acme/skills@abc1234",
		"https://github.com/acme/skills@" + strings.Repeat("a", 40),
	} {
		if _, err := ParseExtra(raw); err == nil {
			t.Errorf("ParseExtra(%q) accepted an unpinned or malformed ref", raw)
		}
	}
	p, err := ParseExtra(" acme/skills@" + strings.Repeat("b", 40) + " ")
	if err != nil {
		t.Fatal(err)
	}
	if p.Repo != "acme/skills" || p.Commit != strings.Repeat("b", 40) {
		t.Fatalf("got %+v", p)
	}
}

func TestStackPacksAreNotOptional(t *testing.T) {
	for _, name := range []string{BasePack, "frontend", "backend"} {
		if IsOptional(name) {
			t.Errorf("%s should be a stack pack", name)
		}
	}
	if IsOptional("no-such-pack") {
		t.Error("an unknown pack is not optional")
	}
}

func TestOptionalPacks(t *testing.T) {
	var got []string
	for _, p := range OptionalPacks() {
		got = append(got, p.Name)
	}
	if !slices.Equal(got, []string{"security", "content"}) {
		t.Fatalf("optional packs = %v", got)
	}
}

func TestResolve_OptionalPackAloneStillAddsBase(t *testing.T) {
	got, err := Resolve([]string{"security"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(names(got), "superpowers") || !slices.Contains(names(got), "trailofbits-skills") {
		t.Fatalf("got %v", names(got))
	}
}

func TestCatalog_RuntimeSkillsDeclareRequirements(t *testing.T) {
	for _, pack := range Catalog() {
		for _, p := range pack.Plugins {
			for _, s := range p.Skills {
				for _, r := range s.Requires {
					if r.Name == "" || len(r.Command) == 0 || r.Hint == "" {
						t.Errorf("%s/%s has an incomplete requirement %+v", p.Name, s.DirName(), r)
					}
				}
				if s.DirName() == "webapp-testing" && len(s.Requires) == 0 {
					t.Error("webapp-testing must be gated on Playwright")
				}
			}
		}
	}
}
