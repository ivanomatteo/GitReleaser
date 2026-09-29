package cli

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"unicode"

	"github.com/ivano/gitreleaser/internal/config"
	gitclient "github.com/ivano/gitreleaser/internal/git"
	"github.com/ivano/gitreleaser/internal/service"
	"github.com/spf13/cobra"
)

func (a *app) runCommand() *cobra.Command {
	var affected bool
	c := &cobra.Command{Use: "run [service] <script> [-- args...]", RunE: func(cmd *cobra.Command, args []string) error {
		var scriptArgs []string
		if dash := cmd.ArgsLenAtDash(); dash >= 0 {
			args, scriptArgs = args[:dash], args[dash:]
		}
		if len(args) < 1 || len(args) > 2 {
			return codedError{1, errors.New("run requires a script, optionally preceded by a service")}
		}
		if affected && len(args) == 2 {
			return codedError{1, errors.New("--affected cannot be used with a service")}
		}
		script := args[len(args)-1]

		sel, err := a.selectServices(args[:len(args)-1], affected)
		if err != nil {
			return err
		}

		// The script runs from the repository root, so a script found relative to the
		// working directory is made absolute; anything else is looked up in PATH.
		if _, err := os.Stat(script); err == nil {
			if script, err = filepath.Abs(script); err != nil {
				return err
			}
		}

		// Validate every service before running anything.
		envs := make([][]string, len(sel))
		for i, t := range sel {
			env, err := serviceEnv(t.name, t.service, t.release)
			if err != nil {
				return codedError{2, err}
			}
			envs[i] = env
		}
		base := inheritedEnv()
		for i, t := range sel {
			fmt.Fprintf(a.err, "==> %s\n", t.name)
			x := exec.Command(script, scriptArgs...)
			x.Dir = a.repo
			x.Env = append(slices.Clone(base), envs[i]...)
			x.Stdin = cmd.InOrStdin()
			x.Stdout = a.out
			x.Stderr = a.err
			if err := x.Run(); err != nil {
				code := 1
				if exitErr, ok := errors.AsType[*exec.ExitError](err); ok && exitErr.ExitCode() > 0 {
					code = exitErr.ExitCode()
				}
				return codedError{code, fmt.Errorf("%s failed for service %s: %w", args[len(args)-1], t.name, err)}
			}
		}
		return nil
	}}
	c.Flags().BoolVar(&affected, "affected", false, "run only for affected services")
	return c
}

// target is a service selected by run or template, with its latest release (nil when
// the service was never released or the repository is not a Git repository).
type target struct {
	name    string
	service config.Service
	release *service.Release
}

// selectServices returns the named service, or all services in alphabetical order,
// or with affected only the affected ones (which requires a Git repository).
func (a *app) selectServices(named []string, affected bool) ([]target, error) {
	var cfg config.Config
	var names []string
	if affected {
		e, err := a.engine(false)
		if err != nil {
			return nil, err
		}
		cfg = e.Config
		ss, err := getStatuses(e, e.Names())
		if err != nil {
			return nil, classify(err)
		}
		for _, s := range ss {
			if s.Affected {
				names = append(names, s.Name)
			}
		}
	} else {
		var err error
		if cfg, err = a.loadConfig(); err != nil {
			return nil, err
		}
		if len(named) == 1 {
			if _, ok := cfg.Services[named[0]]; !ok {
				return nil, fmt.Errorf("unknown service %q", named[0])
			}
			names = named
		} else {
			for name := range cfg.Services {
				names = append(names, name)
			}
			slices.Sort(names)
		}
	}

	targets := make([]target, len(names))
	for i, name := range names {
		targets[i] = target{name: name, service: cfg.Services[name]}
	}
	// The latest release is exposed when available: outside a Git repository, or for a
	// service never released, it is simply missing.
	if g := (gitclient.Client{Dir: a.repo}); g.CheckRepository() == nil {
		e := service.Engine{Config: cfg, Git: g}
		for i := range targets {
			r, err := e.Latest(targets[i].name)
			if err != nil {
				return nil, classify(err)
			}
			targets[i].release = r
		}
	}
	return targets, nil
}

var envNamePattern = regexp.MustCompile(`^[A-Z0-9_]+$`)

// serviceEnv returns the RELEASER_* variables exposed to a script run for a service.
// Lists are space separated, so paths containing whitespace are rejected.
func serviceEnv(name string, svc config.Service, release *service.Release) ([]string, error) {
	for _, p := range append(slices.Clone(svc.Paths), svc.Dependencies...) {
		if strings.ContainsFunc(p, unicode.IsSpace) {
			return nil, fmt.Errorf("service %s: path %q contains whitespace and cannot be exposed to a script", name, p)
		}
	}
	env := []string{
		"RELEASER_NAME=" + name,
		"RELEASER_PATHS=" + strings.Join(svc.Paths, " "),
		"RELEASER_DEPS=" + strings.Join(svc.Dependencies, " "),
	}
	version, tag := "", ""
	if release != nil {
		version, tag = release.Version.String(), release.Tag
	}
	env = append(env, "RELEASER_VERSION="+version, "RELEASER_TAG="+tag)
	keys := make([]string, 0, len(svc.Vars))
	for k := range svc.Vars {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	seen := map[string]string{}
	for _, k := range keys {
		n := strings.ToUpper(strings.ReplaceAll(k, "-", "_"))
		if !envNamePattern.MatchString(n) {
			return nil, fmt.Errorf("service %s: variable %q cannot be converted to an environment variable name", name, k)
		}
		if other, ok := seen[n]; ok {
			return nil, fmt.Errorf("service %s: variables %q and %q both map to RELEASER_VAR_%s", name, other, k, n)
		}
		seen[n] = k
		env = append(env, "RELEASER_VAR_"+n+"="+svc.Vars[k])
	}
	return env, nil
}

// inheritedEnv returns the current environment without the variables set by run,
// so that values from an outer releaser run do not leak into the script.
func inheritedEnv() []string {
	return slices.DeleteFunc(os.Environ(), func(kv string) bool {
		k, _, _ := strings.Cut(kv, "=")
		switch k {
		case "RELEASER_NAME", "RELEASER_PATHS", "RELEASER_DEPS", "RELEASER_VERSION", "RELEASER_TAG":
			return true
		}
		return strings.HasPrefix(k, "RELEASER_VAR_")
	})
}
