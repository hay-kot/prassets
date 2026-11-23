package commands

import (
	"context"
	"strings"

	"github.com/urfave/cli/v3"

	"github.com/hay-kot/prassets/internal/layout"
)

func newLsCmd(flags *Flags) *cli.Command {
	return &cli.Command{
		Name:      "ls",
		Usage:     "list this branch's PR assets, another type's, or everything under the repo prefix",
		ArgsUsage: "[prefix]",
		Flags: append(typeFlags(),
			&cli.BoolFlag{Name: "all", Usage: "list everything under the repo prefix"},
		),
		Action: func(ctx context.Context, c *cli.Command) error {
			return runLs(ctx, c, flags, flags.Format)
		},
	}
}

func newMdCmd(flags *Flags) *cli.Command {
	return &cli.Command{
		Name:      "md",
		Usage:     "print Markdown for this branch's assets, ready to paste in a PR body (ls --format md)",
		ArgsUsage: "[prefix]",
		Flags: append(typeFlags(),
			&cli.BoolFlag{Name: "all", Usage: "everything under the repo prefix"},
		),
		Action: func(ctx context.Context, c *cli.Command) error {
			return runLs(ctx, c, flags, FormatMD)
		},
	}
}

func runLs(ctx context.Context, c *cli.Command, flags *Flags, format string) error {
	if c.Args().Len() > 1 {
		return usageErrorf("ls takes at most one prefix")
	}
	raw := c.Args().First()
	all := c.Bool("all")
	typeSet := c.IsSet("type") || c.IsSet("slug")
	if (all && raw != "") || ((all || raw != "") && typeSet) {
		return usageErrorf("use only one of --all, a prefix, or -t/-s")
	}

	s, err := openSession(ctx, flags)
	if err != nil {
		return err
	}

	var prefix string
	switch {
	case all:
		prefix = s.resolved.Prefix
	case raw != "":
		if prefix, err = repoPrefix(s.resolved.Prefix, raw); err != nil {
			return err
		}
	default:
		typ, err := s.resolved.Type(c.String("type"))
		if err != nil {
			return &UsageError{Err: err}
		}
		data, err := s.keyData(ctx, typ, c.String("slug"), "")
		if err != nil {
			return err
		}
		dir, err := typ.Template.Dir(data)
		if err != nil {
			return err
		}
		prefix = s.resolved.Prefix + dir
	}

	st, err := s.store(ctx)
	if err != nil {
		return err
	}
	objects, err := st.List(ctx, prefix)
	if err != nil {
		return err
	}
	results := make([]Result, 0, len(objects))
	for _, o := range objects {
		results = append(results, Result{Key: o.Key, URL: s.url(o.Key), Size: o.Size, ContentType: contentType(o.Key)})
	}
	return writeListing(c.Root().Writer, format, results)
}

// repoPrefix keeps a user-given listing prefix inside the repo prefix.
func repoPrefix(base, raw string) (string, error) {
	p := raw
	if !strings.HasPrefix(p, base) {
		p = base + strings.TrimPrefix(p, "/")
	}
	if err := layout.CheckKey(p + "x"); err != nil {
		return "", usageErrorf("invalid prefix %q: %v", raw, err)
	}
	return p, nil
}
