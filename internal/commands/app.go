// Package commands implements the pra command line.
package commands

import (
	"context"
	"fmt"
	"os"

	"github.com/mattn/go-isatty"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/urfave/cli/v3"
)

// NewApp builds the pra command. flags receives the parsed global flags.
func NewApp(flags *Flags, info BuildInfo) *cli.Command {
	app := &cli.Command{
		Name:                  "pra",
		Usage:                 "upload PR screenshots and other public assets to an S3-compatible bucket and print stable URLs",
		Version:               fmt.Sprintf("%s (%s) %s", info.Version, info.Commit, info.Date),
		EnableShellCompletion: true,
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "config", Usage: "global config file", Sources: cli.EnvVars("PRASSETS_CONFIG"), Destination: &flags.ConfigFile},
			&cli.StringFlag{Name: "repo", Usage: "repo id host/owner/name, instead of the git remote", Destination: &flags.Repo},
			&cli.StringFlag{Name: "remote", Usage: "git remote that names the repo", Value: "origin", Destination: &flags.Remote},
			&cli.StringFlag{Name: "branch", Usage: "branch for .Branch, instead of the checked-out branch", Destination: &flags.Branch},
			&cli.StringFlag{Name: "format", Usage: "output format: url, md or json", Value: FormatURL, Destination: &flags.Format},
			&cli.StringFlag{Name: "log-level", Usage: "log level (debug, info, warn, error)", Sources: cli.EnvVars("PRASSETS_LOG_LEVEL"), Value: "info", Destination: &flags.LogLevel},
			&cli.BoolFlag{Name: "no-color", Usage: "disable colored output", Sources: cli.EnvVars("NO_COLOR"), Destination: &flags.NoColor},
		},
		Before: func(ctx context.Context, _ *cli.Command) (context.Context, error) {
			switch flags.Format {
			case FormatURL, FormatMD, FormatJSON:
			default:
				return ctx, usageErrorf("--format must be url, md or json, not %q", flags.Format)
			}
			level, err := zerolog.ParseLevel(flags.LogLevel)
			if err != nil {
				return ctx, usageErrorf("--log-level: %v", err)
			}
			if !isatty.IsTerminal(os.Stderr.Fd()) {
				flags.NoColor = true
			}
			log.Logger = zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr, NoColor: flags.NoColor}).Level(level).With().Timestamp().Logger()
			return ctx, nil
		},
		Commands: []*cli.Command{
			newPutCmd(flags),
			newLsCmd(flags),
			newMdCmd(flags),
			newGetCmd(flags),
			newRmCmd(flags),
			newTypesCmd(flags),
			newConfigCmd(flags),
			newVaultCmd(flags),
			newVersionCmd(flags, info),
		},
	}
	markUsageErrors(app)
	return app
}
