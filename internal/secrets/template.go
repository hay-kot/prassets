// Package secrets renders the secret fields of a store and reads the
// encrypted vault files they can refer to.
package secrets

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"slices"
	"strings"
	"text/template"
	"text/template/parse"

	"github.com/hay-kot/prassets/internal/paths"
)

// Function names a secret template can call.
const (
	FuncEnv   = "env"
	FuncCmd   = "cmd"
	FuncVault = "vault"
	FuncFile  = "file"
)

// Template is a parsed secret template.
type Template struct {
	text string
	tree *template.Template
}

func stubFuncs() template.FuncMap {
	return template.FuncMap{
		FuncEnv:   func(string) (string, error) { return "", nil },
		FuncCmd:   func(string) (string, error) { return "", nil },
		FuncVault: func(string, string) (string, error) { return "", nil },
		FuncFile:  func(string) (string, error) { return "", nil },
	}
}

// Parse parses a secret template without running it.
func Parse(text string) (*Template, error) {
	tree, err := template.New("secret").Option("missingkey=error").Funcs(stubFuncs()).Parse(text)
	if err != nil {
		// The parse error quotes the template, which may hold a literal secret.
		return nil, errors.New("invalid secret template syntax")
	}
	return &Template{text: text, tree: tree}, nil
}

// IsLiteral reports whether the template has no actions, so its text is the
// value itself.
func (t *Template) IsLiteral() bool {
	return !strings.Contains(t.text, "{{")
}

// Sources returns the secret functions the template calls, sorted, such as
// ["env", "vault"]. It is safe to print.
func (t *Template) Sources() []string {
	var out []string
	t.walk(func(name string, _ []parse.Node) {
		if !slices.Contains(out, name) {
			out = append(out, name)
		}
	})
	slices.Sort(out)
	return out
}

// VaultRefs returns the vault names the template passes to vault as string
// literals, so config load can check that each one exists.
func (t *Template) VaultRefs() []string {
	var out []string
	t.walk(func(name string, args []parse.Node) {
		if name != FuncVault || len(args) == 0 {
			return
		}
		if s, ok := args[0].(*parse.StringNode); ok && !slices.Contains(out, s.Text) {
			out = append(out, s.Text)
		}
	})
	return out
}

func (t *Template) walk(fn func(name string, args []parse.Node)) {
	var visit func(n parse.Node)
	visit = func(n parse.Node) {
		switch n := n.(type) {
		case *parse.ListNode:
			if n == nil {
				return
			}
			for _, c := range n.Nodes {
				visit(c)
			}
		case *parse.ActionNode:
			visit(n.Pipe)
		case *parse.PipeNode:
			if n == nil {
				return
			}
			for _, c := range n.Cmds {
				visit(c)
			}
		case *parse.CommandNode:
			if len(n.Args) > 0 {
				if id, ok := n.Args[0].(*parse.IdentifierNode); ok {
					fn(id.Ident, n.Args[1:])
				}
			}
			for _, a := range n.Args {
				visit(a)
			}
		case *parse.IfNode:
			visit(n.Pipe)
			visit(n.List)
			visit(n.ElseList)
		case *parse.WithNode:
			visit(n.Pipe)
			visit(n.List)
			visit(n.ElseList)
		case *parse.RangeNode:
			visit(n.Pipe)
			visit(n.List)
			visit(n.ElseList)
		default:
		}
	}
	for _, tt := range t.tree.Templates() {
		if tt.Tree != nil {
			visit(tt.Root)
		}
	}
}

// Resolver renders secret templates. It decrypts each vault at most once.
type Resolver struct {
	vaults map[string]VaultSpec
	loaded map[string]*Vault
	getenv func(string) string
}

// NewResolver returns a Resolver for the configured vaults.
func NewResolver(vaults map[string]VaultSpec) *Resolver {
	return &Resolver{vaults: vaults, loaded: map[string]*Vault{}, getenv: os.Getenv}
}

// Render runs the template and returns the secret. Errors name the source
// (env var, command, vault path, file) but never a value.
func (r *Resolver) Render(ctx context.Context, t *Template) (string, error) {
	if t.IsLiteral() {
		return t.text, nil
	}
	// text/template wraps function errors with the template text, which can
	// hold a literal secret, so keep the first function error and return it.
	var firstErr error
	keep := func(err error) error {
		if firstErr == nil {
			firstErr = err
		}
		return err
	}
	funcs := template.FuncMap{
		FuncEnv: func(name string) (string, error) {
			v := r.getenv(name)
			if v == "" {
				return "", keep(fmt.Errorf("env var %s is unset or empty", name))
			}
			return v, nil
		},
		FuncCmd: func(command string) (string, error) {
			v, err := runCommand(ctx, command)
			if err != nil {
				return "", keep(err)
			}
			return v, nil
		},
		FuncVault: func(name, path string) (string, error) {
			v, err := r.vaultValue(ctx, name, path)
			if err != nil {
				return "", keep(err)
			}
			return v, nil
		},
		FuncFile: func(path string) (string, error) {
			v, err := readFile(path)
			if err != nil {
				return "", keep(err)
			}
			return v, nil
		},
	}
	tree, err := t.tree.Clone()
	if err != nil {
		return "", errors.New("clone secret template")
	}
	var b strings.Builder
	if err := tree.Funcs(funcs).Execute(&b, nil); err != nil {
		if firstErr != nil {
			return "", firstErr
		}
		return "", errors.New("secret template failed to render")
	}
	return b.String(), nil
}

func (r *Resolver) vaultValue(ctx context.Context, name, path string) (string, error) {
	v, ok := r.loaded[name]
	if !ok {
		spec, found := r.vaults[name]
		if !found {
			return "", fmt.Errorf("vault %q is not defined in the config", name)
		}
		var err error
		if v, err = LoadVault(ctx, name, spec); err != nil {
			return "", err
		}
		r.loaded[name] = v
	}
	return v.Lookup(path)
}

func runCommand(ctx context.Context, command string) (string, error) {
	c := exec.CommandContext(ctx, "sh", "-c", command) //nolint:gosec // cmd runs commands from the user's own global config by design
	var stderr strings.Builder
	c.Stderr = &stderr
	out, err := c.Output()
	if err != nil {
		return "", fmt.Errorf("cmd %q failed: %w: %s", command, err, strings.TrimSpace(stderr.String()))
	}
	return strings.TrimSpace(string(out)), nil
}

func readFile(path string) (string, error) {
	p, err := paths.Expand(path)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(p) //nolint:gosec // the path comes from the user's own global config
	if err != nil {
		return "", fmt.Errorf("file %q: %w", path, err)
	}
	return strings.TrimSpace(string(data)), nil
}
