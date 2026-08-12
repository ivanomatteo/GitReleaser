package version

import (
	"fmt"

	masterminds "github.com/Masterminds/semver/v3"
)

type Version struct{ value *masterminds.Version }

func Parse(s string) (Version, error) {
	v, err := masterminds.StrictNewVersion(s)
	if err != nil {
		return Version{}, fmt.Errorf("invalid semantic version %q: %w", s, err)
	}
	return Version{value: v}, nil
}

func Compare(a, b Version) int       { return a.value.Compare(b.value) }
func (v Version) String() string     { return v.value.String() }
func (v Version) BumpPatch() Version { n := v.value.IncPatch(); return Version{value: &n} }
func (v Version) BumpMinor() Version { n := v.value.IncMinor(); return Version{value: &n} }
func (v Version) BumpMajor() Version { n := v.value.IncMajor(); return Version{value: &n} }

func (v Version) Bump(kind string) (Version, error) {
	switch kind {
	case "patch":
		return v.BumpPatch(), nil
	case "minor":
		return v.BumpMinor(), nil
	case "major":
		return v.BumpMajor(), nil
	default:
		return Version{}, fmt.Errorf("invalid bump %q (expected patch, minor, or major)", kind)
	}
}
