package version

import "testing"

func TestStrictParsingAndBumps(t *testing.T) {
	v, err := Parse("1.10.3")
	if err != nil {
		t.Fatal(err)
	}
	checks := map[string]string{"patch": "1.10.4", "minor": "1.11.0", "major": "2.0.0"}
	for bump, want := range checks {
		got, err := v.Bump(bump)
		if err != nil || got.String() != want {
			t.Fatalf("%s: got %s, %v", bump, got.String(), err)
		}
	}
	for _, bad := range []string{"1.2", "v1.2.3", "01.2.3", "foo"} {
		if _, err := Parse(bad); err == nil {
			t.Errorf("accepted %q", bad)
		}
	}
}

func TestPrereleaseOrderingAndBumps(t *testing.T) {
	rc, _ := Parse("2.1.0-rc.1")
	final, _ := Parse("2.1.0")
	if Compare(rc, final) >= 0 {
		t.Fatal("a prerelease must precede its final version")
	}
	for bump, want := range map[string]string{"patch": "2.1.0", "minor": "2.2.0", "major": "3.0.0"} {
		if got, _ := rc.Bump(bump); got.String() != want {
			t.Errorf("2.1.0-rc.1 + %s = %s, want %s", bump, got, want)
		}
	}
}
