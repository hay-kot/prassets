package commands

import (
	"context"
	"fmt"
	"text/tabwriter"

	"github.com/urfave/cli/v3"
)

type typeInfo struct {
	Name     string `json:"name"       yaml:"name"`
	Key      string `json:"key"        yaml:"key"`
	Source   string `json:"source"     yaml:"source"`
	Usage    string `json:"usage"      yaml:"usage"`
	NeedSlug bool   `json:"needs_slug" yaml:"needs_slug"`
}

func (s *session) typeInfos() []typeInfo {
	out := make([]typeInfo, 0, len(s.resolved.Types))
	for _, t := range s.resolved.Types {
		out = append(out, typeInfo{
			Name:     t.Name,
			Key:      s.resolved.Prefix + t.Template.Text(),
			Source:   t.Source,
			Usage:    t.Usage,
			NeedSlug: t.Template.UsesSlug(),
		})
	}
	return out
}

func newTypesCmd(flags *Flags) *cli.Command {
	return &cli.Command{
		Name:  "types",
		Usage: "list the repo's asset types: name, key template, source and usage",
		Action: func(ctx context.Context, c *cli.Command) error {
			s, err := openSession(ctx, flags)
			if err != nil {
				return err
			}
			infos := s.typeInfos()
			if flags.Format == FormatJSON {
				return writeJSON(c.Root().Writer, infos)
			}
			tw := tabwriter.NewWriter(c.Root().Writer, 0, 0, 2, ' ', 0)
			_, _ = fmt.Fprintln(tw, "NAME\tKEY\tSLUG\tSOURCE\tUSAGE")
			for _, t := range infos {
				slug := "-"
				if t.NeedSlug {
					slug = "required"
				}
				_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", t.Name, t.Key, slug, t.Source, t.Usage)
			}
			return tw.Flush()
		},
	}
}
