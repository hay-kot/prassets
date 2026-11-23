package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/rs/zerolog/log"
	"github.com/urfave/cli/v3"
)

// PutCmd implements the put command.
type PutCmd struct {
	flags *Flags
}

// NewPutCmd creates a new put command.
func NewPutCmd(flags *Flags) *PutCmd {
	return &PutCmd{flags: flags}
}

// Register adds the put command to the application.
func (cmd *PutCmd) Register(app *cli.Command) *cli.Command {
	app.Commands = append(app.Commands, &cli.Command{
		Name:  "put",
		Usage: "put command",
		Flags: []cli.Flag{
			// Add command-specific flags here
		},
		Action: cmd.run,
	})

	return app
}

func (cmd *PutCmd) run(_ context.Context, _ *cli.Command) error {
	log.Info().Msg("running put command")
	if cmd.flags.JSON {
		return json.NewEncoder(os.Stdout).Encode(map[string]string{
			"message": "Hello World!",
		})
	}

	fmt.Println("Hello World!")

	return nil
}
