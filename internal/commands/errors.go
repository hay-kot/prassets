package commands

import (
	"context"
	"errors"
	"fmt"

	"github.com/urfave/cli/v3"
)

// UsageError is a mistake in the command line. main exits with code 2 for it.
type UsageError struct {
	Err error
}

func (e *UsageError) Error() string { return e.Err.Error() }

func (e *UsageError) Unwrap() error { return e.Err }

func usageErrorf(format string, args ...any) error {
	return &UsageError{Err: fmt.Errorf(format, args...)}
}

// IsUsageError reports whether err is a usage error.
func IsUsageError(err error) bool {
	_, ok := errors.AsType[*UsageError](err)
	return ok
}

func onUsageError(_ context.Context, _ *cli.Command, err error, _ bool) error {
	return &UsageError{Err: err}
}

// groupAction shows help for a command that only holds subcommands, and makes
// an unknown subcommand a usage error instead of urfave/cli's exit code 3.
func groupAction(_ context.Context, c *cli.Command) error {
	if c.Args().Present() {
		return usageErrorf("unknown command %q; run %s --help", c.Args().First(), c.FullName())
	}
	if c.Root() == c {
		return cli.ShowRootCommandHelp(c)
	}
	return cli.ShowSubcommandHelp(c)
}

// markUsageErrors makes urfave/cli flag and argument errors usage errors on
// every command, since OnUsageError does not inherit.
func markUsageErrors(c *cli.Command) {
	c.OnUsageError = onUsageError
	if len(c.Commands) > 0 && c.Action == nil {
		c.Action = groupAction
	}
	for _, sub := range c.Commands {
		markUsageErrors(sub)
	}
}
