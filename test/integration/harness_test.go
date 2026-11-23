//go:build integration

package integration

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

const commandTimeout = 60 * time.Second

// Harness runs the built pra binary with isolated per-test state.
type Harness struct {
	t         *testing.T
	homeDir   string
	configDir string
	// Dir is the working directory for commands.
	Dir string
	// Env adds environment variables, as KEY=value.
	Env []string
}

// NewHarness creates a harness with isolated HOME and XDG directories.
func NewHarness(t *testing.T) *Harness {
	t.Helper()
	return &Harness{t: t, homeDir: t.TempDir(), configDir: t.TempDir(), Dir: t.TempDir()}
}

// Result is the outcome of one command.
type Result struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

// Run executes pra and returns stdout, stderr and the exit code.
func (h *Harness) Run(args ...string) Result {
	h.t.Helper()
	return h.RunWithStdin("", args...)
}

// RunWithStdin executes pra with stdin.
func (h *Harness) RunWithStdin(stdin string, args ...string) Result {
	h.t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), commandTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, cliBin, args...)
	cmd.Dir = h.Dir
	cmd.Stdin = strings.NewReader(stdin)
	cmd.Env = append([]string{
		"PATH=" + os.Getenv("PATH"),
		"TMPDIR=" + os.Getenv("TMPDIR"),
		"HOME=" + h.homeDir,
		"XDG_CONFIG_HOME=" + h.configDir,
		"GIT_CONFIG_GLOBAL=/dev/null",
		"GIT_CONFIG_SYSTEM=/dev/null",
		"NO_COLOR=1",
	}, h.Env...)
	var stdout, stderr strings.Builder
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	code := 0
	if err != nil {
		exitErr, ok := errors.AsType[*exec.ExitError](err)
		if !ok {
			h.t.Fatalf("run pra %v: %v", args, err)
		}
		code = exitErr.ExitCode()
	}
	return Result{Stdout: stdout.String(), Stderr: stderr.String(), ExitCode: code}
}

// Git runs git in the harness directory.
func (h *Harness) Git(args ...string) {
	h.t.Helper()
	cmd := exec.CommandContext(h.t.Context(), "git", args...)
	cmd.Dir = h.Dir
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null")
	if out, err := cmd.CombinedOutput(); err != nil {
		h.t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}
