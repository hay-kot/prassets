package commands

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"

	"github.com/urfave/cli/v3"
)

func newGetCmd(flags *Flags) *cli.Command {
	return &cli.Command{
		Name:      "get",
		Usage:     "download one object and print the local path",
		ArgsUsage: "<key, URL or file name>",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "dir", Aliases: []string{"o"}, Usage: "directory to save into", Value: "."},
		},
		Action: func(ctx context.Context, c *cli.Command) error {
			if c.Args().Len() != 1 {
				return usageErrorf("get takes exactly one key")
			}
			s, err := openSession(ctx, flags)
			if err != nil {
				return err
			}
			key, err := s.resolveKeyArg(ctx, c.Args().First())
			if err != nil {
				return err
			}
			st, err := s.store(ctx)
			if err != nil {
				return err
			}
			exists, err := st.Exists(ctx, key)
			if err != nil {
				return err
			}
			if !exists {
				return fmt.Errorf("%s does not exist", key)
			}

			dest := filepath.Join(c.String("dir"), path.Base(key))
			f, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644) //nolint:gosec // the user picks the directory
			if err != nil {
				if errors.Is(err, fs.ErrExist) {
					return fmt.Errorf("%s already exists; remove it or pass -o with another directory", dest)
				}
				return err
			}
			if err := st.Get(ctx, key, f); err != nil {
				_ = f.Close()
				_ = os.Remove(dest)
				return err
			}
			if err := f.Close(); err != nil {
				return err
			}
			_, _ = fmt.Fprintln(c.Root().Writer, dest)
			return nil
		},
	}
}
