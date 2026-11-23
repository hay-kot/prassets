package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/rs/zerolog/log"
	"github.com/urfave/cli/v3"
)

// LsCmd implements the ls command.
type LsCmd struct {
	flags *Flags
}

// NewLsCmd creates a new ls command.
func NewLsCmd(flags *Flags) *LsCmd {
	return &LsCmd{flags: flags}
}

// Register adds the ls command to the application.
func (cmd *LsCmd) Register(app *cli.Command) *cli.Command {
	app.Commands = append(app.Commands, &cli.Command{
		Name:  "ls",
		Usage: "ls command",
		Flags: []cli.Flag{
			// Add command-specific flags here
		},
		Action: cmd.run,
	})

	return app
}

func (cmd *LsCmd) run(_ context.Context, _ *cli.Command) error {
	log.Info().Msg("running ls command")
	if cmd.flags.JSON {
		return json.NewEncoder(os.Stdout).Encode(map[string]string{
			"message": "Hello World!",
		})
	}

	fmt.Println("Hello World!")

	return nil
}
