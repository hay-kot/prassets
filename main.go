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

	"github.com/hay-kot/prassets/internal/commands"
)

var (
	// Build information. Populated at build-time via -ldflags flag.
	version = "dev"
	commit  = "HEAD"
	date    = "now"
)

func buildInfo() commands.BuildInfo {
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
	return commands.BuildInfo{Version: version, Commit: short, Date: date}
}

func main() {
	os.Exit(run())
}

func run() int {
	log.Logger = zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr}).With().Timestamp().Logger()

	flags := &commands.Flags{}
	app := commands.NewApp(flags, buildInfo())

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
		if commands.IsUsageError(err) {
			return 2
		}
		return 1
	}

	return 0
}
