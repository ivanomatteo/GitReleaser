package git

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

type ChangedFile struct{ OldPath, NewPath string }

type Client struct{ Dir string }

func (g Client) run(args ...string) (string, error) {
	out, err := g.runRaw(args...)
	return strings.TrimSpace(out), err
}

// runRaw returns stdout untouched, for NUL-separated output where whitespace is significant.
func (g Client) runRaw(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = g.Dir
	var out, stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("git %s: %s", args[0], msg)
	}
	return out.String(), nil
}

func (g Client) Tags(pattern string) ([]string, error) {
	o, err := g.run("tag", "--list", pattern)
	if err != nil {
		return nil, err
	}
	if o == "" {
		return nil, nil
	}
	return strings.Split(o, "\n"), nil
}
func (g Client) Resolve(ref string) (string, error) {
	return g.run("rev-parse", "--verify", ref+"^{commit}")
}
func (g Client) IsAncestor(a, b string) (bool, error) {
	cmd := exec.Command("git", "merge-base", "--is-ancestor", a, b)
	cmd.Dir = g.Dir
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err == nil {
		return true, nil
	}
	if ee, ok := err.(*exec.ExitError); ok && ee.ExitCode() == 1 {
		return false, nil
	}
	return false, fmt.Errorf("git merge-base: %s", strings.TrimSpace(stderr.String()))
}
func (g Client) DiffFiles(from, to string) ([]ChangedFile, error) {
	// diff-tree is plumbing: its output does not depend on user configuration, and -z
	// disables path quoting so that non-ASCII or unusual file names are reported verbatim.
	o, err := g.runRaw("diff-tree", "-r", "-z", "--name-status", "-M", "--no-commit-id", from, to)
	if err != nil {
		return nil, err
	}
	if o == "" {
		return nil, nil
	}
	fields := strings.Split(strings.TrimSuffix(o, "\x00"), "\x00")
	var files []ChangedFile
	for i := 0; i < len(fields); {
		status := fields[i]
		if status != "" && (status[0] == 'R' || status[0] == 'C') {
			if i+2 >= len(fields) {
				return nil, fmt.Errorf("unexpected git rename output %q", status)
			}
			files = append(files, ChangedFile{OldPath: fields[i+1], NewPath: fields[i+2]})
			i += 3
			continue
		}
		if status == "" || i+1 >= len(fields) {
			return nil, fmt.Errorf("unexpected git diff-tree output %q", o)
		}
		files = append(files, ChangedFile{OldPath: fields[i+1], NewPath: fields[i+1]})
		i += 2
	}
	return files, nil
}
func (g Client) IsClean() (bool, error) {
	// Untracked files cannot end up in a tag created on HEAD, so they do not block a release.
	o, err := g.run("status", "--porcelain", "--untracked-files=no")
	return o == "", err
}
func (g Client) IsShallow() (bool, error) {
	o, err := g.run("rev-parse", "--is-shallow-repository")
	return o == "true", err
}
func (g Client) CreateTag(tag, commit, message string) error {
	_, err := g.run("tag", "-a", tag, commit, "-m", message)
	return err
}
func (g Client) DeleteTag(tag string) error {
	_, err := g.run("tag", "-d", tag)
	return err
}

// PushTags publishes all tags atomically: either every tag reaches the remote or none does.
func (g Client) PushTags(remote string, tags ...string) error {
	args := []string{"push", "--atomic", remote}
	for _, t := range tags {
		args = append(args, "refs/tags/"+t)
	}
	_, err := g.run(args...)
	return err
}
func (g Client) CheckRepository() error { _, err := g.run("rev-parse", "--git-dir"); return err }

var ErrIncompleteHistory = errors.New("required history is unavailable (the clone may be shallow); fetch tags and sufficient history explicitly")
