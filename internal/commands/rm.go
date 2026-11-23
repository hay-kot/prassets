package commands

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/mattn/go-isatty"
	"github.com/urfave/cli/v3"
)

func newRmCmd(flags *Flags) *cli.Command {
	return &cli.Command{
		Name:      "rm",
		Usage:     "delete objects; keys outside the repo's types are refused",
		ArgsUsage: "<key, URL or file name>...",
		Flags: []cli.Flag{
			&cli.BoolFlag{Name: "yes", Aliases: []string{"y"}, Usage: "do not ask before deleting"},
		},
		Action: func(ctx context.Context, c *cli.Command) error {
			args := c.Args().Slice()
			if len(args) == 0 {
				return usageErrorf("rm needs at least one key")
			}
			s, err := openSession(ctx, flags)
			if err != nil {
				return err
			}
			matcher, err := s.resolved.Matcher()
			if err != nil {
				return err
			}
			keys := make([]string, 0, len(args))
			for _, a := range args {
				key, err := s.resolveKeyArg(ctx, a)
				if err != nil {
					return err
				}
				if !matcher.Writable(key) {
					return fmt.Errorf("%s matches none of the types of %s and is read-only", key, s.resolved.Repo.ID())
				}
				keys = append(keys, key)
			}

			st, err := s.store(ctx)
			if err != nil {
				return err
			}
			for _, key := range keys {
				exists, err := st.Exists(ctx, key)
				if err != nil {
					return err
				}
				if !exists {
					return fmt.Errorf("%s does not exist", key)
				}
			}

			if !c.Bool("yes") && isatty.IsTerminal(os.Stdin.Fd()) {
				ok, err := confirm(c, fmt.Sprintf("Delete %d object(s)?\n  %s\n", len(keys), strings.Join(keys, "\n  ")))
				if err != nil {
					return err
				}
				if !ok {
					return errors.New("aborted")
				}
			}

			results := make([]Result, 0, len(keys))
			for _, key := range keys {
				if err := st.Delete(ctx, key); err != nil {
					return err
				}
				results = append(results, Result{Key: key, URL: s.url(key), Size: 0, ContentType: contentType(key)})
				if flags.Format != FormatJSON {
					_, _ = fmt.Fprintln(c.Root().Writer, key)
				}
			}
			if flags.Format == FormatJSON {
				return writeJSON(c.Root().Writer, results)
			}
			return nil
		},
	}
}

func confirm(c *cli.Command, prompt string) (bool, error) {
	_, _ = fmt.Fprint(c.Root().ErrWriter, prompt+"[y/N] ")
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil && line == "" {
		return false, err
	}
	answer := strings.ToLower(strings.TrimSpace(line))
	return answer == "y" || answer == "yes", nil
}
