package commands

import (
	"context"

	"github.com/urfave/cli/v3"

	"github.com/hay-kot/prassets/internal/config"
)

func newSchemaCmd() *cli.Command {
	return &cli.Command{
		Name:      "schema",
		Usage:     "print the JSON schema of the global config (global) or of .prassets.yaml (repo)",
		ArgsUsage: "[global|repo]",
		Action: func(_ context.Context, c *cli.Command) error {
			if c.Args().Len() > 1 {
				return usageErrorf("schema takes at most one argument: global or repo")
			}
			var gen func() ([]byte, error)
			switch c.Args().First() {
			case "", "global":
				gen = config.GlobalSchema
			case "repo":
				gen = config.RepoSchema
			default:
				return usageErrorf("unknown schema %q: use global or repo", c.Args().First())
			}
			data, err := gen()
			if err != nil {
				return err
			}
			_, err = c.Root().Writer.Write(data)
			return err
		},
	}
}
