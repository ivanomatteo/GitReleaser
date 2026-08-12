package config

import "testing"

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
