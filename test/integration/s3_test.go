//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hay-kot/prassets/internal/devstack"
)

type result struct {
	Key         string `json:"key"`
	URL         string `json:"url"`
	Size        int64  `json:"size"`
	ContentType string `json:"content_type"`
}

// newS3Repo returns a harness in a git checkout whose origin is a repo id
// unique to the test, so tests share the bucket without seeing each other.
func newS3Repo(t *testing.T, branch string) (*Harness, string) {
	t.Helper()
	requireS3(t)
	h := NewHarness(t)
	owner := fmt.Sprintf("it%d", time.Now().UnixNano())
	h.Git("init", "-q", "-b", branch)
	h.Git("remote", "add", "origin", "git@github.com:"+owner+"/widgets.git")

	cfg := fmt.Sprintf(`stores:
  dev:
    endpoint: %s
    bucket: %s
    public_url: http://%s/%s
    insecure: true
    access_key: '{{ env "PRA_IT_ACCESS_KEY" }}'
    secret_key: '{{ env "PRA_IT_SECRET_KEY" }}'
default:
  store: dev
  prefix: "github/{{ .Owner }}/{{ .Repo }}/"
`, s3Endpoint, devstack.Bucket, s3Endpoint, devstack.Bucket)
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(cfgPath, []byte(cfg), 0o600))
	h.Env = append(h.Env,
		"PRASSETS_CONFIG="+cfgPath,
		"PRA_IT_ACCESS_KEY="+devstack.AccessKey,
		"PRA_IT_SECRET_KEY="+devstack.SecretKey,
	)
	return h, "github/" + owner + "/widgets/"
}

func writeFile(t *testing.T, h *Harness, name, content string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(h.Dir, name), []byte(content), 0o600))
}

func ok(t *testing.T, r Result) string {
	t.Helper()
	require.Equal(t, 0, r.ExitCode, "stdout:\n%s\nstderr:\n%s", r.Stdout, r.Stderr)
	return r.Stdout
}

func listJSON(t *testing.T, h *Harness, args ...string) []result {
	t.Helper()
	out := ok(t, h.Run(append([]string{"--format", "json", "ls"}, args...)...))
	var got []result
	require.NoError(t, json.Unmarshal([]byte(out), &got), out)
	return got
}

func TestS3PutListGetRemove(t *testing.T) {
	h, prefix := newS3Repo(t, "Feat/Thing")
	writeFile(t, h, "Screen Shot 1.PNG", "first")
	writeFile(t, h, "b.png", "bee")

	key := prefix + "pr/feat-thing/screen-shot-1.png"

	// put normalizes the name, prints the URL and logs the rename to stderr.
	r := h.Run("put", "Screen Shot 1.PNG")
	out := ok(t, r)
	wantURL := "http://" + s3Endpoint + "/" + devstack.Bucket + "/" + key
	assert.Equal(t, wantURL+"\n", out)
	assert.Contains(t, r.Stderr, "renamed")

	// The public URL serves the object with the immutable cache header.
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, wantURL, nil)
	require.NoError(t, err)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	_ = resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "public, max-age=31536000, immutable", resp.Header.Get("Cache-Control"))
	assert.Equal(t, "image/png", resp.Header.Get("Content-Type"))

	// A batch with one existing key uploads nothing.
	r = h.Run("put", "b.png", "Screen Shot 1.PNG")
	assert.Equal(t, 1, r.ExitCode)
	assert.Contains(t, r.Stderr, key+" already exists")
	assert.Contains(t, r.Stderr, "--force")
	assert.Len(t, listJSON(t, h), 1, "b.png must not be uploaded when the batch is refused")

	// --force replaces the object and warns about caches.
	writeFile(t, h, "Screen Shot 1.PNG", "second!")
	r = h.Run("put", "--force", "Screen Shot 1.PNG", "b.png")
	ok(t, r)
	assert.Contains(t, r.Stderr, "old copy")

	got := listJSON(t, h)
	require.Len(t, got, 2)
	assert.Equal(t, prefix+"pr/feat-thing/b.png", got[0].Key)
	assert.Equal(t, key, got[1].Key)
	assert.Equal(t, int64(len("second!")), got[1].Size)
	assert.Equal(t, "image/png", got[1].ContentType)

	// ls prints size and URL; md prints image Markdown.
	assert.Contains(t, ok(t, h.Run("ls")), "7  "+wantURL)
	assert.Contains(t, ok(t, h.Run("md")), "![screen-shot-1]("+wantURL+")")
	assert.Len(t, listJSON(t, h, "--all"), 2)
	assert.Len(t, listJSON(t, h, "pr/"), 2)

	// Another branch sees only its own PR area.
	assert.Empty(t, listJSON(t, h, "--branch", "other"))

	// get downloads by bare file name and refuses to overwrite a local file.
	dl := t.TempDir()
	out = ok(t, h.Run("get", "-o", dl, "screen-shot-1.png"))
	assert.Equal(t, filepath.Join(dl, "screen-shot-1.png")+"\n", out)
	data, err := os.ReadFile(filepath.Join(dl, "screen-shot-1.png"))
	require.NoError(t, err)
	assert.Equal(t, "second!", string(data))
	r = h.Run("get", "-o", dl, key)
	assert.Equal(t, 1, r.ExitCode)
	assert.Contains(t, r.Stderr, "already exists")

	// rm refuses keys outside the repo's types.
	r = h.Run("rm", "--yes", prefix+"other/b.png")
	assert.Equal(t, 1, r.ExitCode)
	assert.Contains(t, r.Stderr, "read-only")
	r = h.Run("rm", "--yes", "github/someone-else/widgets/pr/feat-thing/b.png")
	assert.Equal(t, 1, r.ExitCode)

	// rm by bare name and by URL.
	assert.Equal(t, prefix+"pr/feat-thing/b.png\n", ok(t, h.Run("rm", "--yes", "b.png")))
	ok(t, h.Run("rm", "--yes", wantURL))
	assert.Empty(t, listJSON(t, h))

	r = h.Run("rm", "--yes", "b.png")
	assert.Equal(t, 1, r.ExitCode)
	assert.Contains(t, r.Stderr, "does not exist")
}

func TestS3RepoFileTypes(t *testing.T) {
	h, prefix := newS3Repo(t, "main")
	writeFile(t, h, ".prassets.yaml", "types:\n  blog:\n    key: \"blog/{{ .Slug }}/{{ .File }}\"\n    usage: blog images\n")
	writeFile(t, h, "hero.webp", "img")

	r := h.Run("put", "-t", "blog", "hero.webp")
	assert.Equal(t, 2, r.ExitCode, r.Stderr)
	assert.Contains(t, r.Stderr, "--slug")

	out := ok(t, h.Run("--format", "json", "put", "-t", "blog", "-s", "my-post", "--as", "cover.webp", "hero.webp"))
	var got []result
	require.NoError(t, json.Unmarshal([]byte(out), &got), out)
	require.Len(t, got, 1)
	assert.Equal(t, prefix+"blog/my-post/cover.webp", got[0].Key)
	assert.Equal(t, "image/webp", got[0].ContentType)

	assert.Len(t, listJSON(t, h, "-t", "blog", "-s", "my-post"), 1)
	assert.Contains(t, ok(t, h.Run("types")), ".prassets.yaml")

	ok(t, h.Run("rm", "--yes", prefix+"blog/my-post/cover.webp"))
}

func TestS3ConfigCheck(t *testing.T) {
	h, _ := newS3Repo(t, "main")
	out := ok(t, h.Run("config", "check"))
	assert.Contains(t, out, "ok    bucket")
	assert.NotContains(t, out, devstack.SecretKey)

	h.Env = append(h.Env, "PRA_IT_SECRET_KEY=wrong-secret")
	r := h.Run("config", "check")
	assert.Equal(t, 1, r.ExitCode)
	assert.Contains(t, r.Stdout, "FAIL  bucket")
	assert.NotContains(t, r.Stdout+r.Stderr, "wrong-secret")

	show := ok(t, h.Run("config", "show"))
	assert.NotContains(t, show, devstack.AccessKey)
	assert.Contains(t, show, "redacted: from env")
}
