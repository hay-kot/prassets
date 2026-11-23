package layout

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// recipinnedTypes is the layout of the tool this CLI ports, written as key
// templates, so its test cases carry over.
var recipinnedTypes = map[string]string{
	"brand":        "brand/{{ .File }}",
	"email":        "email/campaign={{ .Slug }}/{{ .File }}",
	"blog":         "website/type=blog/post={{ .Slug }}/{{ .File }}",
	"help":         "website/type=help/article={{ .Slug }}/{{ .File }}",
	"page":         "website/type=page/page={{ .Slug }}/{{ .File }}",
	"release-note": "app/type=release-note/note={{ .Slug }}/{{ .File }}",
	"pr":           "github/type=pr/branch={{ .Branch }}/{{ .File }}",
}

func recipinnedMatcher(t *testing.T) *Matcher {
	t.Helper()
	templates := make([]*Template, 0, len(recipinnedTypes))
	for _, text := range recipinnedTypes {
		tmpl, err := ParseKey(text)
		require.NoError(t, err)
		templates = append(templates, tmpl)
	}
	m, err := NewMatcher("", "github.com", "hay-kot", "recipinned", templates...)
	require.NoError(t, err)
	return m
}

func TestBuildKey(t *testing.T) {
	tests := []struct {
		name    string
		typ     string
		slug    string
		branch  string
		file    string
		want    string
		wantErr bool
	}{
		{name: "blog", typ: "blog", slug: "how-to-share-cookbooks", file: "hero.webp", want: "website/type=blog/post=how-to-share-cookbooks/hero.webp"},
		{name: "help", typ: "help", slug: "calendar-linking", file: "cover.webp", want: "website/type=help/article=calendar-linking/cover.webp"},
		{name: "page", typ: "page", slug: "dietitians", file: "client-cookbook.webp", want: "website/type=page/page=dietitians/client-cookbook.webp"},
		{name: "email", typ: "email", slug: "02-recipes", file: "a.jpg", want: "email/campaign=02-recipes/a.jpg"},
		{name: "release note", typ: "release-note", slug: "launch", file: "a.png", want: "app/type=release-note/note=launch/a.png"},
		{name: "pr", typ: "pr", branch: "feat-assets-pr-type", file: "recipe-view.png", want: "github/type=pr/branch=feat-assets-pr-type/recipe-view.png"},
		{name: "pr branch with slash", typ: "pr", branch: "feat/assets-pr-type", file: "recipe-view.png", wantErr: true},
		{name: "pr upper case branch", typ: "pr", branch: "Feat-assets", file: "recipe-view.png", wantErr: true},
		{name: "pr no branch", typ: "pr", file: "recipe-view.png", wantErr: true},
		{name: "pr dot-dot branch", typ: "pr", branch: "..", file: "a.png", wantErr: true},
		{name: "pr with slug", typ: "pr", branch: "feat-x", slug: "x", file: "a.png", wantErr: true},
		{name: "brand", typ: "brand", file: "logo-banner.png", want: "brand/logo-banner.png"},
		{name: "brand with slug", typ: "brand", slug: "x", file: "logo.png", wantErr: true},
		{name: "missing slug", typ: "blog", file: "hero.webp", wantErr: true},
		{name: "slash in slug", typ: "blog", slug: "a/b", file: "hero.webp", wantErr: true},
		{name: "dot-dot slug", typ: "blog", slug: "..", file: "hero.webp", wantErr: true},
		{name: "upper case file", typ: "blog", slug: "a", file: "Hero.webp", wantErr: true},
		{name: "dot file", typ: "blog", slug: "a", file: ".hero.webp", wantErr: true},
		{name: "space in file", typ: "blog", slug: "a", file: "my hero.webp", wantErr: true},
	}

	m := recipinnedMatcher(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpl, err := ParseKey(recipinnedTypes[tt.typ])
			require.NoError(t, err)

			got, err := BuildKey("", tmpl, Data{Host: "github.com", Owner: "hay-kot", Repo: "recipinned", Branch: tt.branch, Slug: tt.slug, File: tt.file})
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
			assert.True(t, m.Writable(got))
		})
	}
}

func TestWritable(t *testing.T) {
	tests := []struct {
		key  string
		want bool
	}{
		{key: "website/type=blog/post=a/hero.webp", want: true},
		{key: "brand/logo.png", want: true},
		{key: "github/type=pr/branch=feat-x/a.png", want: true},
		{key: "github/type=pr/branch=feat/x/a.png", want: false},
		{key: "email/logo-banner.png", want: false},
		{key: "email/1-launch/recipinned-hoodie-2026-10-01.jpg", want: false},
		{key: "email/", want: false},
		{key: "usda/FoodData_Central_foundation_food_json_2024-10-31.zip", want: false},
		{key: "website/type=blog/post=a/", want: false},
		{key: "website/type=blog/post=a/b/c.webp", want: false},
		{key: "brand/sub/logo.png", want: false},
		{key: "website/type=blog/post=../hero.webp", want: false},
		{key: "website/type=blog/post=a/HERO.webp", want: false},
		{key: "/brand/logo.png", want: false},
		{key: "brandXlogo.png", want: false},
	}

	m := recipinnedMatcher(t)
	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			assert.Equal(t, tt.want, m.Writable(tt.key))
		})
	}
}

func TestWritableWithPrefix(t *testing.T) {
	prefixTmpl, err := ParsePrefix("github/{{ .Owner }}/{{ .Repo }}/")
	require.NoError(t, err)
	prefix, err := RenderPrefix(prefixTmpl, "github.com", "acme", "widgets")
	require.NoError(t, err)
	require.Equal(t, "github/acme/widgets/", prefix)

	pr, err := ParseKey(BuiltinPR)
	require.NoError(t, err)
	m, err := NewMatcher(prefix, "github.com", "acme", "widgets", pr)
	require.NoError(t, err)

	assert.True(t, m.Writable("github/acme/widgets/pr/feat-x/view.png"))
	assert.False(t, m.Writable("github/acme/other/pr/feat-x/view.png"))
	assert.False(t, m.Writable("pr/feat-x/view.png"))
}

func TestPatternUsesRepoFields(t *testing.T) {
	tmpl, err := ParseKey("{{ .Owner | lower }}/{{ .Branch }}/{{ .File }}")
	require.NoError(t, err)
	m, err := NewMatcher("", "github.com", "Acme", "widgets", tmpl)
	require.NoError(t, err)
	assert.True(t, m.Writable("acme/main/a.png"))
	assert.False(t, m.Writable("other/main/a.png"))
}

func TestParseKey(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		wantErr  string
		wantSlug bool
	}{
		{name: "builtin pr", text: BuiltinPR},
		{name: "slug", text: "blog/{{ .Slug }}/{{ .File }}", wantSlug: true},
		{name: "slug via dollar", text: "blog/{{ $.Slug }}/{{ .File }}", wantSlug: true},
		{name: "slug in pipeline", text: "blog/{{ .Slug | lower }}/{{ .File }}", wantSlug: true},
		{name: "slug inside if", text: "blog/{{ if .Slug }}{{ .Slug }}{{ end }}/{{ .File }}", wantSlug: true},
		{name: "slug in text is not a field", text: "Slug/{{ .File }}"},
		{name: "slugify func", text: "{{ slugify .Owner }}/{{ .File }}"},
		{name: "no file", text: "pr/{{ .Branch }}", wantErr: "must reference .File"},
		{name: "unknown field", text: "{{ .Brnach }}/{{ .File }}", wantErr: "unknown field .Brnach"},
		{name: "env not allowed", text: "{{ env \"HOME\" }}/{{ .File }}", wantErr: "function \"env\" not defined"},
		{name: "cmd not allowed", text: "{{ cmd \"id\" }}/{{ .File }}", wantErr: "function \"cmd\" not defined"},
		{name: "vault not allowed", text: "{{ vault \"a\" \"b\" }}/{{ .File }}", wantErr: "function \"vault\" not defined"},
		{name: "file not allowed", text: "{{ file \"/etc/passwd\" }}/{{ .File }}", wantErr: "function \"file\" not defined"},
		{name: "bad syntax", text: "{{ .File", wantErr: "parse template"},
		{name: "file not last", text: "{{ .File }}/x", wantErr: "must end with"},
		{name: "bare dot", text: "{{ . }}/{{ .File }}", wantErr: "bare {{ . }}"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpl, err := ParseKey(tt.text)
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantSlug, tmpl.UsesSlug())
		})
	}
}

func TestParsePrefix(t *testing.T) {
	_, err := ParsePrefix("github/{{ .Owner }}/{{ .Repo }}/")
	require.NoError(t, err)
	_, err = ParsePrefix("")
	require.NoError(t, err)
	_, err = ParsePrefix("x/{{ .Branch }}/")
	require.ErrorContains(t, err, "may only reference")
	_, err = ParsePrefix("/abs/")
	require.ErrorContains(t, err, "must not start with '/'")
	_, err = ParsePrefix("{{ env \"X\" }}")
	require.ErrorContains(t, err, "not defined")
}

func TestBuildKeyChecksRenderedKey(t *testing.T) {
	tmpl, err := ParseKey("{{ .Owner }}/{{ .File }}")
	require.NoError(t, err)
	tests := []struct {
		name   string
		prefix string
		owner  string
	}{
		{name: "dot-dot owner", owner: ".."},
		{name: "empty owner", owner: ""},
		{name: "leading slash prefix", prefix: "/x/", owner: "a"},
		{name: "double slash prefix", prefix: "x//", owner: "a"},
		{name: "dot segment prefix", prefix: "./", owner: "a"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := BuildKey(tt.prefix, tmpl, Data{Owner: tt.owner, File: "a.png"})
			require.Error(t, err)
		})
	}
}

func TestDir(t *testing.T) {
	tmpl, err := ParseKey(BuiltinPR)
	require.NoError(t, err)
	dir, err := tmpl.Dir(Data{Branch: "feat-x"})
	require.NoError(t, err)
	assert.Equal(t, "pr/feat-x/", dir)

	flat, err := ParseKey("{{ .File }}")
	require.NoError(t, err)
	dir, err = flat.Dir(Data{})
	require.NoError(t, err)
	assert.Empty(t, dir)
}

func TestSlugify(t *testing.T) {
	tests := map[string]string{
		"hay-kot/Recipe Wake":    "hay-kot-recipe-wake",
		"feat/v1-implementation": "feat-v1-implementation",
		"--a--b--":               "a-b",
		"Main":                   "main",
		"fix/ünïcode":            "fix-n-code",
		"a_b.c":                  "a_b.c",
		"":                       "",
	}
	for in, want := range tests {
		assert.Equal(t, want, Slugify(in), in)
	}
}

func TestNormalizeFileName(t *testing.T) {
	tests := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{in: "Screenshot 2026-11-23 at 10.42.13.png", want: "screenshot-2026-11-23-at-10.42.13.png"},
		{in: "dir/Sub Dir/View.PNG", want: "view.png"},
		{in: "already-ok.webp", want: "already-ok.webp"},
		{in: "no-ext", want: "no-ext"},
		{in: ".png", wantErr: true},
		{in: "___.png", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got, err := NormalizeFileName(tt.in)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
