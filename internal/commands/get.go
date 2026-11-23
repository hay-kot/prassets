package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/rs/zerolog/log"
	"github.com/urfave/cli/v3"
)

// GetCmd implements the get command.
type GetCmd struct {
	flags *Flags
}

// NewGetCmd creates a new get command.
func NewGetCmd(flags *Flags) *GetCmd {
	return &GetCmd{flags: flags}
}

// Register adds the get command to the application.
func (cmd *GetCmd) Register(app *cli.Command) *cli.Command {
	app.Commands = append(app.Commands, &cli.Command{
		Name:  "get",
		Usage: "get command",
		Flags: []cli.Flag{
			// Add command-specific flags here
		},
		Action: cmd.run,
	})

	return app
}

func (cmd *GetCmd) run(_ context.Context, _ *cli.Command) error {
	log.Info().Msg("running get command")
	if cmd.flags.JSON {
		return json.NewEncoder(os.Stdout).Encode(map[string]string{
			"message": "Hello World!",
		})
	}

	fmt.Println("Hello World!")

	return nil
}
