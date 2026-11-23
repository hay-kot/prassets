package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/rs/zerolog/log"
	"github.com/urfave/cli/v3"
)

// RmCmd implements the rm command.
type RmCmd struct {
	flags *Flags
}

// NewRmCmd creates a new rm command.
func NewRmCmd(flags *Flags) *RmCmd {
	return &RmCmd{flags: flags}
}

// Register adds the rm command to the application.
func (cmd *RmCmd) Register(app *cli.Command) *cli.Command {
	app.Commands = append(app.Commands, &cli.Command{
		Name:  "rm",
		Usage: "rm command",
		Flags: []cli.Flag{
			// Add command-specific flags here
		},
		Action: cmd.run,
	})

	return app
}

func (cmd *RmCmd) run(_ context.Context, _ *cli.Command) error {
	log.Info().Msg("running rm command")
	if cmd.flags.JSON {
		return json.NewEncoder(os.Stdout).Encode(map[string]string{
			"message": "Hello World!",
		})
	}

	fmt.Println("Hello World!")

	return nil
}
