package secrets

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"filippo.io/age"
	"filippo.io/age/armor"
	"go.yaml.in/yaml/v3"

	"github.com/hay-kot/prassets/internal/paths"
)

// Vault formats.
const (
	FormatAge  = "age"
	FormatSops = "sops"
)

// VaultSpec says where a vault lives and how to decrypt it.
type VaultSpec struct {
	Path     string
	Format   string
	Identity string
}

// IdentityPath returns the age identity file for the vault: its identity
// field, then $PRASSETS_AGE_IDENTITY, then $SOPS_AGE_KEY_FILE, then
// ~/.config/sops/age/keys.txt.
func (s VaultSpec) IdentityPath() (string, error) {
	for _, p := range []string{s.Identity, os.Getenv("PRASSETS_AGE_IDENTITY"), os.Getenv("SOPS_AGE_KEY_FILE")} {
		if p != "" {
			return paths.Expand(p)
		}
	}
	return paths.Expand("~/.config/sops/age/keys.txt")
}

// FilePath returns the vault path with "~" expanded.
func (s VaultSpec) FilePath() (string, error) {
	return paths.Expand(s.Path)
}

// Vault is a decrypted vault: nested maps with string leaves.
type Vault struct {
	name string
	path string
	data map[string]any
}

// LoadVault decrypts and parses a vault file.
func LoadVault(ctx context.Context, name string, spec VaultSpec) (*Vault, error) {
	plain, err := Decrypt(ctx, spec)
	if err != nil {
		return nil, fmt.Errorf("vault %q: %w", name, err)
	}
	data, err := ParseDocument(plain)
	if err != nil {
		return nil, fmt.Errorf("vault %q (%s): %w", name, spec.Path, err)
	}
	return &Vault{name: name, path: spec.Path, data: data}, nil
}

// ParseDocument parses a plaintext vault. An empty document is an empty vault.
func ParseDocument(plain []byte) (map[string]any, error) {
	var data map[string]any
	if err := yaml.Unmarshal(plain, &data); err != nil {
		// yaml errors quote only line numbers and types, never values.
		return nil, fmt.Errorf("invalid YAML: %w", err)
	}
	if data == nil {
		data = map[string]any{}
	}
	return data, nil
}

// Lookup returns the string at a dotted path such as "r2.access_key".
func (v *Vault) Lookup(path string) (string, error) {
	var cur any = v.data
	for part := range strings.SplitSeq(path, ".") {
		m, ok := cur.(map[string]any)
		if !ok {
			return "", fmt.Errorf("vault %q (%s): key %q not found", v.name, v.path, path)
		}
		if cur, ok = m[part]; !ok {
			return "", fmt.Errorf("vault %q (%s): key %q not found", v.name, v.path, path)
		}
	}
	s, ok := cur.(string)
	if !ok {
		return "", fmt.Errorf("vault %q (%s): key %q is not a string (quote the value)", v.name, v.path, path)
	}
	return s, nil
}

// Keys returns the dotted path of every leaf, sorted.
func (v *Vault) Keys() []string {
	return Keys(v.data)
}

// Keys returns the dotted path of every leaf in a parsed vault, sorted.
func Keys(data map[string]any) []string {
	var out []string
	var walk func(prefix string, m map[string]any)
	walk = func(prefix string, m map[string]any) {
		for k, val := range m {
			p := prefix + k
			if sub, ok := val.(map[string]any); ok {
				walk(p+".", sub)
				continue
			}
			out = append(out, p)
		}
	}
	walk("", data)
	slices.Sort(out)
	return out
}

// Decrypt returns the plaintext of a vault file.
func Decrypt(ctx context.Context, spec VaultSpec) ([]byte, error) {
	switch spec.Format {
	case FormatAge:
		return decryptAge(spec)
	case FormatSops:
		return decryptSops(ctx, spec)
	default:
		return nil, fmt.Errorf("unknown vault format %q (one of: age, sops)", spec.Format)
	}
}

func readIdentities(spec VaultSpec) ([]age.Identity, string, error) {
	idPath, err := spec.IdentityPath()
	if err != nil {
		return nil, "", err
	}
	f, err := os.Open(idPath) //nolint:gosec // identity path comes from the user's config or env
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, idPath, fmt.Errorf("age identity %s does not exist; create one with age-keygen -o %s", idPath, idPath)
		}
		return nil, idPath, fmt.Errorf("open age identity %s: %w", idPath, err)
	}
	defer func() { _ = f.Close() }()
	ids, err := age.ParseIdentities(f)
	if err != nil {
		return nil, idPath, fmt.Errorf("parse age identity %s: %w", idPath, err)
	}
	return ids, idPath, nil
}

const armorHeader = "-----BEGIN AGE ENCRYPTED FILE-----"

func decryptAge(spec VaultSpec) ([]byte, error) {
	ids, idPath, err := readIdentities(spec)
	if err != nil {
		return nil, err
	}
	path, err := spec.FilePath()
	if err != nil {
		return nil, err
	}
	f, err := os.Open(path) //nolint:gosec // vault path comes from the user's config
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", spec.Path, err)
	}
	defer func() { _ = f.Close() }()

	br := bufio.NewReader(f)
	var src io.Reader = br
	if head, _ := br.Peek(len(armorHeader)); string(head) == armorHeader {
		src = armor.NewReader(br)
	}
	r, err := age.Decrypt(src, ids...)
	if err != nil {
		return nil, fmt.Errorf("decrypt %s with identity %s: %w", spec.Path, idPath, err)
	}
	plain, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("decrypt %s: %w", spec.Path, err)
	}
	return plain, nil
}

// sops is a large dependency tree (every cloud KMS client), so shell out to
// the sops binary instead of importing github.com/getsops/sops/v3/decrypt.
func decryptSops(ctx context.Context, spec VaultSpec) ([]byte, error) {
	path, err := spec.FilePath()
	if err != nil {
		return nil, err
	}
	if _, err := exec.LookPath("sops"); err != nil {
		return nil, errors.New("sops vaults need the sops binary on PATH")
	}
	c := exec.CommandContext(ctx, "sops", "decrypt", "--input-type", "yaml", "--output-type", "yaml", path) //nolint:gosec // the path comes from the user's config
	c.Env = os.Environ()
	// sops on macOS looks in ~/Library/Application Support by default, so
	// pass the identity this tool resolved.
	if idPath, err := spec.IdentityPath(); err == nil {
		if _, statErr := os.Stat(idPath); statErr == nil {
			c.Env = append(c.Env, "SOPS_AGE_KEY_FILE="+idPath)
		}
	}
	var stderr bytes.Buffer
	c.Stderr = &stderr
	out, err := c.Output()
	if err != nil {
		return nil, fmt.Errorf("sops decrypt %s: %w: %s", spec.Path, err, strings.TrimSpace(stderr.String()))
	}
	return out, nil
}

// IsArmored reports whether the vault file uses age's ASCII armor.
func IsArmored(spec VaultSpec) (bool, error) {
	path, err := spec.FilePath()
	if err != nil {
		return false, err
	}
	f, err := os.Open(path) //nolint:gosec // vault path comes from the user's config
	if err != nil {
		return false, err
	}
	defer func() { _ = f.Close() }()
	head := make([]byte, len(armorHeader))
	n, _ := io.ReadFull(f, head)
	return string(head[:n]) == armorHeader, nil
}

// EncryptAge encrypts plain to the recipients of the vault's identity file and
// replaces the vault file atomically with mode 0600.
func EncryptAge(spec VaultSpec, plain []byte, armored bool) error {
	ids, idPath, err := readIdentities(spec)
	if err != nil {
		return err
	}
	var recipients []age.Recipient
	for _, id := range ids {
		switch id := id.(type) {
		case *age.X25519Identity:
			recipients = append(recipients, id.Recipient())
		case *age.HybridIdentity:
			recipients = append(recipients, id.Recipient())
		default:
		}
	}
	if len(recipients) == 0 {
		return fmt.Errorf("age identity %s has no X25519 or hybrid identity to encrypt to", idPath)
	}

	path, err := spec.FilePath()
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".prassets-vault-*")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(tmp.Name()) }()

	if err := writeEncrypted(tmp, plain, armored, recipients); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("encrypt %s: %w", spec.Path, err)
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp.Name(), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

func writeEncrypted(dst io.Writer, plain []byte, armored bool, recipients []age.Recipient) error {
	out := dst
	var aw io.WriteCloser
	if armored {
		aw = armor.NewWriter(dst)
		out = aw
	}
	w, err := age.Encrypt(out, recipients...)
	if err != nil {
		return err
	}
	if _, err := w.Write(plain); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	if aw != nil {
		return aw.Close()
	}
	return nil
}
