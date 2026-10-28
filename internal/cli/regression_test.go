package cli

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// monorepo creates a repository with a single "api" service released as api/v1.0.0.
func monorepo(t *testing.T) (string, func(...string) string, func(string, string)) {
	t.Helper()
	repo, git := rootTestRepo(t)
	write := func(name, contents string) {
		t.Helper()
		p := filepath.Join(repo, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("releaser.yml", "services:\n  api:\n    paths: [services/api]\n")
	write("services/api/main.go", "package api\n")
	git("add", ".")
	git("commit", "-m", "api")
	git("tag", "-a", "api/v1.0.0", "-m", "Release api v1.0.0")
	return repo, git, write
}

// runCapture executes the CLI and returns what it wrote to stdout.
func runCapture(t *testing.T, args ...string) (string, error) {
	t.Helper()
	var out, errOut bytes.Buffer
	cmd := newApp(&out, &errOut)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return out.String(), err
}

func TestChangesReportsUnquotedAndRelevantRenamedPaths(t *testing.T) {
	repo, git, write := monorepo(t)
	write("services/api/caffè.go", "package api\n")
	git("add", ".")
	git("commit", "-m", "unicode")
	if err := os.Mkdir(filepath.Join(repo, "other"), 0o755); err != nil {
		t.Fatal(err)
	}
	git("mv", "services/api/main.go", "other/main.go")
	git("commit", "-m", "move out")

	out, err := runCapture(t, "--repo", repo, "changes", "api")
	if err != nil {
		t.Fatal(err)
	}
	if out != "services/api/caffè.go\nservices/api/main.go\n" {
		t.Fatalf("unexpected changes %q", out)
	}
}

func TestDefaultConfigIsResolvedAgainstRepo(t *testing.T) {
	repo, _, _ := monorepo(t)
	t.Chdir(t.TempDir())
	out, err := runCapture(t, "--repo", repo, "version-number", "api")
	if err != nil {
		t.Fatal(err)
	}
	if out != "1.0.0\n" {
		t.Fatalf("unexpected output %q", out)
	}
	// An explicit relative --config stays relative to the working directory.
	if _, err = runCapture(t, "--repo", repo, "--config", "releaser.yml", "version-number", "api"); ExitCode(err) != 2 {
		t.Fatalf("expected configuration error, got %v", err)
	}
}

func TestShallowCloneReportsIncompleteHistory(t *testing.T) {
	repo, git, write := monorepo(t)
	for _, c := range []string{"a", "b"} {
		write("services/api/main.go", "package api\n// "+c+"\n")
		git("commit", "-am", c)
	}
	clone := filepath.Join(t.TempDir(), "clone")
	for _, args := range [][]string{
		{"clone", "-q", "--depth", "1", "file://" + repo, clone},
		{"-C", clone, "fetch", "-q", "--depth", "1", "origin", "refs/tags/*:refs/tags/*"},
	} {
		if out, err := exec.Command("git", args...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	_, err := runCapture(t, "--repo", clone, "status")
	if err == nil || ExitCode(err) != 3 || !strings.Contains(err.Error(), "history is unavailable") {
		t.Fatalf("expected incomplete history error with exit 3, got %v (exit %d)", err, ExitCode(err))
	}
}

func TestRootPrefixRequiresSeparator(t *testing.T) {
	repo, git := rootTestRepo(t)
	git("tag", "dev1.0.0")
	_, err := runCapture(t, "--repo", repo, "release", "--root", "patch", "--dry-run")
	if err == nil || !strings.Contains(err.Error(), "no released version") {
		t.Fatalf("dev1.0.0 must not be treated as a release, got %v", err)
	}
}

func TestPushFailureRollsBackAllTags(t *testing.T) {
	repo, git, write := monorepo(t)
	write("releaser.yml", "services:\n  api:\n    paths: [services/api]\n  web:\n    paths: [services/web]\n")
	write("services/web/main.go", "package web\n")
	git("add", ".")
	git("commit", "-m", "web")
	git("tag", "-a", "web/v1.0.0", "-m", "Release web v1.0.0")
	write("services/api/main.go", "package api\n// x\n")
	write("services/web/main.go", "package web\n// x\n")
	git("commit", "-am", "change")

	bare := filepath.Join(t.TempDir(), "remote.git")
	if out, err := exec.Command("git", "init", "-q", "--bare", bare).CombinedOutput(); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	// Reject only the web tag: an atomic push must publish neither.
	hook := "#!/bin/sh\nwhile read old new ref; do case $ref in refs/tags/web/*) exit 1;; esac; done\n"
	if err := os.WriteFile(filepath.Join(bare, "hooks", "pre-receive"), []byte(hook), 0o755); err != nil {
		t.Fatal(err)
	}
	git("remote", "add", "origin", bare)

	if _, err := runCapture(t, "--repo", repo, "release", "--affected", "patch", "--push"); err == nil {
		t.Fatal("expected push failure")
	}
	if got := git("tag", "--list", "*/v1.0.1"); got != "" {
		t.Fatalf("local tags were not rolled back: %q", got)
	}
	if out, _ := exec.Command("git", "-C", bare, "tag").CombinedOutput(); len(bytes.TrimSpace(out)) != 0 {
		t.Fatalf("remote received tags: %q", out)
	}
}

func TestReleaseRequiresBumpBeforeAffectedCheck(t *testing.T) {
	repo, _, _ := monorepo(t)
	_, err := runCapture(t, "--repo", repo, "release", "api")
	if err == nil || !strings.Contains(err.Error(), "bump or --version is required") {
		t.Fatalf("expected missing bump error, got %v", err)
	}
}

func TestExplicitVersionMustBeGreaterUnlessForced(t *testing.T) {
	repo, git, write := monorepo(t)
	git("tag", "-a", "api/v2.0.0", "-m", "Release api v2.0.0")
	write("services/api/main.go", "package api\n// x\n")
	git("commit", "-am", "change")

	for _, v := range []string{"1.5.0", "2.0.0"} {
		_, err := runCapture(t, "--repo", repo, "release", "api", "--version", v)
		if err == nil || !strings.Contains(err.Error(), "not greater than the current version 2.0.0") {
			t.Fatalf("expected rejection of %s, got %v", v, err)
		}
	}
	if _, err := runCapture(t, "--repo", repo, "release", "api", "--version", "1.5.0", "--force"); err != nil {
		t.Fatalf("forced lower version failed: %v", err)
	}
	if _, err := runCapture(t, "--repo", repo, "release", "api", "--version", "3.0.0"); err != nil {
		t.Fatalf("greater version failed: %v", err)
	}
	if got := git("tag", "--list", "api/v1.5.0", "api/v3.0.0"); got != "api/v1.5.0\napi/v3.0.0" {
		t.Fatalf("unexpected tags %q", got)
	}

	git("tag", "-a", "v2.0.0", "-m", "root release")
	_, err := runCapture(t, "--repo", repo, "release", "--root", "--version", "1.0.0", "--prefix=")
	if err == nil || !strings.Contains(err.Error(), "not greater") {
		t.Fatalf("expected root rejection, got %v", err)
	}
}

func TestUntrackedFilesDoNotBlockRelease(t *testing.T) {
	repo, git, write := monorepo(t)
	write("services/api/main.go", "package api\n// x\n")
	git("commit", "-am", "change")
	write("build/artifact.bin", "untracked\n")

	if _, err := runCapture(t, "--repo", repo, "release", "api", "patch"); err != nil {
		t.Fatalf("untracked file blocked the release: %v", err)
	}
	write("services/api/main.go", "package api\n// dirty\n")
	if _, err := runCapture(t, "--repo", repo, "release", "api", "minor", "--force"); err == nil || !strings.Contains(err.Error(), "not clean") {
		t.Fatalf("expected dirty tree error, got %v", err)
	}
}

func TestDivergedTagFailsStatusVersionRunAndTemplate(t *testing.T) {
	repo, git, write := monorepo(t)
	write("releaser.yml", "services:\n  api:\n    paths: [services/api]\n  worker:\n    paths: [services/worker]\n")
	write("services/worker/main.go", "package worker\n")
	git("add", ".")
	git("commit", "-m", "worker")
	git("checkout", "-q", "-b", "hotfix")
	write("services/api/main.go", "package api\n// hotfix\n")
	git("commit", "-am", "hotfix")
	git("tag", "-a", "api/v1.0.1", "-m", "Release api v1.0.1")
	git("checkout", "-q", "-")
	write("services/api/main.go", "package api\n// mainline\n")
	git("commit", "-am", "mainline")

	dir := t.TempDir()
	script := filepath.Join(dir, "show.sh")
	if err := os.WriteFile(script, []byte("#!/bin/sh\necho \"$RELEASER_NAME $RELEASER_VERSION\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	tpl := filepath.Join(dir, "svc.tpl")
	if err := os.WriteFile(tpl, []byte("{{.Name}} {{.Version}}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	for _, args := range [][]string{
		{"status", "api"},
		{"version-number", "api"},
		{"version-tag", "api"},
		{"next-version-number", "api", "patch"},
		{"next-version-tag", "api", "patch"},
		{"run", script},
		{"run", "api", script},
		{"template", tpl},
		{"template", "api", tpl},
	} {
		out, err := runCapture(t, append([]string{"--repo", repo}, args...)...)
		if err == nil || ExitCode(err) != 1 || !strings.Contains(err.Error(), "latest release api/v1.0.1 is not an ancestor of HEAD") {
			t.Errorf("%v: expected diverged tag error with exit 1, got %v (exit %d)", args, err, ExitCode(err))
		}
		if out != "" {
			t.Errorf("%v: unexpected stdout %q", args, out)
		}
	}

	// A service whose own release is fine is not blocked by another service's diverged tag.
	if out, err := runCapture(t, "--repo", repo, "run", "worker", script); err != nil || out != "worker \n" {
		t.Errorf("run worker: got %q, %v", out, err)
	}
	if out, err := runCapture(t, "--repo", repo, "template", "worker", tpl); err != nil || out != "worker \n" {
		t.Errorf("template worker: got %q, %v", out, err)
	}

	git("checkout", "-q", "hotfix")
	if out, err := runCapture(t, "--repo", repo, "next-version-tag", "api", "patch"); err != nil || out != "api/v1.0.2\n" {
		t.Fatalf("from the hotfix branch: got %q, %v", out, err)
	}
}
