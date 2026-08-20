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
