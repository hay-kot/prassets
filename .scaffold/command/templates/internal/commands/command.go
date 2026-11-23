package commands

import (
	"context"
	"fmt"

	"github.com/urfave/cli/v3"
)

func new{{ .Scaffold.command_name | toPascalCase }}Cmd(flags *Flags) *cli.Command {
	return &cli.Command{
		Name:  "{{ .Scaffold.command_name }}",
		Usage: "{{ .Scaffold.command_name }} command",
		Action: func(ctx context.Context, c *cli.Command) error {
			s, err := openSession(ctx, flags)
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintln(c.Root().Writer, s.resolved.Repo.ID())
			return nil
		},
	}
}
