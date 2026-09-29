package cli

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
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
			data := templateData{Name: t.name, Paths: t.service.Paths, Deps: t.service.Dependencies, Vars: t.service.Vars}
			if data.Vars == nil {
				data.Vars = map[string]string{}
			}
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
