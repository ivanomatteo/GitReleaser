package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTemplateRendersServices(t *testing.T) {
	repo, _, write := monorepo(t)
	write("releaser.yml", "services:\n  api:\n    paths: [services/api]\n    dependencies: [common/auth]\n    vars:\n      image: registry/api\n  worker:\n    paths: [services/worker]\n")
	dir := t.TempDir()
	tpl := filepath.Join(dir, "svc.tpl")
	if err := os.WriteFile(tpl, []byte("{{.Name}} {{.Version}} {{.Tag}} {{range .Paths}}{{.}} {{end}}{{range .Deps}}{{.}} {{end}}{{index .Vars \"image\"}}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := runCapture(t, "--repo", repo, "template", tpl)
	if err != nil {
		t.Fatal(err)
	}
	if want := "api 1.0.0 api/v1.0.0 services/api common/auth registry/api\nworker   services/worker \n"; out != want {
		t.Fatalf("got %q, want %q", out, want)
	}

	out, err = runCapture(t, "--repo", repo, "template", "worker", tpl)
	if err != nil || !strings.HasPrefix(out, "worker ") || strings.Count(out, "\n") != 1 {
		t.Fatalf("single service: %q, %v", out, err)
	}

	outPattern := filepath.Join(dir, "out", "{{.Name}}.txt")
	out, err = runCapture(t, "--repo", repo, "template", tpl, "-o", outPattern)
	if err != nil {
		t.Fatal(err)
	}
	apiFile := filepath.Join(dir, "out", "api.txt")
	if want := apiFile + "\n" + filepath.Join(dir, "out", "worker.txt") + "\n"; out != want {
		t.Fatalf("got %q, want %q", out, want)
	}
	if b, err := os.ReadFile(apiFile); err != nil || !strings.HasPrefix(string(b), "api 1.0.0") {
		t.Fatalf("api.txt: %q, %v", b, err)
	}
}

func TestTemplateErrorsWriteNothing(t *testing.T) {
	repo, _, write := monorepo(t)
	write("releaser.yml", "services:\n  api:\n    paths: [services/api]\n    vars:\n      image: registry/api\n  worker:\n    paths: [services/worker]\n")
	dir := t.TempDir()
	tpl := filepath.Join(dir, "svc.tpl")
	if err := os.WriteFile(tpl, []byte("{{.Vars.image}}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// worker has no "image" variable: missing keys are errors.
	if _, err := runCapture(t, "--repo", repo, "template", tpl, "-o", filepath.Join(dir, "{{.Name}}")); err == nil || !strings.Contains(err.Error(), "worker") {
		t.Fatalf("expected a missing key error, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "api")); !os.IsNotExist(err) {
		t.Fatal("no file should be written when a service fails")
	}
	for _, args := range [][]string{
		{"template", "api", tpl, "-o", filepath.Join(dir, "same")},
		{"template", tpl, "-o", filepath.Join(dir, "same")},
		{"template", "missing", tpl},
		{"template", filepath.Join(dir, "nope.tpl")},
	} {
		_, err := runCapture(t, append([]string{"--repo", repo}, args...)...)
		if (err == nil) != (args[1] == "api") {
			t.Fatalf("%v: unexpected result %v", args, err)
		}
	}
}
