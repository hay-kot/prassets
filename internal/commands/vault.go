package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/rs/zerolog/log"
	"github.com/urfave/cli/v3"
)

// VaultCmd implements the vault command.
type VaultCmd struct {
	flags *Flags
}

// NewVaultCmd creates a new vault command.
func NewVaultCmd(flags *Flags) *VaultCmd {
	return &VaultCmd{flags: flags}
}

// Register adds the vault command to the application.
func (cmd *VaultCmd) Register(app *cli.Command) *cli.Command {
	app.Commands = append(app.Commands, &cli.Command{
		Name:  "vault",
		Usage: "vault command",
		Flags: []cli.Flag{
			// Add command-specific flags here
		},
		Action: cmd.run,
	})

	return app
}

func (cmd *VaultCmd) run(_ context.Context, _ *cli.Command) error {
	log.Info().Msg("running vault command")
	if cmd.flags.JSON {
		return json.NewEncoder(os.Stdout).Encode(map[string]string{
			"message": "Hello World!",
		})
	}

	fmt.Println("Hello World!")

	return nil
}
