package secrets

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"filippo.io/age"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const secretValue = "s3cr3t-value-do-not-print"

type ageFixture struct {
	identityPath string
	recipient    string
}

func newIdentity(t *testing.T) ageFixture {
	t.Helper()
	id, err := age.GenerateX25519Identity()
	require.NoError(t, err)
	p := filepath.Join(t.TempDir(), "keys.txt")
	require.NoError(t, os.WriteFile(p, []byte("# test identity\n"+id.String()+"\n"), 0o600))
	return ageFixture{identityPath: p, recipient: id.Recipient().String()}
}

func writeAgeVault(t *testing.T, fx ageFixture, plain string, armored bool) string {
	t.Helper()
	r, err := age.ParseX25519Recipient(fx.recipient)
	require.NoError(t, err)
	var buf bytes.Buffer
	require.NoError(t, writeEncrypted(&buf, []byte(plain), armored, []age.Recipient{r}))
	p := filepath.Join(t.TempDir(), "secrets.age.yaml")
	require.NoError(t, os.WriteFile(p, buf.Bytes(), 0o600))
	return p
}

const vaultDoc = "r2:\n  access_key: AKIA123\n  secret_key: " + secretValue + "\n  port: 9000\nflat: x\n"

func TestRenderFunctions(t *testing.T) {
	fx := newIdentity(t)
	vaultPath := writeAgeVault(t, fx, vaultDoc, true)
	secretFile := filepath.Join(t.TempDir(), "secret.txt")
	require.NoError(t, os.WriteFile(secretFile, []byte("  from-file\n"), 0o600))

	t.Setenv("PRASSETS_TEST_SECRET", "from-env")
	t.Setenv("PRASSETS_TEST_EMPTY", "")

	r := NewResolver(map[string]VaultSpec{
		"personal": {Path: vaultPath, Format: FormatAge, Identity: fx.identityPath},
	})

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
		{name: "vault", text: `{{ vault "personal" "r2.access_key" }}`, want: "AKIA123"},
		{name: "vault missing key", text: `{{ vault "personal" "r2.nope" }}`, wantErr: `key "r2.nope" not found`},
		{name: "vault non-string", text: `{{ vault "personal" "r2.port" }}`, wantErr: "is not a string"},
		{name: "vault path through leaf", text: `{{ vault "personal" "flat.x" }}`, wantErr: "not found"},
		{name: "vault missing", text: `{{ vault "nope" "a" }}`, wantErr: `vault "nope" is not defined`},
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
				assert.NotContains(t, err.Error(), secretValue)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestRenderErrorHidesTemplateText(t *testing.T) {
	r := NewResolver(nil)
	tmpl, err := Parse(`{{ "` + secretValue + `" }}{{ env "PRASSETS_TEST_UNSET_XYZ" }}`)
	require.NoError(t, err)
	_, err = r.Render(context.Background(), tmpl)
	require.Error(t, err)
	assert.NotContains(t, err.Error(), secretValue)

	_, err = Parse(`{{ "` + secretValue + `" `)
	require.Error(t, err)
	assert.NotContains(t, err.Error(), secretValue)
}

func TestVaultDecryptsOnce(t *testing.T) {
	fx := newIdentity(t)
	vaultPath := writeAgeVault(t, fx, vaultDoc, false)
	r := NewResolver(map[string]VaultSpec{"v": {Path: vaultPath, Format: FormatAge, Identity: fx.identityPath}})

	tmpl, err := Parse(`{{ vault "v" "r2.access_key" }}`)
	require.NoError(t, err)
	_, err = r.Render(context.Background(), tmpl)
	require.NoError(t, err)

	require.NoError(t, os.Remove(vaultPath))
	got, err := r.Render(context.Background(), tmpl)
	require.NoError(t, err)
	assert.Equal(t, "AKIA123", got)
}

func TestTemplateIntrospection(t *testing.T) {
	tmpl, err := Parse(`{{ vault "a" "x" }}-{{ env "B" | printf "%s" }}{{ if true }}{{ vault "c" "y" }}{{ end }}`)
	require.NoError(t, err)
	assert.Equal(t, []string{"env", "printf", "vault"}, tmpl.Sources())
	assert.Equal(t, []string{"a", "c"}, tmpl.VaultRefs())
	assert.False(t, tmpl.IsLiteral())

	lit, err := Parse("plain")
	require.NoError(t, err)
	assert.True(t, lit.IsLiteral())
	assert.Empty(t, lit.Sources())
}

func TestKeys(t *testing.T) {
	data, err := ParseDocument([]byte(vaultDoc))
	require.NoError(t, err)
	assert.Equal(t, []string{"flat", "r2.access_key", "r2.port", "r2.secret_key"}, Keys(data))

	empty, err := ParseDocument([]byte("# nothing yet\n"))
	require.NoError(t, err)
	assert.Empty(t, Keys(empty))

	_, err = ParseDocument([]byte("- a\n- b\n"))
	require.Error(t, err)
}

func TestEncryptAgeRoundTrip(t *testing.T) {
	for _, armored := range []bool{true, false} {
		fx := newIdentity(t)
		spec := VaultSpec{Path: filepath.Join(t.TempDir(), "sub", "v.age.yaml"), Format: FormatAge, Identity: fx.identityPath}
		require.NoError(t, EncryptAge(spec, []byte(vaultDoc), armored))

		info, err := os.Stat(spec.Path)
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())

		gotArmored, err := IsArmored(spec)
		require.NoError(t, err)
		assert.Equal(t, armored, gotArmored)

		plain, err := Decrypt(context.Background(), spec)
		require.NoError(t, err)
		assert.Equal(t, vaultDoc, string(plain))
	}
}

func TestDecryptWrongIdentity(t *testing.T) {
	fx := newIdentity(t)
	other := newIdentity(t)
	p := writeAgeVault(t, fx, vaultDoc, false)
	_, err := Decrypt(context.Background(), VaultSpec{Path: p, Format: FormatAge, Identity: other.identityPath})
	require.ErrorContains(t, err, other.identityPath)
}

func TestMissingIdentity(t *testing.T) {
	_, err := Decrypt(context.Background(), VaultSpec{Path: "x", Format: FormatAge, Identity: "/nonexistent/keys.txt"})
	require.ErrorContains(t, err, "age-keygen")
}

func TestIdentityPathOrder(t *testing.T) {
	t.Setenv("PRASSETS_AGE_IDENTITY", "/a")
	t.Setenv("SOPS_AGE_KEY_FILE", "/b")
	p, err := VaultSpec{Identity: "/explicit"}.IdentityPath()
	require.NoError(t, err)
	assert.Equal(t, "/explicit", p)

	p, err = VaultSpec{}.IdentityPath()
	require.NoError(t, err)
	assert.Equal(t, "/a", p)

	t.Setenv("PRASSETS_AGE_IDENTITY", "")
	p, err = VaultSpec{}.IdentityPath()
	require.NoError(t, err)
	assert.Equal(t, "/b", p)

	t.Setenv("SOPS_AGE_KEY_FILE", "")
	p, err = VaultSpec{}.IdentityPath()
	require.NoError(t, err)
	assert.True(t, strings.HasSuffix(p, filepath.Join(".config", "sops", "age", "keys.txt")))
}

func TestSopsVault(t *testing.T) {
	if err := exec.CommandContext(context.Background(), "sops", "--version").Run(); err != nil {
		t.Skipf("sops is not usable: %v", err)
	}
	fx := newIdentity(t)
	dir := t.TempDir()
	plainPath := filepath.Join(dir, "plain.yaml")
	require.NoError(t, os.WriteFile(plainPath, []byte(vaultDoc), 0o600))

	c := exec.CommandContext(context.Background(), "sops", "encrypt", "--age", fx.recipient, "--input-type", "yaml", "--output-type", "yaml", plainPath) //nolint:gosec // test-generated arguments
	c.Env = append(os.Environ(), "SOPS_AGE_KEY_FILE="+fx.identityPath)
	enc, err := c.Output()
	require.NoError(t, err)
	encPath := filepath.Join(dir, "secrets.sops.yaml")
	require.NoError(t, os.WriteFile(encPath, enc, 0o600))

	r := NewResolver(map[string]VaultSpec{"s": {Path: encPath, Format: FormatSops, Identity: fx.identityPath}})
	tmpl, err := Parse(`{{ vault "s" "r2.secret_key" }}`)
	require.NoError(t, err)
	got, err := r.Render(context.Background(), tmpl)
	require.NoError(t, err)
	assert.Equal(t, secretValue, got)
}
