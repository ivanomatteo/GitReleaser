package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunExposesServiceEnvironment(t *testing.T) {
	repo, git, write := monorepo(t)
	write("releaser.yml", "services:\n  api:\n    paths: [services/api]\n    dependencies: [common/auth, common/log]\n    vars:\n      docker-image: registry/api\n  worker:\n    paths: [services/worker]\n")
	write("services/worker/main.go", "package worker\n")
	git("add", ".")
	git("commit", "-m", "worker")
	git("tag", "-a", "worker/v1.0.0", "-m", "Release worker v1.0.0")
	git("tag", "-a", "worker/v1.2.0", "-m", "Release worker v1.2.0")
	script := filepath.Join(t.TempDir(), "show.sh")
	if err := os.WriteFile(script, []byte("#!/bin/sh\necho \"$RELEASER_NAME|$RELEASER_PATHS|$RELEASER_DEPS|$RELEASER_VAR_DOCKER_IMAGE|$RELEASER_VERSION|$RELEASER_TAG|$PWD|$*\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("RELEASER_VAR_DOCKER_IMAGE", "leaked")

	out, err := runCapture(t, "--repo", repo, "run", script, "--", "a", "b")
	if err != nil {
		t.Fatal(err)
	}
	want := "api|services/api|common/auth common/log|registry/api|1.0.0|api/v1.0.0|" + repo + "|a b\n" +
		"worker|services/worker|||1.2.0|worker/v1.2.0|" + repo + "|a b\n"
	if out != want {
		t.Fatalf("unexpected output:\n%s\nwant:\n%s", out, want)
	}

	out, err = runCapture(t, "--repo", repo, "run", "worker", script)
	if err != nil || !strings.HasPrefix(out, "worker|") || strings.Count(out, "\n") != 1 {
		t.Fatalf("single service run: %q, %v", out, err)
	}

	write("services/api/main.go", "package api // changed\n")
	git("commit", "-am", "change api")
	out, err = runCapture(t, "--repo", repo, "run", "--affected", script)
	if err != nil || !strings.HasPrefix(out, "api|") || strings.Count(out, "\n") != 1 {
		t.Fatalf("affected run: %q, %v", out, err)
	}
}

func TestRunStopsAtFirstFailureWithScriptExitCode(t *testing.T) {
	repo, _, write := monorepo(t)
	write("releaser.yml", "services:\n  api:\n    paths: [services/api]\n  worker:\n    paths: [services/worker]\n")
	script := filepath.Join(t.TempDir(), "fail.sh")
	if err := os.WriteFile(script, []byte("#!/bin/sh\necho $RELEASER_NAME\nexit 7\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	out, err := runCapture(t, "--repo", repo, "run", script)
	if err == nil || ExitCode(err) != 7 || out != "api\n" {
		t.Fatalf("got %q, %v (exit %d)", out, err, ExitCode(err))
	}
}

func TestRunValidatesArgumentsAndVariables(t *testing.T) {
	repo, _, write := monorepo(t)
	for _, args := range [][]string{{"run"}, {"run", "a", "b", "c"}, {"run", "missing", "x.sh"}, {"run", "--affected", "api", "x.sh"}} {
		if _, err := runCapture(t, append([]string{"--repo", repo}, args...)...); err == nil {
			t.Fatalf("expected an error for %v", args)
		}
	}
	write("releaser.yml", "services:\n  api:\n    paths: [services/api]\n    vars:\n      my-var: a\n      my_var: b\n")
	var out, errOut bytes.Buffer
	cmd := newApp(&out, &errOut)
	cmd.SetArgs([]string{"--repo", repo, "run", "true"})
	if err := cmd.Execute(); err == nil || ExitCode(err) != 2 || !strings.Contains(err.Error(), "RELEASER_VAR_MY_VAR") {
		t.Fatalf("expected a variable collision error, got %v", err)
	}
	if errOut.Len() != 0 {
		t.Fatalf("nothing should run before validation: %q", errOut.String())
	}
}

func TestRunWithoutReleaseOrRepositoryLeavesVersionEmpty(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "releaser.yml"), []byte("services:\n  api:\n    paths: [services/api]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(t.TempDir(), "show.sh")
	if err := os.WriteFile(script, []byte("#!/bin/sh\necho \"$RELEASER_NAME|$RELEASER_VERSION|$RELEASER_TAG\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("RELEASER_TAG", "leaked")
	out, err := runCapture(t, "--repo", dir, "run", script)
	if err != nil || out != "api||\n" {
		t.Fatalf("got %q, %v", out, err)
	}
}
