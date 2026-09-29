package cli

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"text/template"

	"github.com/spf13/cobra"
)

// templateData is the value a template is executed with, once per service.
type templateData struct {
	Name    string
	Paths   []string
	Deps    []string
	Version string
	Tag     string
	Vars    map[string]string
}

// templateVars returns vars with "-" replaced by "_" in the keys, so that a key
// like docker-file can be written as .Vars.docker_file.
func templateVars(name string, vars map[string]string) (map[string]string, error) {
	keys := make([]string, 0, len(vars))
	for k := range vars {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	result := make(map[string]string, len(vars))
	seen := map[string]string{}
	for _, k := range keys {
		n := strings.ReplaceAll(k, "-", "_")
		if other, ok := seen[n]; ok {
			return nil, fmt.Errorf("service %s: variables %q and %q both map to .Vars.%s", name, other, k, n)
		}
		seen[n] = k
		result[n] = vars[k]
	}
	return result, nil
}

func (a *app) templateCommand() *cobra.Command {
	var output string
	c := &cobra.Command{Use: "template [service] <file>", Args: cobra.RangeArgs(1, 2), RunE: func(cmd *cobra.Command, args []string) error {
		file := args[len(args)-1]
		src, err := os.ReadFile(file)
		if err != nil {
			return codedError{1, err}
		}
		tpl, err := template.New(filepath.Base(file)).Option("missingkey=error").Parse(string(src))
		if err != nil {
			return codedError{1, err}
		}
		var outTpl *template.Template
		if output != "" {
			if outTpl, err = template.New("--output").Option("missingkey=error").Parse(output); err != nil {
				return codedError{1, err}
			}
		}
		sel, err := a.selectServices(args[:len(args)-1], false)
		if err != nil {
			return err
		}

		// Everything is rendered before writing, so an error leaves no partial output.
		type rendered struct {
			path    string
			content []byte
		}
		results := make([]rendered, 0, len(sel))
		owners := map[string]string{}
		for _, t := range sel {
			vars, err := templateVars(t.name, t.service.Vars)
			if err != nil {
				return codedError{2, err}
			}
			data := templateData{Name: t.name, Paths: t.service.Paths, Deps: t.service.Dependencies, Vars: vars}
			if t.release != nil {
				data.Version, data.Tag = t.release.Version.String(), t.release.Tag
			}
			var buf bytes.Buffer
			if err = tpl.Execute(&buf, data); err != nil {
				return codedError{1, fmt.Errorf("service %s: %w", t.name, err)}
			}
			r := rendered{content: buf.Bytes()}
			if outTpl != nil {
				var p bytes.Buffer
				if err = outTpl.Execute(&p, data); err != nil {
					return codedError{1, fmt.Errorf("service %s: %w", t.name, err)}
				}
				if r.path = p.String(); r.path == "" {
					return codedError{1, fmt.Errorf("service %s: --output is empty", t.name)}
				}
				if other, ok := owners[filepath.Clean(r.path)]; ok {
					return codedError{1, fmt.Errorf("services %s and %s both write %s; use a per-service --output such as '{{.Name}}.yaml'", other, t.name, r.path)}
				}
				owners[filepath.Clean(r.path)] = t.name
			}
			results = append(results, r)
		}

		for _, r := range results {
			if r.path == "" {
				if _, err = a.out.Write(r.content); err != nil {
					return err
				}
				continue
			}
			if err = os.MkdirAll(filepath.Dir(r.path), 0o755); err == nil {
				err = os.WriteFile(r.path, r.content, 0o644)
			}
			if err != nil {
				return codedError{1, err}
			}
			fmt.Fprintln(a.out, r.path)
		}
		return nil
	}}
	c.Flags().StringVarP(&output, "output", "o", "", "output file, itself a template (e.g. 'deploy/{{.Name}}.yaml'); default stdout")
	return c
}
