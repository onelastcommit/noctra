package agent

import (
	"strings"
	"testing"
)

func TestLanguageSection(t *testing.T) {
	cases := []struct {
		variant, want, notWant string
	}{
		{"british", "British English", "American"},
		{"american", "American English", "British"},
		{"", "British English", "American"},
		{"klingon", "British English", "American"},
	}
	for _, c := range cases {
		got := LanguageSection(c.variant)
		if !strings.Contains(got, c.want) || strings.Contains(got, c.notWant) {
			t.Errorf("LanguageSection(%q) = %q, want %q and not %q", c.variant, got, c.want, c.notWant)
		}
		if !strings.Contains(got, "CSS properties") {
			t.Errorf("LanguageSection(%q) should protect spec-fixed spellings", c.variant)
		}
	}
	if !strings.Contains(LanguageSection("british"), "colour") || !strings.Contains(LanguageSection("american"), "color") {
		t.Error("each variant should give spelling examples in that variant")
	}
}
