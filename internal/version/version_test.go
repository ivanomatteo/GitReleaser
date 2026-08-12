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
