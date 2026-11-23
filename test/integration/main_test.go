//go:build integration

package integration

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hay-kot/prassets/internal/devstack"
)

// rustfsImage must match dev/compose.yaml.
const rustfsImage = "rustfs/rustfs:1.0.6"

var (
	cliBin string
	// s3Endpoint is empty when no S3 backend is available; S3 tests skip.
	s3Endpoint  string
	s3SkipCause string
)

func TestMain(m *testing.M) {
	os.Exit(run(m))
}

func run(m *testing.M) int {
	tmp, err := os.MkdirTemp("", "pra-integration-")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer func() { _ = os.RemoveAll(tmp) }()

	cliBin = os.Getenv("PRA_BIN")
	if cliBin == "" {
		cliBin = filepath.Join(tmp, "pra")
		build := exec.CommandContext(context.Background(), "go", "build", "-o", cliBin, "../..")
		build.Stdout, build.Stderr = os.Stderr, os.Stderr
		if err := build.Run(); err != nil {
			fmt.Fprintln(os.Stderr, "build pra:", err)
			return 1
		}
	}

	stop, err := startS3()
	if err != nil {
		s3SkipCause = err.Error()
		fmt.Fprintln(os.Stderr, "S3 tests will skip:", err)
	}
	if stop != nil {
		defer stop()
	}
	return m.Run()
}

// startS3 uses $PRASSETS_TEST_S3_ENDPOINT (for example the dev stack from
// mise run dev:up) or starts a throwaway RustFS container.
func startS3() (func(), error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	if ep := os.Getenv("PRASSETS_TEST_S3_ENDPOINT"); ep != "" {
		if err := devstack.EnsureBucket(ctx, ep, devstack.AccessKey, devstack.SecretKey, devstack.Bucket); err != nil {
			return nil, err
		}
		s3Endpoint = ep
		return func() {}, nil
	}

	if err := exec.CommandContext(ctx, "docker", "info").Run(); err != nil {
		return nil, fmt.Errorf("docker is not available (%w); set PRASSETS_TEST_S3_ENDPOINT or start Docker", err)
	}
	out, err := exec.CommandContext(ctx, "docker", "run", "-d", "--rm",
		"-p", "127.0.0.1::9000",
		"-e", "RUSTFS_ACCESS_KEY="+devstack.AccessKey,
		"-e", "RUSTFS_SECRET_KEY="+devstack.SecretKey,
		rustfsImage).Output()
	if err != nil {
		return nil, fmt.Errorf("start %s: %w", rustfsImage, err)
	}
	id := strings.TrimSpace(string(out))
	stop := func() { _ = exec.CommandContext(context.Background(), "docker", "rm", "-f", id).Run() }

	port, err := exec.CommandContext(ctx, "docker", "port", id, "9000/tcp").Output()
	if err != nil {
		stop()
		return nil, fmt.Errorf("docker port: %w", err)
	}
	ep := strings.TrimSpace(string(bytes.SplitN(port, []byte("\n"), 2)[0]))
	if err := devstack.EnsureBucket(ctx, ep, devstack.AccessKey, devstack.SecretKey, devstack.Bucket); err != nil {
		stop()
		return nil, err
	}
	s3Endpoint = ep
	return stop, nil
}

func requireS3(t *testing.T) {
	t.Helper()
	if s3Endpoint == "" {
		t.Skip(s3SkipCause)
	}
}
