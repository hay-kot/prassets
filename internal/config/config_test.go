package config

import (
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hay-kot/prassets/internal/gitctx"
)

var update = flag.Bool("update", false, "rewrite the committed JSON schemas")

const specConfig = `
vaults:
  personal:
    path: ~/.config/prassets/secrets.age.yaml
    format: age
    identity: ~/.config/age/keys.txt
stores:
  personal:
    endpoint: abc123.r2.cloudflarestorage.com
    bucket: my-public
    region: auto
    public_url: https://assets.example.com
    insecure: false
    access_key: '{{ vault "personal" "r2.access_key" }}'
    secret_key: '{{ vault "personal" "r2.secret_key" }}'
  work:
    endpoint: s3.us-east-1.amazonaws.com
    bucket: team-pr-assets
    region: us-east-1
    public_url: https://team-pr-assets.s3.amazonaws.com
    access_key: '{{ env "AWS_ACCESS_KEY_ID" }}'
    secret_key: '{{ cmd "op read op://Work/pr-assets/secret" }}'
default:
  store: personal
  prefix: "github/{{ .Owner }}/{{ .Repo }}/"
repos:
  github.com/hay-kot/recipinned:
    store: personal
    prefix: ""
    pr:
      key: "github/type=pr/branch={{ .Branch }}/{{ .File }}"
  GitHub.com/Acme/Widgets:
    store: work
    types:
      brand:
        key: "brand-from-global/{{ .File }}"
`

const specRepoFile = `
pr:
  key: "screenshots/{{ .Branch }}/{{ .File }}"
types:
  blog:
    key: "website/type=blog/post={{ .Slug }}/{{ .File }}"
    usage: images for one blog post; the slug is the .mdx filename
  brand:
    key: "brand/{{ .File }}"
    usage: logos shared by email, the site and the app
`

func mustParse(t *testing.T, data string) *Global {
	t.Helper()
	g, err := Parse("/cfg.yaml", []byte(data))
	require.NoError(t, err)
	return g
}

func TestParseSpecExample(t *testing.T) {
	g := mustParse(t, specConfig)
	assert.Contains(t, g.Config.Repos, "github.com/acme/widgets", "repo ids are lower cased")
}

func TestParseErrors(t *testing.T) {
	base := "stores:\n  s:\n    endpoint: e.example.com\n    bucket: b\n    public_url: https://x\n    access_key: a\n    secret_key: b\n"
	tests := []struct {
		name    string
		data    string
		wantErr string
	}{
		{name: "unknown top-level key", data: base + "log_level: debug\n", wantErr: "field log_level not found"},
		{name: "unknown store key", data: base + "    acess_key: x\n", wantErr: "field acess_key not found"},
		{name: "missing store", data: base + "default:\n  store: nope\n", wantErr: `store "nope" is not defined`},
		{name: "missing vault", data: "stores:\n  s:\n    endpoint: e\n    bucket: b\n    public_url: https://x\n    access_key: '{{ vault \"v\" \"a\" }}'\n    secret_key: b\n", wantErr: `vault "v" is not defined`},
		{name: "bad vault format", data: "vaults:\n  v:\n    path: x\n    format: gpg\n", wantErr: "format must be age or sops"},
		{name: "endpoint scheme", data: "stores:\n  s:\n    endpoint: https://e\n    bucket: b\n    public_url: https://x\n    access_key: a\n    secret_key: b\n", wantErr: "no scheme"},
		{name: "bad public url", data: "stores:\n  s:\n    endpoint: e\n    bucket: b\n    public_url: assets.example.com\n    access_key: a\n    secret_key: b\n", wantErr: "public_url"},
		{name: "missing secret", data: "stores:\n  s:\n    endpoint: e\n    bucket: b\n    public_url: https://x\n    access_key: a\n", wantErr: "secret_key is required"},
		{name: "bad secret template", data: "stores:\n  s:\n    endpoint: e\n    bucket: b\n    public_url: https://x\n    access_key: '{{ env '\n    secret_key: b\n", wantErr: "invalid secret template"},
		{name: "key template typo", data: base + "default:\n  store: s\n  pr:\n    key: '{{ .Brnach }}/{{ .File }}'\n", wantErr: "unknown field .Brnach"},
		{name: "key template env", data: base + "default:\n  store: s\n  types:\n    x:\n      key: '{{ env \"HOME\" }}/{{ .File }}'\n", wantErr: "not defined"},
		{name: "prefix with branch", data: base + "default:\n  store: s\n  prefix: '{{ .Branch }}/'\n", wantErr: "may only reference"},
		{name: "pr under types", data: base + "default:\n  store: s\n  types:\n    pr:\n      key: '{{ .File }}'\n", wantErr: "under pr, not under types"},
		{name: "bad repo id", data: base + "repos:\n  widgets:\n    store: s\n", wantErr: "want host/owner/name"},
		{name: "duplicate repo id", data: base + "repos:\n  github.com/a/b:\n    store: s\n  GitHub.com/A/B:\n    store: s\n", wantErr: "listed twice"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse("/cfg.yaml", []byte(tt.data))
			require.ErrorContains(t, err, tt.wantErr)
			assert.Contains(t, err.Error(), "/cfg.yaml")
		})
	}
}

func TestParseRepoFile(t *testing.T) {
	_, err := ParseRepoFile(".prassets.yaml", []byte(specRepoFile))
	require.NoError(t, err)

	for name, data := range map[string]string{ //nolint:gosec // test data, not credentials
		"store":  "store: personal\n",
		"prefix": "prefix: x/\n",
		"secret": "access_key: x\n",
	} {
		t.Run(name, func(t *testing.T) {
			_, err := ParseRepoFile(".prassets.yaml", []byte(data))
			require.ErrorContains(t, err, "may hold only pr and types")
		})
	}

	_, err = ParseRepoFile(".prassets.yaml", []byte("types:\n  x:\n    key: '{{ cmd \"id\" }}/{{ .File }}'\n"))
	require.ErrorContains(t, err, "not defined")
}

func TestLoadMissing(t *testing.T) {
	_, err := Load(filepath.Join(t.TempDir(), "nope.yaml"))
	require.ErrorIs(t, err, ErrNotFound)

	rf, err := LoadRepoFile(filepath.Join(t.TempDir(), RepoFileName))
	require.NoError(t, err)
	assert.Nil(t, rf)
}

func TestResolveMergeOrder(t *testing.T) {
	g := mustParse(t, specConfig)
	rf, err := ParseRepoFile(".prassets.yaml", []byte(specRepoFile))
	require.NoError(t, err)

	r, err := g.Resolve(gitctx.Repo{Host: "github.com", Owner: "acme", Name: "widgets"}, rf, "/repo/.prassets.yaml")
	require.NoError(t, err)
	assert.Equal(t, "work", r.StoreName)
	assert.Equal(t, "repos.github.com/acme/widgets", r.Entry)
	assert.Empty(t, r.Prefix)

	got := map[string][2]string{}
	names := []string{}
	for _, typ := range r.Types {
		got[typ.Name] = [2]string{typ.Template.Text(), typ.Source}
		names = append(names, typ.Name)
	}
	assert.Equal(t, []string{"pr", "blog", "brand"}, names)
	assert.Equal(t, [2]string{"screenshots/{{ .Branch }}/{{ .File }}", SourceRepoFile}, got["pr"])
	assert.Equal(t, [2]string{"website/type=blog/post={{ .Slug }}/{{ .File }}", SourceRepoFile}, got["blog"])
	assert.Equal(t, [2]string{"brand-from-global/{{ .File }}", "config repos.github.com/acme/widgets"}, got["brand"])

	pr, err := r.Type("pr")
	require.NoError(t, err)
	assert.Equal(t, builtinPRUsage, pr.Usage)
}

func TestResolveDefault(t *testing.T) {
	g := mustParse(t, specConfig)
	r, err := g.Resolve(gitctx.Repo{Host: "github.com", Owner: "someone", Name: "thing"}, nil, "")
	require.NoError(t, err)
	assert.Equal(t, "default", r.Entry)
	assert.Equal(t, "github/someone/thing/", r.Prefix)
	require.Len(t, r.Types, 1)
	assert.Equal(t, SourceBuiltin, r.Types[0].Source)

	m, err := r.Matcher()
	require.NoError(t, err)
	assert.True(t, m.Writable("github/someone/thing/pr/feat-x/a.png"))
	assert.False(t, m.Writable("github/other/thing/pr/feat-x/a.png"))

	_, err = r.Type("blog")
	require.ErrorContains(t, err, "unknown type")
}

func TestResolveRepoFileCannotEscapePrefix(t *testing.T) {
	g := mustParse(t, specConfig)
	rf, err := ParseRepoFile(".prassets.yaml", []byte("types:\n  x:\n    key: '{{ .File }}'\n"))
	require.NoError(t, err)
	r, err := g.Resolve(gitctx.Repo{Host: "github.com", Owner: "someone", Name: "thing"}, rf, "")
	require.NoError(t, err)
	m, err := r.Matcher()
	require.NoError(t, err)
	assert.True(t, m.Writable("github/someone/thing/a.png"))
	assert.False(t, m.Writable("a.png"))
}

func TestResolveNoEntry(t *testing.T) {
	g := mustParse(t, "stores:\n  s:\n    endpoint: e\n    bucket: b\n    public_url: https://x\n    access_key: a\n    secret_key: b\n")
	_, err := g.Resolve(gitctx.Repo{Host: "github.com", Owner: "x", Name: "y"}, nil, "")
	require.EqualError(t, err, "no config for repo github.com/x/y; add it under repos: in /cfg.yaml")
}

func TestStarterParses(t *testing.T) {
	_, err := Parse("starter", []byte(Starter))
	require.NoError(t, err)
}

func TestDefaultPath(t *testing.T) {
	t.Setenv("PRASSETS_CONFIG", "")
	t.Setenv("XDG_CONFIG_HOME", "/xdg")
	p, err := DefaultPath()
	require.NoError(t, err)
	assert.Equal(t, "/xdg/prassets/config.yaml", p)

	t.Setenv("PRASSETS_CONFIG", "/elsewhere.yaml")
	p, err = DefaultPath()
	require.NoError(t, err)
	assert.Equal(t, "/elsewhere.yaml", p)
}

func TestSchemaDrift(t *testing.T) {
	for file, gen := range map[string]func() ([]byte, error){
		"schema.json":      GlobalSchema,
		"schema.repo.json": RepoSchema,
	} {
		t.Run(file, func(t *testing.T) {
			want, err := gen()
			require.NoError(t, err)
			path := filepath.Join("..", "..", file)
			if *update {
				require.NoError(t, os.WriteFile(path, want, 0o600))
			}
			got, err := os.ReadFile(path) //nolint:gosec // fixed paths in the repo
			require.NoError(t, err, "run: mise run schema")
			assert.Equal(t, string(want), string(got), "%s is out of date; run: mise run schema", file)
		})
	}
}
