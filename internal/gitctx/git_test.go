package gitctx

import (
	"context"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func gitRun(t *testing.T, dir string, args ...string) {
	t.Helper()
	c := exec.CommandContext(context.Background(), "git", append([]string{"-C", dir}, args...)...) //nolint:gosec // test fixture
	c.Env = append(c.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null")
	out, err := c.CombinedOutput()
	require.NoError(t, err, string(out))
}

func TestGit(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	gitRun(t, dir, "init", "-q", "-b", "Feat/Thing")
	gitRun(t, dir, "remote", "add", "origin", "git@github.com:Acme/Widgets.git")
	gitRun(t, dir, "-c", "user.name=t", "-c", "user.email=t@example.com", "-c", "commit.gpgsign=false", "commit", "-q", "--allow-empty", "-m", "x")

	top, err := TopLevel(ctx, dir)
	require.NoError(t, err)
	want, err := filepath.EvalSymlinks(dir)
	require.NoError(t, err)
	gotTop, err := filepath.EvalSymlinks(top)
	require.NoError(t, err)
	assert.Equal(t, want, gotTop)

	branch, err := CurrentBranch(ctx, dir)
	require.NoError(t, err)
	assert.Equal(t, "Feat/Thing", branch)

	u, err := RemoteURL(ctx, dir, "origin")
	require.NoError(t, err)
	assert.Equal(t, "git@github.com:Acme/Widgets.git", u)

	_, err = RemoteURL(ctx, dir, "upstream")
	require.ErrorContains(t, err, "--repo")

	gitRun(t, dir, "checkout", "-q", "--detach")
	_, err = CurrentBranch(ctx, dir)
	require.ErrorIs(t, err, ErrDetachedHead)

	_, err = TopLevel(ctx, t.TempDir())
	require.Error(t, err)
}
