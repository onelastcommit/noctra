package plugins

import (
	"strings"
	"testing"
)

func TestLoadCatalog_EmbeddedCatalogueLoads(t *testing.T) {
	packs, err := loadCatalog(catalogJSON)
	if err != nil {
		t.Fatalf("embedded catalogue: %v", err)
	}
	if len(packs) == 0 {
		t.Fatal("embedded catalogue has no packs")
	}
}

func TestLoadCatalog_Rejects(t *testing.T) {
	sha := strings.Repeat("a", 40)
	cases := map[string]string{
		"malformed json":      `{"plugins":`,
		"short commit":        `{"plugins":{"p":{"repo":"o/r","commit":"abc123","licence":"MIT"}},"packs":[]}`,
		"missing licence":     `{"plugins":{"p":{"repo":"o/r","commit":"` + sha + `"}},"packs":[]}`,
		"unknown plugin":      `{"plugins":{},"packs":[{"name":"x","include":[{"plugin":"ghost","skills":[{"path":"skills/a"}]}]}]}`,
		"unknown requirement": `{"plugins":{"p":{"repo":"o/r","commit":"` + sha + `","licence":"MIT"}},"packs":[{"name":"x","include":[{"plugin":"p","skills":[{"path":"skills/a","requires":["ghost"]}]}]}]}`,
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := loadCatalog([]byte(raw)); err == nil {
				t.Fatal("want an error")
			}
		})
	}
}

func TestLoadCatalog_SharesOnePinAcrossPacks(t *testing.T) {
	sha := strings.Repeat("b", 40)
	raw := `{"plugins":{"p":{"repo":"o/r","commit":"` + sha + `","licence":"MIT"}},"packs":[
		{"name":"one","include":[{"plugin":"p","skills":[{"path":"skills/a"}]}]},
		{"name":"two","include":[{"plugin":"p","skills":[{"path":"skills/b"}]}]}]}`
	packs, err := loadCatalog([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	if packs[0].Plugins[0].Commit != sha || packs[1].Plugins[0].Commit != sha {
		t.Fatal("both packs should read the single pin")
	}
}
