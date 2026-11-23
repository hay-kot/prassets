package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/urfave/cli/v3"

	"github.com/hay-kot/prassets/internal/commands"
	"github.com/hay-kot/prassets/internal/config"
)

var (
	// Build information. Populated at build-time via -ldflags flag.
	version = "dev"
	commit  = "HEAD"
	date    = "now"
)

func build() string {
	if version == "dev" {
		if info, ok := debug.ReadBuildInfo(); ok {
			version = info.Main.Version
			for _, s := range info.Settings {
				switch s.Key {
				case "vcs.revision":
					commit = s.Value
				case "vcs.time":
					date = s.Value
				}
			}
		}
	}

	short := commit
	if len(commit) > 7 {
		short = commit[:7]
	}

	return fmt.Sprintf("%s (%s) %s", version, short, date)
}

func setupLogger(level string, noColor bool) error {
	parsedLevel, err := zerolog.ParseLevel(level)
	if err != nil {
		return fmt.Errorf("failed to parse log level: %w", err)
	}

	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, NoColor: noColor}).Level(parsedLevel)

	return nil
}

func main() {
	os.Exit(run())
}

func run() int {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	flags := &commands.Flags{}

	app := &cli.Command{
		Name:                  "prassets",
		Usage:                 `Upload PR screenshots and other public assets to an S3-compatible bucket and print stable URLs`,
		Version:               build(),
		EnableShellCompletion: true,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "log-level",
				Usage:       "log level (debug, info, warn, error, fatal, panic)",
				Sources:     cli.EnvVars("LOG_LEVEL"),
				Value:       "info",
				Destination: &flags.LogLevel,
			},
			&cli.BoolFlag{
				Name:        "no-color",
				Usage:       "disable colored output",
				Sources:     cli.EnvVars("NO_COLOR"),
				Destination: &flags.NoColor,
			},
			&cli.StringFlag{
				Name:        "config",
				Usage:       "path to config file",
				Sources:     cli.EnvVars("CONFIG_FILE"),
				Destination: &flags.ConfigFile,
			},
			&cli.BoolFlag{
				Name:        "json",
				Usage:       "output in JSON format",
				Destination: &flags.JSON,
			},
		},
		Before: func(ctx context.Context, c *cli.Command) (context.Context, error) {
			cfg, err := func() (config.Config, error) {
				if flags.ConfigFile != "" {
					return config.ReadFrom(flags.ConfigFile)
				}
				return config.Read()
			}()
			if err != nil {
				return ctx, fmt.Errorf("loading config: %w", err)
			}

			if !c.IsSet("log-level") && cfg.LogLevel != "" {
				flags.LogLevel = cfg.LogLevel
			}
			if err := setupLogger(flags.LogLevel, flags.NoColor); err != nil {
				return ctx, err
			}

			return ctx, nil
		},
	}
	app = commands.NewPutCmd(flags).Register(app)
	app = commands.NewLsCmd(flags).Register(app)
	app = commands.NewMdCmd(flags).Register(app)
	app = commands.NewGetCmd(flags).Register(app)
	app = commands.NewRmCmd(flags).Register(app)
	app = commands.NewTypesCmd(flags).Register(app)
	app = commands.NewConfigCmd(flags).Register(app)
	app = commands.NewVaultCmd(flags).Register(app)
	// +scaffold:command:register

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := app.Run(ctx, os.Args); err != nil {
		colorRed := "\033[38;2;215;95;107m"
		colorGray := "\033[38;2;163;163;163m"
		colorReset := "\033[0m"
		if flags.NoColor {
			colorRed = ""
			colorGray = ""
			colorReset = ""
		}
		fmt.Fprintf(os.Stderr, "\n%s╭ Error%s\n%s│%s %s%s%s\n%s╵%s\n",
			colorRed, colorReset,
			colorRed, colorReset, colorGray, err.Error(), colorReset,
			colorRed, colorReset,
		)
		return 1
	}

	return 0
}
