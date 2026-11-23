package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/rs/zerolog/log"
	"github.com/urfave/cli/v3"
)

// MdCmd implements the md command.
type MdCmd struct {
	flags *Flags
}

// NewMdCmd creates a new md command.
func NewMdCmd(flags *Flags) *MdCmd {
	return &MdCmd{flags: flags}
}

// Register adds the md command to the application.
func (cmd *MdCmd) Register(app *cli.Command) *cli.Command {
	app.Commands = append(app.Commands, &cli.Command{
		Name:  "md",
		Usage: "md command",
		Flags: []cli.Flag{
			// Add command-specific flags here
		},
		Action: cmd.run,
	})

	return app
}

func (cmd *MdCmd) run(_ context.Context, _ *cli.Command) error {
	log.Info().Msg("running md command")
	if cmd.flags.JSON {
		return json.NewEncoder(os.Stdout).Encode(map[string]string{
			"message": "Hello World!",
		})
	}

	fmt.Println("Hello World!")

	return nil
}
