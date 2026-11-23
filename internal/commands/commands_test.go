package commands

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testConfig = `
stores:
  s:
    endpoint: s3.example.com
    bucket: b
    public_url: https://assets.example.com/
    access_key: AKIA-LITERAL-SECRET
    secret_key: '{{ env "PRASSETS_TEST_SECRET" }}'
default:
  store: s
  prefix: "github/{{ .Owner }}/{{ .Repo }}/"
repos:
  github.com/acme/widgets:
    store: s
    types:
      blog:
        key: "blog/{{ .Slug }}/{{ .File }}"
        usage: blog images
`

const (
	repoFlag = "--repo"
	widgets  = "github.com/acme/widgets"
)

func runApp(t *testing.T, args ...string) (string, error) {
	t.Helper()
	p := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(p, []byte(testConfig), 0o600))
	var out bytes.Buffer
	app := NewApp(&Flags{}, BuildInfo{Version: "v1.2.3", Commit: "abc1234", Date: "today"})
	app.Writer = &out
	app.ErrWriter = &bytes.Buffer{}
	err := app.Run(context.Background(), append([]string{"pra", "--config", p}, args...))
	return out.String(), err
}

func TestConfigShowRedactsSecrets(t *testing.T) {
	t.Setenv("PRASSETS_TEST_SECRET", "env-secret-value")
	for _, format := range []string{"url", "json"} {
		out, err := runApp(t, repoFlag, widgets, "--format", format, "config", "show")
		require.NoError(t, err)
		assert.NotContains(t, out, "AKIA-LITERAL-SECRET")
		assert.NotContains(t, out, "env-secret-value")
		assert.Contains(t, out, "redacted: literal")
		assert.Contains(t, out, "redacted: from env")
		assert.Contains(t, out, "blog")
	}
}

func TestTypes(t *testing.T) {
	out, err := runApp(t, repoFlag, widgets, "types")
	require.NoError(t, err)
	assert.Contains(t, out, "pr/{{ .Branch }}/{{ .File }}")
	assert.Contains(t, out, "blog/{{ .Slug }}/{{ .File }}")
	assert.Contains(t, out, "config repos.github.com/acme/widgets")

	out, err = runApp(t, repoFlag, "github.com/someone/thing", "types")
	require.NoError(t, err)
	assert.Contains(t, out, "github/someone/thing/pr/{{ .Branch }}/{{ .File }}")
}

func TestUsageErrors(t *testing.T) {
	tests := [][]string{
		{"bogus"},
		{"config", "bogus"},
		{"--format", "xml", "types"},
		{"put"},
		{repoFlag, widgets, "put", "-t", "nope", "x.png"},
		{repoFlag, widgets, "--branch", "main", "put", "-t", "blog", "x.png"},
		{repoFlag, widgets, "--branch", "main", "put", "-s", "x", "x.png"},
		{repoFlag, widgets, "put", "--as", "a.png", "x.png", "y.png"},
		{"ls", "--all", "-t", "pr"},
		{"put", "--no-such-flag"},
		{repoFlag, "widgets", "types"},
	}
	for _, args := range tests {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			_, err := runApp(t, args...)
			require.Error(t, err)
			assert.True(t, IsUsageError(err), "want a usage error, got %v", err)
		})
	}
}

func TestPutChecksBeforeUpload(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "A.png")
	b := filepath.Join(dir, "a.PNG")
	unknown := filepath.Join(dir, "notes.qqq")
	for _, f := range []string{a, b, unknown} {
		require.NoError(t, os.WriteFile(f, []byte("x"), 0o600))
	}
	base := []string{repoFlag, widgets, "--branch", "Feat/X", "put"}

	_, err := runApp(t, append(base, a, b)...)
	require.ErrorContains(t, err, "both upload as a.png")

	_, err = runApp(t, append(base, unknown)...)
	require.ErrorContains(t, err, "unknown file extension")

	_, err = runApp(t, append(base, "--as", "Bad Name.png", a)...)
	require.ErrorContains(t, err, "invalid --as name")

	// Secrets render only after every local check passes.
	_, err = runApp(t, append(base, a)...)
	require.ErrorContains(t, err, "env var PRASSETS_TEST_SECRET is unset")
}

func TestVersion(t *testing.T) {
	out, err := runApp(t, "version")
	require.NoError(t, err)
	assert.Equal(t, "pra v1.2.3 (abc1234) today\n", out)
}

func TestOutputHelpers(t *testing.T) {
	assert.Equal(t, "https://a.example.com/pr/x/a%20b.png", publicURL("https://a.example.com/", "pr/x/a b.png"))
	assert.Equal(t, "https://a.example.com/type=pr/a.png", publicURL("https://a.example.com", "type=pr/a.png"))

	assert.Equal(t, "![view](u)", markdown(Result{Key: "pr/x/view.png", URL: "u", Size: 0, ContentType: "image/png"}))
	assert.Equal(t, "[doc.pdf](u)", markdown(Result{Key: "pr/x/doc.pdf", URL: "u", Size: 0, ContentType: "application/pdf"}))

	assert.Equal(t, "image/png", contentType("a.PNG"))
	assert.Empty(t, contentType("a.qqq"))
	assert.Empty(t, contentType("noext"))

	assert.Equal(t, "view-2.png", suggestName("view.png"))
	assert.Equal(t, "view-2", suggestName("view"))
}

func TestRepoPrefix(t *testing.T) {
	p, err := repoPrefix("github/a/b/", "pr/")
	require.NoError(t, err)
	assert.Equal(t, "github/a/b/pr/", p)

	p, err = repoPrefix("github/a/b/", "github/a/b/pr/")
	require.NoError(t, err)
	assert.Equal(t, "github/a/b/pr/", p)

	_, err = repoPrefix("github/a/b/", "../c/")
	require.Error(t, err)
}

func TestRedact(t *testing.T) {
	assert.Equal(t, "<redacted: literal>", redact("plain-secret"))
	assert.Equal(t, "<redacted: from cmd, vault>", redact(`{{ vault "a" "b" }}{{ cmd "x" }}`))
	assert.Empty(t, redact(""))
}
