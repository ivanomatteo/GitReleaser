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
	return strings.TrimSpace(out.String()), nil
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
	o, err := g.run("diff", "--name-status", "-M", "--no-ext-diff", from+".."+to)
	if err != nil {
		return nil, err
	}
	if o == "" {
		return nil, nil
	}
	var files []ChangedFile
	for _, line := range strings.Split(o, "\n") {
		f := strings.Split(line, "\t")
		if len(f) < 2 {
			return nil, fmt.Errorf("unexpected git diff output %q", line)
		}
		if strings.HasPrefix(f[0], "R") || strings.HasPrefix(f[0], "C") {
			if len(f) != 3 {
				return nil, fmt.Errorf("unexpected git rename output %q", line)
			}
			files = append(files, ChangedFile{OldPath: f[1], NewPath: f[2]})
		} else {
			files = append(files, ChangedFile{OldPath: f[1], NewPath: f[1]})
		}
	}
	return files, nil
}
func (g Client) IsClean() (bool, error) {
	o, err := g.run("status", "--porcelain")
	return o == "", err
}
func (g Client) CreateTag(tag, commit, message string) error {
	_, err := g.run("tag", "-a", tag, commit, "-m", message)
	return err
}
func (g Client) PushTag(remote, tag string) error {
	_, err := g.run("push", remote, "refs/tags/"+tag)
	return err
}
func (g Client) CheckRepository() error { _, err := g.run("rev-parse", "--git-dir"); return err }

var ErrIncompleteHistory = errors.New("required history is unavailable (the clone may be shallow); fetch tags and sufficient history explicitly")
