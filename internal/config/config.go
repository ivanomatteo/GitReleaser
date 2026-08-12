package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type Service struct {
	Paths        []string `yaml:"paths"`
	Dependencies []string `yaml:"dependencies"`
	Ignore       []string `yaml:"ignore"`
}

type Config struct {
	Services map[string]Service `yaml:"services"`
	Ignore   []string           `yaml:"ignore"`
	Remote   string             `yaml:"remote"`
}

func Load(path string) (Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config %s: %w", path, err)
	}
	var c Config
	if err := yaml.Unmarshal(b, &c); err != nil {
		return Config{}, fmt.Errorf("parse config %s: %w", path, err)
	}
	if c.Remote == "" {
		c.Remote = "origin"
	}
	if err := c.Validate(); err != nil {
		return Config{}, err
	}
	return c, nil
}

func (c Config) Validate() error {
	if len(c.Services) == 0 {
		return errors.New("configuration contains no services")
	}
	for name, svc := range c.Services {
		if strings.TrimSpace(name) == "" || strings.Contains(name, "/") {
			return fmt.Errorf("invalid service name %q", name)
		}
		if len(svc.Paths) == 0 {
			return fmt.Errorf("service %s has no paths", name)
		}
		for _, p := range append(append([]string{}, svc.Paths...), svc.Dependencies...) {
			if err := validPath(p); err != nil {
				return fmt.Errorf("service %s: %w", name, err)
			}
		}
	}
	return nil
}

func validPath(p string) error {
	p = filepath.ToSlash(filepath.Clean(p))
	if p == "." || p == "" || filepath.IsAbs(p) || p == ".." || strings.HasPrefix(p, "../") {
		return fmt.Errorf("invalid repository path %q", p)
	}
	return nil
}
