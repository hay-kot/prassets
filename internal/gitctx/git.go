package gitctx

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// ErrDetachedHead means HEAD points at a commit, not a branch.
var ErrDetachedHead = errors.New("HEAD is detached; check out a branch or pass --branch")

func git(ctx context.Context, dir string, args ...string) (string, int, error) {
	c := exec.CommandContext(ctx, "git", append([]string{"-C", dir}, args...)...) //nolint:gosec // fixed git subcommands
	var stderr bytes.Buffer
	c.Stderr = &stderr
	out, err := c.Output()
	if err != nil {
		if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
			return "", exitErr.ExitCode(), fmt.Errorf("git %s: %s", strings.Join(args, " "), strings.TrimSpace(stderr.String()))
		}
		return "", -1, fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return strings.TrimSpace(string(out)), 0, nil
}

// TopLevel returns the root of the work tree that holds dir.
func TopLevel(ctx context.Context, dir string) (string, error) {
	out, _, err := git(ctx, dir, "rev-parse", "--show-toplevel")
	return out, err
}

// RemoteURL returns the URL of the named remote.
func RemoteURL(ctx context.Context, dir, remote string) (string, error) {
	out, _, err := git(ctx, dir, "remote", "get-url", remote)
	if err != nil {
		return "", fmt.Errorf("no git remote %q; pass --repo host/owner/name or --remote: %w", remote, err)
	}
	return out, nil
}

// CurrentBranch returns the checked-out branch name, not slugified.
func CurrentBranch(ctx context.Context, dir string) (string, error) {
	out, code, err := git(ctx, dir, "symbolic-ref", "--short", "-q", "HEAD")
	if code == 1 {
		return "", ErrDetachedHead
	}
	if err != nil {
		return "", err
	}
	return out, nil
}
