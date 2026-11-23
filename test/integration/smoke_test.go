//go:build integration

package integration

import (
	"strings"
	"testing"
)

func TestSmokeHelp(t *testing.T) {
	r := NewHarness(t).Run("--help")
	if r.ExitCode != 0 || !strings.Contains(r.Stdout, "pra") {
		t.Fatalf("help: exit %d\n%s%s", r.ExitCode, r.Stdout, r.Stderr)
	}
}

func TestSmokeVersion(t *testing.T) {
	r := NewHarness(t).Run("version")
	if r.ExitCode != 0 || !strings.HasPrefix(r.Stdout, "pra ") {
		t.Fatalf("version: exit %d\n%s%s", r.ExitCode, r.Stdout, r.Stderr)
	}
}

func TestSmokeUsageExitCode(t *testing.T) {
	for _, args := range [][]string{{"bogus"}, {"put"}, {"--format", "xml", "version"}} {
		if r := NewHarness(t).Run(args...); r.ExitCode != 2 {
			t.Fatalf("%v: want exit 2, got %d\n%s", args, r.ExitCode, r.Stderr)
		}
	}
}

func TestSmokeConfigInit(t *testing.T) {
	h := NewHarness(t)
	r := h.Run("config", "init")
	if r.ExitCode != 0 {
		t.Fatalf("config init: %s", r.Stderr)
	}
	if r := h.Run("config", "init"); r.ExitCode != 1 || !strings.Contains(r.Stderr, "already exists") {
		t.Fatalf("second config init: exit %d\n%s", r.ExitCode, r.Stderr)
	}
}
