package commands

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/mattn/go-isatty"
	"github.com/rs/zerolog/log"
	"github.com/urfave/cli/v3"

	"github.com/hay-kot/prassets/internal/config"
	"github.com/hay-kot/prassets/internal/secrets"
)

const emptyVault = "# prassets vault: nested maps with string leaves, for example\n# r2:\n#   access_key: ...\n"

func newVaultCmd(flags *Flags) *cli.Command {
	vaultAction := func(fn func(ctx context.Context, c *cli.Command, name string, spec secrets.VaultSpec) error) cli.ActionFunc {
		return func(ctx context.Context, c *cli.Command) error {
			if c.Args().Len() != 1 {
				return usageErrorf("%s takes exactly one vault name", c.Name)
			}
			g, err := flags.loadGlobal()
			if err != nil {
				return err
			}
			name := c.Args().First()
			v, ok := g.Config.Vaults[name]
			if !ok {
				return fmt.Errorf("vault %q is not defined under vaults in %s", name, g.Path)
			}
			return fn(ctx, c, name, config.VaultSpecs(map[string]config.Vault{name: v})[name])
		}
	}
	return &cli.Command{
		Name:  "vault",
		Usage: "create, edit and inspect encrypted vault files",
		Commands: []*cli.Command{
			{
				Name:      "init",
				Usage:     "create an empty age vault",
				ArgsUsage: "<name>",
				Action:    vaultAction(vaultInit),
			},
			{
				Name:      "edit",
				Usage:     "decrypt an age vault, open $EDITOR and encrypt it again",
				ArgsUsage: "<name>",
				Action:    vaultAction(vaultEdit),
			},
			{
				Name:      "keys",
				Usage:     "list the dotted key paths in a vault, never the values",
				ArgsUsage: "<name>",
				Action: vaultAction(func(ctx context.Context, c *cli.Command, name string, spec secrets.VaultSpec) error {
					v, err := secrets.LoadVault(ctx, name, spec)
					if err != nil {
						return err
					}
					keys := v.Keys()
					if flags.Format == FormatJSON {
						if keys == nil {
							keys = []string{}
						}
						return writeJSON(c.Root().Writer, keys)
					}
					for _, k := range keys {
						_, _ = fmt.Fprintln(c.Root().Writer, k)
					}
					return nil
				}),
			},
		},
	}
}

func requireAge(name string, spec secrets.VaultSpec) error {
	if spec.Format != secrets.FormatAge {
		return fmt.Errorf("vault %q uses format %s; edit it with sops", name, spec.Format)
	}
	return nil
}

func vaultInit(_ context.Context, c *cli.Command, name string, spec secrets.VaultSpec) error {
	if err := requireAge(name, spec); err != nil {
		return err
	}
	p, err := spec.FilePath()
	if err != nil {
		return err
	}
	if _, err := os.Stat(p); err == nil {
		return fmt.Errorf("vault %q already exists at %s", name, p)
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	if err := secrets.EncryptAge(spec, []byte(emptyVault), true); err != nil {
		return err
	}
	_, _ = fmt.Fprintln(c.Root().Writer, p)
	return nil
}

func vaultEdit(ctx context.Context, c *cli.Command, name string, spec secrets.VaultSpec) error {
	if err := requireAge(name, spec); err != nil {
		return err
	}
	armored, err := secrets.IsArmored(spec)
	if err != nil {
		return fmt.Errorf("vault %q: %w; create it with pra vault init %s", name, err, name)
	}
	plain, err := secrets.Decrypt(ctx, spec)
	if err != nil {
		return fmt.Errorf("vault %q: %w", name, err)
	}

	dir, err := os.MkdirTemp("", "prassets-vault-")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(dir) }()
	tmp := filepath.Join(dir, name+".yaml")
	if err := os.WriteFile(tmp, plain, 0o600); err != nil {
		return err
	}

	for {
		if err := runEditor(ctx, tmp); err != nil {
			return err
		}
		edited, err := os.ReadFile(tmp) //nolint:gosec // our own temp file
		if err != nil {
			return err
		}
		if _, perr := secrets.ParseDocument(edited); perr != nil {
			if !isatty.IsTerminal(os.Stdin.Fd()) {
				return fmt.Errorf("vault %q not saved: %w", name, perr)
			}
			again, err := confirm(c, fmt.Sprintf("%v\nOpen the editor again? ", perr))
			if err != nil || !again {
				return fmt.Errorf("vault %q not saved: %w", name, perr)
			}
			continue
		}
		if bytes.Equal(edited, plain) {
			log.Info().Str("vault", name).Msg("no changes")
			return nil
		}
		if err := secrets.EncryptAge(spec, edited, armored); err != nil {
			return err
		}
		log.Info().Str("vault", name).Msg("saved")
		return nil
	}
}

func runEditor(ctx context.Context, path string) error {
	editor := os.Getenv("VISUAL")
	if editor == "" {
		editor = os.Getenv("EDITOR")
	}
	if editor == "" {
		editor = "vi"
	}
	// $EDITOR may carry arguments, such as "code --wait".
	cmd := exec.CommandContext(ctx, "sh", "-c", editor+` "$1"`, "sh", path) //nolint:gosec // the user's own $EDITOR
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("editor %q: %w", editor, err)
	}
	return nil
}
