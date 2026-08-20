package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/ivano/gitreleaser/internal/config"
	gitclient "github.com/ivano/gitreleaser/internal/git"
	"github.com/ivano/gitreleaser/internal/service"
	"github.com/ivano/gitreleaser/internal/version"
	"github.com/spf13/cobra"
)

type codedError struct {
	code int
	err  error
}

func (e codedError) Error() string { return e.err.Error() }
func (e codedError) Unwrap() error { return e.err }
func ExitCode(err error) int {
	var e codedError
	if errors.As(err, &e) {
		return e.code
	}
	if errors.Is(err, gitclient.ErrIncompleteHistory) {
		return 3
	}
	return 1
}

type app struct {
	configPath, repo string
	out, err         io.Writer
	root             *cobra.Command
}

func New() *cobra.Command {
	a := &app{out: os.Stdout, err: os.Stderr, repo: "."}
	r := &cobra.Command{Use: "releaser", SilenceUsage: true, SilenceErrors: true, Short: "Release independent services in a monorepo"}
	r.CompletionOptions.DisableDefaultCmd = true
	r.SetOut(a.out)
	r.SetErr(a.err)
	a.root = r
	r.PersistentFlags().StringVarP(&a.configPath, "config", "c", "releaser.yml", "configuration file")
	r.PersistentFlags().StringVar(&a.repo, "repo", ".", "Git repository directory")
	r.AddCommand(a.versionCommand("version-number", false, false), a.versionCommand("version-tag", true, false), a.versionCommand("next-version-number", false, true), a.versionCommand("next-version-tag", true, true))
	r.AddCommand(a.statusCommand(), a.affectedCommand(), a.changesCommand(), a.getVarCommand(), a.planCommand(), a.releaseCommand(), a.configCommand())
	return r
}

func (a *app) getVarCommand() *cobra.Command {
	return &cobra.Command{Use: "get-var <service> <key>", Args: cobra.ExactArgs(2), RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load(a.configPath)
		if err != nil {
			return codedError{2, err}
		}
		svc, ok := cfg.Services[args[0]]
		if !ok {
			return fmt.Errorf("unknown service %q", args[0])
		}
		value, ok := svc.Vars[args[1]]
		if !ok {
			return fmt.Errorf("unknown variable %q for service %q", args[1], args[0])
		}
		fmt.Fprintln(a.out, value)
		return nil
	}}
}

func (a *app) engine(verbose bool) (service.Engine, error) {
	c, err := config.Load(a.configPath)
	if err != nil {
		return service.Engine{}, codedError{2, err}
	}
	g := gitclient.Client{Dir: a.repo}
	if err := g.CheckRepository(); err != nil {
		return service.Engine{}, codedError{3, err}
	}
	return service.Engine{Config: c, Git: g, Verbose: verbose, Warn: func(s string) { fmt.Fprintln(a.err, "WARNING:", s) }}, nil
}
func unreleased(name string) error {
	return codedError{1, fmt.Errorf("%s has no released version", name)}
}

func (a *app) versionCommand(use string, tag, next bool) *cobra.Command {
	args := cobra.ExactArgs(1)
	usage := use + " <service>"
	if next {
		args = cobra.ExactArgs(2)
		usage += " <patch|minor|major>"
	}
	return &cobra.Command{Use: usage, Args: args, RunE: func(cmd *cobra.Command, args []string) error {
		e, err := a.engine(false)
		if err != nil {
			return err
		}
		r, err := e.Latest(args[0])
		if err != nil {
			return classify(err)
		}
		if r == nil {
			return unreleased(args[0])
		}
		v := r.Version
		if next {
			v, err = v.Bump(args[1])
			if err != nil {
				return codedError{1, err}
			}
		}
		if tag {
			fmt.Fprintf(a.out, "%s/v%s\n", args[0], v.String())
		} else {
			fmt.Fprintln(a.out, v.String())
		}
		return nil
	}}
}

func (a *app) statusCommand() *cobra.Command {
	var verbose bool
	c := &cobra.Command{Use: "status [service]", Args: cobra.MaximumNArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		e, err := a.engine(verbose)
		if err != nil {
			return err
		}
		names := e.Names()
		if len(args) == 1 {
			if _, err = e.RequireService(args[0]); err != nil {
				return err
			}
			names = []string{args[0]}
		}
		statuses, err := getStatuses(e, names)
		if err != nil {
			return classify(err)
		}
		if verbose {
			for i, s := range statuses {
				if i > 0 {
					fmt.Fprintln(a.out)
				}
				printVerbose(a.out, s)
			}
			return nil
		}
		w := tabwriter.NewWriter(a.out, 0, 4, 3, ' ', 0)
		fmt.Fprintln(w, "SERVICE\tLAST VERSION\tAFFECTED")
		for _, s := range statuses {
			v := "none"
			if s.Release != nil {
				v = s.Release.Version.String()
			}
			fmt.Fprintf(w, "%s\t%s\t%s\n", s.Name, v, yesno(s.Affected))
		}
		return w.Flush()
	}}
	c.Flags().BoolVarP(&verbose, "verbose", "v", false, "show detailed status")
	return c
}
func printVerbose(w io.Writer, s service.Status) {
	v, t := "none", "none"
	if s.Release != nil {
		v = s.Release.Version.String()
		t = s.Release.Tag
	}
	fmt.Fprintf(w, "Service: %s\n\nLast version:\n  %s\n\nLast tag:\n  %s\n\nChanged files:\n", s.Name, v, t)
	for _, f := range s.Files {
		fmt.Fprintf(w, "  %s\n", f)
	}
	fmt.Fprintf(w, "\nAffected:\n  %s\n", yesno(s.Affected))
}
func getStatuses(e service.Engine, names []string) ([]service.Status, error) {
	r := make([]service.Status, 0, len(names))
	for _, n := range names {
		s, err := e.Status(n)
		if err != nil {
			return nil, err
		}
		r = append(r, s)
	}
	return r, nil
}
func yesno(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

func (a *app) affectedCommand() *cobra.Command {
	return &cobra.Command{Use: "affected", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
		e, err := a.engine(false)
		if err != nil {
			return err
		}
		ss, err := getStatuses(e, e.Names())
		if err != nil {
			return classify(err)
		}
		for _, s := range ss {
			if s.Affected {
				fmt.Fprintln(a.out, s.Name)
			}
		}
		return nil
	}}
}
func (a *app) changesCommand() *cobra.Command {
	return &cobra.Command{Use: "changes <service>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		e, err := a.engine(false)
		if err != nil {
			return err
		}
		s, err := e.Status(args[0])
		if err != nil {
			return classify(err)
		}
		for _, f := range s.Files {
			fmt.Fprintln(a.out, f)
		}
		return nil
	}}
}

type planService struct {
	Name         string  `json:"name"`
	LastVersion  *string `json:"lastVersion"`
	LastTag      *string `json:"lastTag"`
	Affected     bool    `json:"affected"`
	ChangedFiles int     `json:"changedFiles"`
}

func (a *app) planCommand() *cobra.Command {
	var format string
	c := &cobra.Command{Use: "plan", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
		e, err := a.engine(false)
		if err != nil {
			return err
		}
		ss, err := getStatuses(e, e.Names())
		if err != nil {
			return classify(err)
		}
		if format == "json" {
			p := struct {
				Services []planService `json:"services"`
			}{Services: make([]planService, 0, len(ss))}
			for _, s := range ss {
				x := planService{Name: s.Name, Affected: s.Affected, ChangedFiles: len(s.Files)}
				if s.Release != nil {
					v, t := s.Release.Version.String(), s.Release.Tag
					x.LastVersion = &v
					x.LastTag = &t
				}
				p.Services = append(p.Services, x)
			}
			b, _ := json.MarshalIndent(p, "", "  ")
			fmt.Fprintln(a.out, string(b))
			return nil
		}
		if format != "table" && format != "" {
			return codedError{1, fmt.Errorf("unsupported format %q", format)}
		}
		w := tabwriter.NewWriter(a.out, 0, 4, 3, ' ', 0)
		fmt.Fprintln(w, "SERVICE\tVERSION\tAFFECTED\tFILES")
		for _, s := range ss {
			v := "none"
			if s.Release != nil {
				v = s.Release.Version.String()
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%d\n", s.Name, v, yesno(s.Affected), len(s.Files))
		}
		return w.Flush()
	}}
	c.Flags().StringVar(&format, "format", "table", "table or json")
	return c
}

func (a *app) releaseCommand() *cobra.Command {
	var explicit string
	var prefix string
	var dry, push, force, affected, all, root, newRelease bool
	c := &cobra.Command{Use: "release <service> [patch|minor|major] | release (--affected|--all|--root) <patch|minor|major> | release --new [--version <semver>]", Args: cobra.MaximumNArgs(2), RunE: func(cmd *cobra.Command, args []string) error {
		if root {
			if newRelease {
				return codedError{1, errors.New("--new cannot be used with --root")}
			}
			return a.releaseRoot(cmd, args, explicit, prefix, dry, push, force, affected, all)
		}
		if cmd.Flags().Changed("prefix") {
			return codedError{1, errors.New("--prefix can only be used with --root")}
		}
		e, err := a.engine(false)
		if err != nil {
			return err
		}
		if newRelease && (affected || all || force || len(args) != 0) {
			return codedError{1, errors.New("--new cannot be used with service, bump, --affected, --all, or --force")}
		}
		if affected && all {
			return codedError{1, errors.New("use either --affected or --all, not both")}
		}
		bulk := affected || all || newRelease
		if all && !force {
			return codedError{1, errors.New("--all requires --force")}
		}
		if !newRelease && bulk && (len(args) != 1 || explicit != "") {
			return codedError{1, errors.New("bulk release requires exactly one patch, minor, or major bump")}
		}
		if !bulk && explicit != "" && len(args) == 2 {
			return codedError{1, errors.New("use either a bump or --version, not both")}
		}

		names := []string{}
		bump := ""
		if newRelease {
			if explicit == "" {
				explicit = "0.1.0"
			}
			if _, err = version.Parse(explicit); err != nil {
				return codedError{1, err}
			}
			for _, name := range e.Names() {
				current, latestErr := e.Latest(name)
				if latestErr != nil {
					return classify(latestErr)
				}
				if current == nil {
					names = append(names, name)
				}
			}
		} else if bulk {
			bump = args[0]
			if bump != "patch" && bump != "minor" && bump != "major" {
				return codedError{1, fmt.Errorf("invalid bump %q: expected patch, minor, or major", bump)}
			}
			for _, name := range e.Names() {
				status, statusErr := e.Status(name)
				if statusErr != nil {
					return classify(statusErr)
				}
				if all || status.Affected {
					names = append(names, name)
				}
			}
		} else {
			if _, err = e.RequireService(args[0]); err != nil {
				return err
			}
			names = append(names, args[0])
			if len(args) == 2 {
				bump = args[1]
			}
		}

		type releaseItem struct {
			name, current, tag string
			next               version.Version
		}
		items := make([]releaseItem, 0, len(names))
		for _, name := range names {
			current, latestErr := e.Latest(name)
			if latestErr != nil {
				return classify(latestErr)
			}
			status, statusErr := e.Status(name)
			if statusErr != nil {
				return classify(statusErr)
			}
			if !status.Affected && !force {
				return codedError{1, fmt.Errorf("service %s is not affected; use --force to create a release anyway", name)}
			}
			var next version.Version
			if explicit != "" {
				next, err = version.Parse(explicit)
			} else {
				if bump == "" {
					return codedError{1, errors.New("a bump or --version is required")}
				}
				if current == nil {
					return unreleased(name)
				}
				next, err = current.Version.Bump(bump)
			}
			if err != nil {
				return codedError{1, err}
			}
			tag := name + "/v" + next.String()
			tags, tagsErr := e.Git.Tags(tag)
			if tagsErr != nil {
				return classify(tagsErr)
			}
			if len(tags) > 0 {
				return codedError{1, fmt.Errorf("tag %s already exists", tag)}
			}
			cur := "none"
			if current != nil {
				cur = current.Version.String()
			}
			items = append(items, releaseItem{name: name, current: cur, next: next, tag: tag})
		}
		if dry {
			for i, item := range items {
				if i > 0 {
					fmt.Fprintln(a.out)
				}
				fmt.Fprintf(a.out, "Service: %s\nCurrent version: %s\nNext version: %s\nTag: %s\n", item.name, item.current, item.next.String(), item.tag)
			}
			fmt.Fprintln(a.out, "\nNo changes have been made.")
			return nil
		}
		clean, err := e.Git.IsClean()
		if err != nil {
			return classify(err)
		}
		if !clean {
			return codedError{1, errors.New("working tree is not clean")}
		}
		for _, item := range items {
			if err = e.Git.CreateTag(item.tag, "HEAD", fmt.Sprintf("Release %s v%s", item.name, item.next.String())); err != nil {
				return classify(err)
			}
			if push {
				if err = e.Git.PushTag(e.Config.Remote, item.tag); err != nil {
					return classify(err)
				}
			}
			fmt.Fprintln(a.out, item.tag)
		}
		return nil
	}}
	c.Flags().StringVar(&explicit, "version", "", "explicit semantic version")
	c.Flags().BoolVar(&dry, "dry-run", false, "show without creating the tag")
	c.Flags().BoolVar(&push, "push", false, "push the tag to the configured remote")
	c.Flags().BoolVar(&force, "force", false, "release even if the service is not affected")
	c.Flags().BoolVar(&affected, "affected", false, "release all affected services")
	c.Flags().BoolVar(&all, "all", false, "release all services (requires --force)")
	c.Flags().BoolVar(&root, "root", false, "release the repository as a single project (no configuration required)")
	c.Flags().BoolVar(&newRelease, "new", false, "create the first tag for every unreleased service")
	c.Flags().StringVar(&prefix, "prefix", "", "tag prefix for a root release (empty means tags such as v1.2.3)")
	return c
}

var rootTagPattern = regexp.MustCompile(`^(.*)v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(?:[-+].*)?$`)

func (a *app) releaseRoot(cmd *cobra.Command, args []string, explicit, prefix string, dry, push, force, affected, all bool) error {
	if affected || all {
		return codedError{1, errors.New("--root cannot be used with --affected or --all")}
	}
	if len(args) > 1 || (explicit == "" && len(args) != 1) || (explicit != "" && len(args) != 0) {
		return codedError{1, errors.New("root release requires one bump or --version")}
	}
	g := gitclient.Client{Dir: a.repo}
	if err := g.CheckRepository(); err != nil {
		return codedError{3, err}
	}
	tags, err := g.Tags("*")
	if err != nil {
		return classify(err)
	}
	prefixSet := map[string]bool{}
	for _, tag := range tags {
		if m := rootTagPattern.FindStringSubmatch(tag); m != nil {
			if _, parseErr := version.Parse(strings.TrimPrefix(tag, m[1]+"v")); parseErr == nil {
				prefixSet[m[1]] = true
			}
		}
	}
	prefixSpecified := cmd.Flags().Changed("prefix")
	if !prefixSpecified {
		if len(prefixSet) > 1 {
			return codedError{1, errors.New("release tags use heterogeneous prefixes; --prefix is required")}
		}
		for p := range prefixSet {
			prefix = p
		}
	}
	var current *service.Release
	for _, tag := range tags {
		want := prefix + "v"
		if !strings.HasPrefix(tag, want) {
			continue
		}
		v, parseErr := version.Parse(strings.TrimPrefix(tag, want))
		if parseErr != nil {
			continue
		}
		if current == nil || version.Compare(v, current.Version) > 0 {
			current = &service.Release{Version: v, Tag: tag}
		}
	}
	var next version.Version
	if explicit != "" {
		next, err = version.Parse(explicit)
	} else if current == nil {
		return codedError{1, errors.New("repository has no released version; use --version for the initial release")}
	} else {
		next, err = current.Version.Bump(args[0])
	}
	if err != nil {
		return codedError{1, err}
	}
	if current != nil {
		if _, err = g.Resolve(current.Tag); err != nil {
			return codedError{3, gitclient.ErrIncompleteHistory}
		}
		ancestor, ancestorErr := g.IsAncestor(current.Tag, "HEAD")
		if ancestorErr != nil {
			return classify(ancestorErr)
		}
		if !ancestor {
			return codedError{1, fmt.Errorf("release tag %s is not an ancestor of HEAD", current.Tag)}
		}
		files, diffErr := g.DiffFiles(current.Tag, "HEAD")
		if diffErr != nil {
			return classify(diffErr)
		}
		if len(files) == 0 && !force {
			return codedError{1, errors.New("repository has no changes since the previous tag; use --force to create a release anyway")}
		}
	}
	tag := prefix + "v" + next.String()
	if existing, tagsErr := g.Tags(tag); tagsErr != nil {
		return classify(tagsErr)
	} else if len(existing) > 0 {
		return codedError{1, fmt.Errorf("tag %s already exists", tag)}
	}
	cur := "none"
	if current != nil {
		cur = current.Version.String()
	}
	if dry {
		fmt.Fprintf(a.out, "Repository root\nCurrent version: %s\nNext version: %s\nTag: %s\n\nNo changes have been made.\n", cur, next.String(), tag)
		return nil
	}
	clean, err := g.IsClean()
	if err != nil {
		return classify(err)
	}
	if !clean {
		return codedError{1, errors.New("working tree is not clean")}
	}
	if err = g.CreateTag(tag, "HEAD", "Release v"+next.String()); err != nil {
		return classify(err)
	}
	if push {
		if err = g.PushTag("origin", tag); err != nil {
			return classify(err)
		}
	}
	fmt.Fprintln(a.out, tag)
	return nil
}

func (a *app) configCommand() *cobra.Command {
	c := &cobra.Command{Use: "config"}
	c.AddCommand(&cobra.Command{Use: "check", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load(a.configPath)
		if err != nil {
			return codedError{2, err}
		}
		names := make([]string, 0, len(cfg.Services))
		for n := range cfg.Services {
			names = append(names, n)
		}
		sort.Strings(names)
		fmt.Fprintf(a.out, "configuration valid: %d services\n", len(names))
		return nil
	}})
	return c
}
func classify(err error) error {
	if errors.Is(err, gitclient.ErrIncompleteHistory) {
		return codedError{3, err}
	}
	if strings.HasPrefix(err.Error(), "git ") {
		return codedError{3, err}
	}
	return err
}
