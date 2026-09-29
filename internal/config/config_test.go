package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidate(t *testing.T) {
	c := Config{Services: map[string]Service{"api": {Paths: []string{"services/api"}}}}
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
	c.Services["bad/name"] = Service{Paths: []string{"x"}}
	if err := c.Validate(); err == nil {
		t.Fatal("expected invalid name")
	}
}

func TestLoadServiceVars(t *testing.T) {
	path := filepath.Join(t.TempDir(), "releaser.yml")
	contents := "services:\n  api:\n    paths: [services/api]\n    vars:\n      image: registry.example/api\n      channel: stable\n"
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
	c, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := c.Services["api"].Vars["image"]; got != "registry.example/api" {
		t.Fatalf("unexpected variable value %q", got)
	}
}

func TestLoadRejectsNonStringServiceVars(t *testing.T) {
	path := filepath.Join(t.TempDir(), "releaser.yml")
	contents := "services:\n  api:\n    paths: [services/api]\n    vars:\n      replicas: 3\n"
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Load(path)
	if err == nil || !strings.Contains(err.Error(), "must be strings") {
		t.Fatalf("expected a string type error, got %v", err)
	}
}

func TestLoadRejectsUnknownFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "releaser.yml")
	contents := "services:\n  api:\n    paths: [services/api]\n    dependecies: [common]\n"
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil || !strings.Contains(err.Error(), "dependecies") {
		t.Fatalf("expected unknown field error, got %v", err)
	}
}

func TestValidateRejectsNamesInvalidAsGitRefs(t *testing.T) {
	for _, name := range []string{"", "a b", "a~b", "a^b", "a:b", "a..b", ".api", "-api", "api.", "api.lock", "a@{b", "a*", "a?", "a[b", `a\b`, "a/b"} {
		c := Config{Services: map[string]Service{name: {Paths: []string{"x"}}}}
		if err := c.Validate(); err == nil {
			t.Errorf("expected %q to be rejected", name)
		}
	}
	for _, name := range []string{"api", "scraper-service", "api_v2", "web.front"} {
		c := Config{Services: map[string]Service{name: {Paths: []string{"x"}}}}
		if err := c.Validate(); err != nil {
			t.Errorf("expected %q to be accepted: %v", name, err)
		}
	}
}
