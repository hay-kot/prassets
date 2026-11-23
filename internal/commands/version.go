package commands

import (
	"context"
	"fmt"

	"github.com/urfave/cli/v3"
)

// BuildInfo describes the binary.
type BuildInfo struct {
	Version string `json:"version"`
	Commit  string `json:"commit"`
	Date    string `json:"date"`
}

func newVersionCmd(flags *Flags, info BuildInfo) *cli.Command {
	return &cli.Command{
		Name:  "version",
		Usage: "print the version and commit",
		Action: func(_ context.Context, c *cli.Command) error {
			if flags.Format == FormatJSON {
				return writeJSON(c.Root().Writer, info)
			}
			_, _ = fmt.Fprintf(c.Root().Writer, "pra %s (%s) %s\n", info.Version, info.Commit, info.Date)
			return nil
		},
	}
}
