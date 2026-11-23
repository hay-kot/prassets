package secrets

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const secretValue = "s3cr3t-value-do-not-print"

func TestRenderFunctions(t *testing.T) {
	secretFile := filepath.Join(t.TempDir(), "secret.txt")
	require.NoError(t, os.WriteFile(secretFile, []byte("  from-file\n"), 0o600))

	t.Setenv("PRASSETS_TEST_SECRET", "from-env")
	t.Setenv("PRASSETS_TEST_EMPTY", "")

	r := NewResolver()
	tests := []struct {
		name    string
		text    string
		want    string
		wantErr string
	}{
		{name: "literal", text: "minioadmin", want: "minioadmin"},
		{name: "env", text: `{{ env "PRASSETS_TEST_SECRET" }}`, want: "from-env"},
		{name: "env unset", text: `{{ env "PRASSETS_TEST_UNSET_XYZ" }}`, wantErr: "env var PRASSETS_TEST_UNSET_XYZ is unset or empty"},
		{name: "env empty", text: `{{ env "PRASSETS_TEST_EMPTY" }}`, wantErr: "unset or empty"},
		{name: "cmd", text: `{{ cmd "printf '  hi\\n'" }}`, want: "hi"},
		{name: "cmd fails", text: `{{ cmd "echo boom >&2; exit 3" }}`, wantErr: "boom"},
		{name: "file", text: `{{ file "` + secretFile + `" }}`, want: "from-file"},
		{name: "file missing", text: `{{ file "/nonexistent/prassets" }}`, wantErr: "/nonexistent/prassets"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpl, err := Parse(tt.text)
			require.NoError(t, err)
			got, err := r.Render(context.Background(), tmpl)
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestVaultIsNotAFunction(t *testing.T) {
	_, err := Parse(`{{ vault "a" "b" }}`)
	require.Error(t, err)
}

func TestRenderErrorHidesTemplateText(t *testing.T) {
	r := NewResolver()
	tmpl, err := Parse(`{{ "` + secretValue + `" }}{{ env "PRASSETS_TEST_UNSET_XYZ" }}`)
	require.NoError(t, err)
	_, err = r.Render(context.Background(), tmpl)
	require.Error(t, err)
	assert.NotContains(t, err.Error(), secretValue)

	_, err = Parse(`{{ "` + secretValue + `" `)
	require.Error(t, err)
	assert.NotContains(t, err.Error(), secretValue)
}

func TestTemplateSources(t *testing.T) {
	tmpl, err := Parse(`{{ file "a" }}-{{ env "B" | printf "%s" }}{{ if true }}{{ cmd "c" }}{{ end }}`)
	require.NoError(t, err)
	assert.Equal(t, []string{"cmd", "env", "file", "printf"}, tmpl.Sources())
	assert.False(t, tmpl.IsLiteral())

	lit, err := Parse("plain")
	require.NoError(t, err)
	assert.True(t, lit.IsLiteral())
	assert.Empty(t, lit.Sources())
}
