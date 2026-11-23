package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/rs/zerolog/log"
	"github.com/urfave/cli/v3"
)

// ConfigCmd implements the config command.
type ConfigCmd struct {
	flags *Flags
}

// NewConfigCmd creates a new config command.
func NewConfigCmd(flags *Flags) *ConfigCmd {
	return &ConfigCmd{flags: flags}
}

// Register adds the config command to the application.
func (cmd *ConfigCmd) Register(app *cli.Command) *cli.Command {
	app.Commands = append(app.Commands, &cli.Command{
		Name:  "config",
		Usage: "config command",
		Flags: []cli.Flag{
			// Add command-specific flags here
		},
		Action: cmd.run,
	})

	return app
}

func (cmd *ConfigCmd) run(_ context.Context, _ *cli.Command) error {
	log.Info().Msg("running config command")
	if cmd.flags.JSON {
		return json.NewEncoder(os.Stdout).Encode(map[string]string{
			"message": "Hello World!",
		})
	}

	fmt.Println("Hello World!")

	return nil
}
