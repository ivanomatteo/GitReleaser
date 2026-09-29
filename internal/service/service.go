package service

import (
	"fmt"
	"path"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/ivano/gitreleaser/internal/config"
	gitclient "github.com/ivano/gitreleaser/internal/git"
	"github.com/ivano/gitreleaser/internal/version"
)

type Release struct {
	Version version.Version
	Tag     string
}
type Status struct {
	Name     string
	Release  *Release
	Affected bool
	Files    []string
}
type Engine struct {
	Config  config.Config
	Git     gitclient.Client
	Verbose bool
	Warn    func(string)
}

func (e Engine) Names() []string {
	n := make([]string, 0, len(e.Config.Services))
	for s := range e.Config.Services {
		n = append(n, s)
	}
	sort.Strings(n)
	return n
}
func (e Engine) RequireService(name string) (config.Service, error) {
	s, ok := e.Config.Services[name]
	if !ok {
		return s, fmt.Errorf("unknown service %q", name)
	}
	return s, nil
}

func (e Engine) Latest(name string) (*Release, error) {
	if _, err := e.RequireService(name); err != nil {
		return nil, err
	}
	tags, err := e.Git.Tags(name + "/v*")
	if err != nil {
		return nil, err
	}
	var best *Release
	for _, tag := range tags {
		prefix := name + "/v"
		raw := strings.TrimPrefix(tag, prefix)
		v, parseErr := version.Parse(raw)
		if parseErr != nil || !strings.HasPrefix(tag, prefix) {
			if e.Verbose && e.Warn != nil {
				e.Warn("ignoring malformed tag " + tag)
			}
			continue
		}
		r := &Release{Version: v, Tag: tag}
		if best == nil || version.Compare(r.Version, best.Version) > 0 {
			best = r
		}
	}
	return best, nil
}

func (e Engine) Status(name string) (Status, error) {
	svc, err := e.RequireService(name)
	if err != nil {
		return Status{}, err
	}
	rel, err := e.Latest(name)
	if err != nil {
		return Status{}, err
	}
	st := Status{Name: name, Release: rel}
	if rel == nil {
		st.Affected = true
		return st, nil
	}
	if _, err = e.Git.Resolve(rel.Tag); err != nil {
		return st, gitclient.ErrIncompleteHistory
	}
	ancestor, err := e.Git.IsAncestor(rel.Tag, "HEAD")
	if err != nil {
		return st, err
	}
	if !ancestor {
		return st, NotAncestorError(e.Git, rel.Tag)
	}
	changed, err := e.Git.DiffFiles(rel.Tag, "HEAD")
	if err != nil {
		return st, err
	}
	roots := append(append([]string{}, svc.Paths...), svc.Dependencies...)
	patterns := append(append([]string{}, e.Config.Ignore...), svc.Ignore...)
	seen := map[string]bool{}
	for _, f := range changed {
		// For a rename both paths are evaluated; only the ones relevant to the service are reported.
		for _, p := range []string{f.OldPath, f.NewPath} {
			if !seen[p] && matchesRoots(p, roots) && !ignored(p, patterns) {
				st.Files = append(st.Files, p)
				seen[p] = true
			}
		}
	}
	sort.Strings(st.Files)
	st.Affected = len(st.Files) > 0
	return st, nil
}

// NotAncestorError explains why a release tag is not reachable from HEAD: in a shallow
// clone the missing history is reported as such instead of as a diverged tag.
func NotAncestorError(g gitclient.Client, tag string) error {
	if shallow, err := g.IsShallow(); err == nil && shallow {
		return gitclient.ErrIncompleteHistory
	}
	return fmt.Errorf("release tag %s is not an ancestor of HEAD", tag)
}

func matchesRoots(file string, roots []string) bool {
	for _, r := range roots {
		r = strings.TrimSuffix(path.Clean(r), "/")
		if file == r || strings.HasPrefix(file, r+"/") {
			return true
		}
	}
	return false
}
func ignored(file string, patterns []string) bool {
	for _, p := range patterns {
		if glob(p, file) {
			return true
		}
	}
	return false
}

var globCache sync.Map // pattern -> *regexp.Regexp

func glob(pattern, file string) bool {
	re, ok := globCache.Load(pattern)
	if !ok {
		re, _ = globCache.LoadOrStore(pattern, compileGlob(pattern))
	}
	return re.(*regexp.Regexp).MatchString(path.Clean(file))
}

// compileGlob supports "**" (any number of directories), "*" and "?" (within a
// single path segment); every other character is literal.
func compileGlob(pattern string) *regexp.Regexp {
	pattern = path.Clean(pattern)
	var b strings.Builder
	b.WriteByte('^')
	for i := 0; i < len(pattern); {
		switch {
		case i+2 < len(pattern) && pattern[i:i+3] == "**/":
			b.WriteString("(?:.*/)?")
			i += 3
		case i+1 < len(pattern) && pattern[i:i+2] == "**":
			b.WriteString(".*")
			i += 2
		case pattern[i] == '*':
			b.WriteString("[^/]*")
			i++
		case pattern[i] == '?':
			b.WriteString("[^/]")
			i++
		default:
			b.WriteString(regexp.QuoteMeta(pattern[i : i+1]))
			i++
		}
	}
	b.WriteByte('$')
	return regexp.MustCompile(b.String())
}
