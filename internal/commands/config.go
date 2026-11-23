package commands

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/urfave/cli/v3"
	"go.yaml.in/yaml/v3"

	"github.com/hay-kot/prassets/internal/config"
	"github.com/hay-kot/prassets/internal/secrets"
)

func newConfigCmd(flags *Flags) *cli.Command {
	return &cli.Command{
		Name:  "config",
		Usage: "show, create and check the global config",
		Commands: []*cli.Command{
			{
				Name:  "path",
				Usage: "print the global config path",
				Action: func(_ context.Context, c *cli.Command) error {
					p, err := flags.configPath()
					if err != nil {
						return err
					}
					_, _ = fmt.Fprintln(c.Root().Writer, p)
					return nil
				},
			},
			{
				Name:  "init",
				Usage: "write a commented starter config if none exists",
				Action: func(_ context.Context, c *cli.Command) error {
					return configInit(c, flags)
				},
			},
			{
				Name:  "show",
				Usage: "print the resolved config for the current repo, secrets redacted",
				Action: func(ctx context.Context, c *cli.Command) error {
					s, err := openSession(ctx, flags)
					if err != nil {
						return err
					}
					view := showView(s)
					if flags.Format == FormatJSON {
						return writeJSON(c.Root().Writer, view)
					}
					enc := yaml.NewEncoder(c.Root().Writer)
					enc.SetIndent(2)
					return enc.Encode(view)
				},
			},
			{
				Name:  "check",
				Usage: "resolve the repo, store, templates and secrets, then HEAD the bucket",
				Action: func(ctx context.Context, c *cli.Command) error {
					return configCheck(ctx, c.Root().Writer, flags)
				},
			},
		},
	}
}

func configInit(c *cli.Command, flags *Flags) error {
	p, err := flags.configPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	f, err := os.OpenFile(p, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600) //nolint:gosec // the user's own config path
	if err != nil {
		if errors.Is(err, fs.ErrExist) {
			return fmt.Errorf("%s already exists; edit it or remove it first", p)
		}
		return err
	}
	if _, err := io.WriteString(f, config.Starter); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	_, _ = fmt.Fprintln(c.Root().Writer, p)
	return nil
}

type storeView struct {
	Name         string `json:"name"                    yaml:"name"`
	Endpoint     string `json:"endpoint"                yaml:"endpoint"`
	Bucket       string `json:"bucket"                  yaml:"bucket"`
	Region       string `json:"region"                  yaml:"region"`
	PublicURL    string `json:"public_url"              yaml:"public_url"`
	Insecure     bool   `json:"insecure"                yaml:"insecure"`
	AccessKey    string `json:"access_key"              yaml:"access_key"`
	SecretKey    string `json:"secret_key"              yaml:"secret_key"`
	SessionToken string `json:"session_token,omitempty" yaml:"session_token,omitempty"`
}

type configView struct {
	Config   string     `json:"config"    yaml:"config"`
	Repo     string     `json:"repo"      yaml:"repo"`
	RepoFile string     `json:"repo_file" yaml:"repo_file"`
	Entry    string     `json:"entry"     yaml:"entry"`
	Prefix   string     `json:"prefix"    yaml:"prefix"`
	Store    storeView  `json:"store"     yaml:"store"`
	Types    []typeInfo `json:"types"     yaml:"types"`
}

// redact describes a secret field without its value: which functions it
// reads from, or that it is a literal.
func redact(text string) string {
	if text == "" {
		return ""
	}
	t, err := secrets.Parse(text)
	if err != nil {
		return "<redacted: invalid template>"
	}
	if t.IsLiteral() {
		return "<redacted: literal>"
	}
	return "<redacted: from " + strings.Join(t.Sources(), ", ") + ">"
}

func showView(s *session) configView {
	r := s.resolved
	repoFile := r.RepoFilePath
	if repoFile == "" {
		repoFile = "none"
	}
	return configView{
		Config:   r.ConfigPath,
		Repo:     r.Repo.ID(),
		RepoFile: repoFile,
		Entry:    r.Entry,
		Prefix:   r.Prefix,
		Store: storeView{
			Name:         r.StoreName,
			Endpoint:     r.Store.Endpoint,
			Bucket:       r.Store.Bucket,
			Region:       r.Store.RegionOrDefault(),
			PublicURL:    r.Store.PublicURL,
			Insecure:     r.Store.Insecure,
			AccessKey:    redact(r.Store.AccessKey),
			SecretKey:    redact(r.Store.SecretKey),
			SessionToken: redact(r.Store.SessionToken),
		},
		Types: s.typeInfos(),
	}
}

type checker struct {
	w      io.Writer
	failed bool
}

func (ch *checker) ok(step, detail string) {
	_, _ = fmt.Fprintf(ch.w, "ok    %-9s %s\n", step, detail)
}

func (ch *checker) warn(step string, err error) {
	_, _ = fmt.Fprintf(ch.w, "warn  %-9s %v\n", step, err)
}

func (ch *checker) fail(step string, err error) error {
	ch.failed = true
	_, _ = fmt.Fprintf(ch.w, "FAIL  %-9s %v\n", step, err)
	return errors.New("config check failed")
}

func configCheck(ctx context.Context, w io.Writer, flags *Flags) error {
	ch := &checker{w: w, failed: false}
	global, err := flags.loadGlobal()
	if err != nil {
		return ch.fail("config", err)
	}
	ch.ok("config", global.Path)

	s, err := openSession(ctx, flags)
	if err != nil {
		return ch.fail("repo", err)
	}
	r := s.resolved
	ch.ok("repo", r.Repo.ID()+" ("+r.Entry+")")
	if r.RepoFilePath != "" {
		ch.ok("repo file", r.RepoFilePath)
	} else {
		ch.ok("repo file", "none")
	}
	ch.ok("types", fmt.Sprintf("%d type(s), prefix %q", len(r.Types), r.Prefix))

	if pr, err := r.Type(config.PRType); err == nil && pr.Template.UsesBranch() {
		if b, err := s.branch(ctx); err != nil {
			ch.warn("branch", err)
		} else {
			ch.ok("branch", b)
		}
	}

	if _, err := renderSecrets(ctx, r); err != nil {
		return ch.fail("secrets", err)
	}
	ch.ok("secrets", "store "+r.StoreName+" credentials rendered")

	st, err := s.store(ctx)
	if err != nil {
		return ch.fail("bucket", err)
	}
	if err := st.Check(ctx); err != nil {
		return ch.fail("bucket", err)
	}
	ch.ok("bucket", r.Store.Endpoint+"/"+r.Store.Bucket)
	return nil
}
