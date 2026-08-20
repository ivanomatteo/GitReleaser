package cli

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestGetVar(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "releaser.yml")
	contents := "services:\n  api:\n    paths: [services/api]\n    vars:\n      image: registry.example/api:latest\n      empty: \"\"\n"
	if err := os.WriteFile(configPath, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	a := &app{configPath: configPath, out: &out}
	cmd := a.getVarCommand()
	cmd.SetArgs([]string{"api", "image"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if got := out.String(); got != "registry.example/api:latest\n" {
		t.Fatalf("unexpected output %q", got)
	}

	for _, args := range [][]string{{"missing", "image"}, {"api", "missing"}} {
		cmd = a.getVarCommand()
		cmd.SetArgs(args)
		if err := cmd.Execute(); err == nil {
			t.Fatalf("expected an error for %v", args)
		}
	}
}

func TestReleaseNewCreatesOnlyMissingInitialTags(t *testing.T) {
	repo := t.TempDir()
	configPath := filepath.Join(repo, "releaser.yml")
	write := func(name, contents string) {
		t.Helper()
		path := filepath.Join(repo, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	git := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
		return strings.TrimSpace(string(out))
	}

	git("init")
	git("config", "user.name", "Test")
	git("config", "user.email", "test@example.com")
	write("releaser.yml", "services:\n  api:\n    paths: [services/api]\n  web:\n    paths: [services/web]\n  worker:\n    paths: [services/worker]\n")
	write("services/api/main.go", "package api\n")
	write("services/web/main.go", "package web\n")
	write("services/worker/main.go", "package worker\n")
	git("add", ".")
	git("commit", "-m", "initial")
	git("tag", "-a", "api/v1.2.3", "-m", "Release api v1.2.3")

	cmd := New()
	cmd.SetArgs([]string{"--config", configPath, "--repo", repo, "release", "--new"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("default initial release failed: %v", err)
	}
	if got := git("tag", "--list", "*/v*", "--sort=refname"); got != "api/v1.2.3\nweb/v0.1.0\nworker/v0.1.0" {
		t.Fatalf("unexpected tags:\n%s", got)
	}

	// The command is idempotent when every configured service has a release.
	cmd = New()
	cmd.SetArgs([]string{"--config", configPath, "--repo", repo, "release", "--new", "--version", "2.0.0"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("idempotent initial release failed: %v", err)
	}
	if got := git("tag", "--list", "*/v2.0.0"); got != "" {
		t.Fatalf("existing services were retagged: %s", got)
	}
}

func TestReleaseNewValidatesArgumentsAndVersion(t *testing.T) {
	repo, _ := rootTestRepo(t)
	configPath := filepath.Join(repo, "releaser.yml")
	if err := os.WriteFile(configPath, []byte("services:\n  api:\n    paths: [main.go]\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	for _, args := range [][]string{
		{"release", "--new", "api"},
		{"release", "--new", "--force"},
		{"release", "--new", "--version", "v1.0.0"},
	} {
		cmd := New()
		cmd.SetArgs(append([]string{"--config", configPath, "--repo", repo}, args...))
		if err := cmd.Execute(); err == nil {
			t.Fatalf("expected error for %v", args)
		}
	}
}

func TestBulkRelease(t *testing.T) {
	repo := t.TempDir()
	configPath := filepath.Join(repo, "releaser.yml")
	write := func(name, contents string) {
		t.Helper()
		path := filepath.Join(repo, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	git := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
		return strings.TrimSpace(string(out))
	}
	git("init")
	git("config", "user.name", "Test")
	git("config", "user.email", "test@example.com")
	write("releaser.yml", "services:\n  api:\n    paths: [services/api]\n  worker:\n    paths: [services/worker]\n")
	write("services/api/main.go", "package api\n")
	write("services/worker/main.go", "package worker\n")
	git("add", ".")
	git("commit", "-m", "initial")
	git("tag", "-a", "api/v1.0.0", "-m", "Release api v1.0.0")
	git("tag", "-a", "worker/v2.0.0", "-m", "Release worker v2.0.0")
	write("services/api/main.go", "package api\n// changed\n")
	git("add", ".")
	git("commit", "-m", "change api")

	cmd := New()
	cmd.SetArgs([]string{"--config", configPath, "--repo", repo, "release", "--affected", "minor"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("affected release failed: %v", err)
	}
	if got := git("tag", "--list", "*/v*"); !strings.Contains(got, "api/v1.1.0") || strings.Contains(got, "worker/v2.0.1") {
		t.Fatalf("unexpected tags after affected release: %s", got)
	}

	cmd = New()
	cmd.SetArgs([]string{"--config", configPath, "--repo", repo, "release", "--all", "patch"})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "--all requires --force") {
		t.Fatalf("expected --force error, got %v", err)
	}
	cmd = New()
	cmd.SetArgs([]string{"--config", configPath, "--repo", repo, "release", "--all", "--force", "patch"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("all release failed: %v", err)
	}
	if got := git("tag", "--list", "worker/v2.0.1"); got != "worker/v2.0.1" {
		t.Fatalf("worker tag not created: %q", got)
	}
}

func TestReleaseRequiresAffectedUnlessForced(t *testing.T) {
	repo := t.TempDir()
	configPath := filepath.Join(repo, "releaser.yml")
	write := func(name, contents string) {
		t.Helper()
		path := filepath.Join(repo, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	git := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
		return strings.TrimSpace(string(out))
	}

	git("init")
	git("config", "user.name", "Test")
	git("config", "user.email", "test@example.com")
	write("releaser.yml", "services:\n  api:\n    paths:\n      - services/api\n")
	write("services/api/main.go", "package api\n")
	git("add", ".")
	git("commit", "-m", "initial")
	git("tag", "-a", "api/v1.0.0", "-m", "Release api v1.0.0")
	write("unrelated.txt", "change\n")
	git("add", ".")
	git("commit", "-m", "unrelated change")

	cmd := New()
	cmd.SetArgs([]string{"--config", configPath, "--repo", repo, "release", "api", "patch"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "service api is not affected") {
		t.Fatalf("expected not affected error, got %v", err)
	}
	if tags := git("tag", "--list", "api/v1.0.1"); tags != "" {
		t.Fatalf("unexpected tag created: %s", tags)
	}

	cmd = New()
	cmd.SetArgs([]string{"--config", configPath, "--repo", repo, "release", "api", "patch", "--force"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("forced release failed: %v", err)
	}
	if tag := git("tag", "--list", "api/v1.0.1"); tag != "api/v1.0.1" {
		t.Fatalf("forced release did not create tag, got %q", tag)
	}
}

func TestRootReleaseWithoutConfiguration(t *testing.T) {
	repo, git := rootTestRepo(t)
	git("tag", "-a", "v0.1.5", "-m", "initial release")
	os.WriteFile(filepath.Join(repo, "main.go"), []byte("package main\n// change\n"), 0o644)
	git("add", ".")
	git("commit", "-m", "change")

	cmd := New()
	cmd.SetArgs([]string{"--repo", repo, "release", "--root", "patch"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("root release failed: %v", err)
	}
	if got := git("tag", "--list", "v0.1.6"); got != "v0.1.6" {
		t.Fatalf("unexpected tag %q", got)
	}

	cmd = New()
	cmd.SetArgs([]string{"--repo", repo, "release", "--root", "minor"})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "--force") {
		t.Fatalf("expected force error, got %v", err)
	}
}

func TestRootReleaseBootstrapAndPrefixRules(t *testing.T) {
	repo, git := rootTestRepo(t)
	cmd := New()
	cmd.SetArgs([]string{"--repo", repo, "release", "--root", "--version", "1.0.0"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("bootstrap failed: %v", err)
	}
	if got := git("tag", "--list", "v1.0.0"); got != "v1.0.0" {
		t.Fatalf("unexpected bootstrap tag %q", got)
	}

	git("tag", "-a", "release-v2.0.0", "-m", "other prefix")
	os.WriteFile(filepath.Join(repo, "main.go"), []byte("package main\n// next\n"), 0o644)
	git("add", ".")
	git("commit", "-m", "next")
	cmd = New()
	cmd.SetArgs([]string{"--repo", repo, "release", "--root", "patch"})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "heterogeneous prefixes") {
		t.Fatalf("expected prefix error, got %v", err)
	}
	cmd = New()
	cmd.SetArgs([]string{"--repo", repo, "release", "--root", "--prefix=", "patch"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("explicit empty prefix failed: %v", err)
	}
	if got := git("tag", "--list", "v1.0.1"); got != "v1.0.1" {
		t.Fatalf("unexpected explicit-prefix tag %q", got)
	}
}

func TestRootReleaseRejectsBulkModes(t *testing.T) {
	repo, _ := rootTestRepo(t)
	for _, flag := range []string{"--affected", "--all"} {
		cmd := New()
		cmd.SetArgs([]string{"--repo", repo, "release", "--root", flag, "patch"})
		if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "cannot be used") {
			t.Fatalf("expected incompatibility for %s, got %v", flag, err)
		}
	}
}

func rootTestRepo(t *testing.T) (string, func(...string) string) {
	t.Helper()
	repo := t.TempDir()
	git := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
		return strings.TrimSpace(string(out))
	}
	git("init")
	git("config", "user.name", "Test")
	git("config", "user.email", "test@example.com")
	if err := os.WriteFile(filepath.Join(repo, "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git("add", ".")
	git("commit", "-m", "initial")
	return repo, git
}
