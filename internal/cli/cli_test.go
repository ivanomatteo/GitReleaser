package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

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
