package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/rs/zerolog/log"
	"github.com/urfave/cli/v3"
)

// TypesCmd implements the types command.
type TypesCmd struct {
	flags *Flags
}

// NewTypesCmd creates a new types command.
func NewTypesCmd(flags *Flags) *TypesCmd {
	return &TypesCmd{flags: flags}
}

// Register adds the types command to the application.
func (cmd *TypesCmd) Register(app *cli.Command) *cli.Command {
	app.Commands = append(app.Commands, &cli.Command{
		Name:  "types",
		Usage: "types command",
		Flags: []cli.Flag{
			// Add command-specific flags here
		},
		Action: cmd.run,
	})

	return app
}

func (cmd *TypesCmd) run(_ context.Context, _ *cli.Command) error {
	log.Info().Msg("running types command")
	if cmd.flags.JSON {
		return json.NewEncoder(os.Stdout).Encode(map[string]string{
			"message": "Hello World!",
		})
	}

	fmt.Println("Hello World!")

	return nil
}
