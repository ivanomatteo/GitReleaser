package service

import "testing"

func TestGlob(t *testing.T) {
	for _, tc := range []struct {
		pattern, file string
		want          bool
	}{
		{"docs/**", "docs/a/b.txt", true}, {"**/*.md", "services/a/README.md", true}, {"**/*.md", "main.go", false}, {"services/*/docs/**", "services/api/docs/a.md", true},
	} {
		if got := glob(tc.pattern, tc.file); got != tc.want {
			t.Errorf("glob(%q,%q)=%v", tc.pattern, tc.file, got)
		}
	}
}

func TestMatchesRoots(t *testing.T) {
	if !matchesRoots("services/api/main.go", []string{"services/api"}) {
		t.Fatal("expected match")
	}
	if matchesRoots("services/apix/main.go", []string{"services/api"}) {
		t.Fatal("unexpected prefix match")
	}
}
