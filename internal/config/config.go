package config

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/hay-kot/prassets/internal/gitctx"
	"github.com/hay-kot/prassets/internal/layout"
	"github.com/hay-kot/prassets/internal/paths"
	"github.com/hay-kot/prassets/internal/secrets"
)

// RepoFileName is the repo config file at the repo root.
const RepoFileName = ".prassets.yaml"

// ErrNotFound means the global config file does not exist.
var ErrNotFound = errors.New("config file not found")

// DefaultPath returns the global config path: $PRASSETS_CONFIG, else
// $XDG_CONFIG_HOME/prassets/config.yaml, else ~/.config/prassets/config.yaml.
func DefaultPath() (string, error) {
	if p := os.Getenv("PRASSETS_CONFIG"); p != "" {
		return paths.Expand(p)
	}
	dir, err := paths.ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.yaml"), nil
}

// Global is a loaded and validated global config.
type Global struct {
	Path   string
	Config Config
}

// Load reads, strictly decodes and validates the global config at path.
func Load(path string) (*Global, error) {
	data, err := os.ReadFile(path) //nolint:gosec // the path is the user's own config
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("%w: %s; run pra config init to create one", ErrNotFound, path)
		}
		return nil, fmt.Errorf("read config: %w", err)
	}
	return Parse(path, data)
}

// Parse decodes and validates global config data. path is used in errors.
func Parse(path string, data []byte) (*Global, error) {
	var cfg Config
	if err := decodeStrict(data, &cfg); err != nil {
		return nil, fmt.Errorf("config %s: %w", path, err)
	}
	repos := make(map[string]Entry, len(cfg.Repos))
	for id, e := range cfg.Repos {
		repo, err := gitctx.ParseID(id)
		if err != nil {
			return nil, fmt.Errorf("config %s: repos: %w", path, err)
		}
		if _, dup := repos[repo.ID()]; dup {
			return nil, fmt.Errorf("config %s: repos: %s is listed twice", path, repo.ID())
		}
		repos[repo.ID()] = e
	}
	cfg.Repos = repos
	if err := validate(&cfg); err != nil {
		return nil, fmt.Errorf("config %s: %w", path, err)
	}
	return &Global{Path: path, Config: cfg}, nil
}

func decodeStrict(data []byte, out any) error {
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(out); err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	return nil
}

func validate(cfg *Config) error {
	for _, name := range sortedKeys(cfg.Vaults) {
		v := cfg.Vaults[name]
		if v.Path == "" {
			return fmt.Errorf("vaults.%s: path is required", name)
		}
		if v.Format != secrets.FormatAge && v.Format != secrets.FormatSops {
			return fmt.Errorf("vaults.%s: format must be age or sops, not %q", name, v.Format)
		}
	}
	for _, name := range sortedKeys(cfg.Stores) {
		if err := validateStore(cfg, cfg.Stores[name]); err != nil {
			return fmt.Errorf("stores.%s: %w", name, err)
		}
	}
	if cfg.Default != nil {
		if err := validateEntry(cfg, *cfg.Default); err != nil {
			return fmt.Errorf("default: %w", err)
		}
	}
	for _, id := range sortedKeys(cfg.Repos) {
		if err := validateEntry(cfg, cfg.Repos[id]); err != nil {
			return fmt.Errorf("repos.%s: %w", id, err)
		}
	}
	return nil
}

func validateStore(cfg *Config, s Store) error {
	switch {
	case s.Endpoint == "":
		return errors.New("endpoint is required")
	case strings.Contains(s.Endpoint, "://") || strings.Contains(s.Endpoint, "/"):
		return fmt.Errorf("endpoint %q must be a host with no scheme or path", s.Endpoint)
	case s.Bucket == "":
		return errors.New("bucket is required")
	case s.PublicURL == "":
		return errors.New("public_url is required")
	}
	u, err := url.Parse(s.PublicURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return fmt.Errorf("public_url %q must be an http or https URL", s.PublicURL)
	}
	for _, f := range s.SecretFields() {
		if f.Template == "" {
			if f.Name == "session_token" {
				continue
			}
			return fmt.Errorf("%s is required", f.Name)
		}
		t, err := secrets.Parse(f.Template)
		if err != nil {
			return fmt.Errorf("%s: %w", f.Name, err)
		}
		for _, ref := range t.VaultRefs() {
			if _, ok := cfg.Vaults[ref]; !ok {
				return fmt.Errorf("%s: vault %q is not defined under vaults", f.Name, ref)
			}
		}
	}
	return nil
}

// SecretField is one credential field of a store.
type SecretField struct {
	Name     string
	Template string
}

// SecretFields returns the store's secret template fields.
func (s Store) SecretFields() []SecretField {
	return []SecretField{
		{Name: "access_key", Template: s.AccessKey},
		{Name: "secret_key", Template: s.SecretKey},
		{Name: "session_token", Template: s.SessionToken},
	}
}

// RegionOrDefault returns the region, or "auto".
func (s Store) RegionOrDefault() string {
	if s.Region == "" {
		return "auto"
	}
	return s.Region
}

func validateEntry(cfg *Config, e Entry) error {
	if e.Store == "" {
		return errors.New("store is required")
	}
	if _, ok := cfg.Stores[e.Store]; !ok {
		return fmt.Errorf("store %q is not defined under stores", e.Store)
	}
	if _, err := layout.ParsePrefix(e.Prefix); err != nil {
		return fmt.Errorf("prefix: %w", err)
	}
	return validateTypes(e.PR, e.Types)
}

var typeNamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*$`)

func validateTypes(pr *TypeDef, types map[string]TypeDef) error {
	if pr != nil {
		if _, err := layout.ParseKey(pr.Key); err != nil {
			return fmt.Errorf("pr.key: %w", err)
		}
	}
	for _, name := range sortedKeys(types) {
		if name == PRType {
			return errors.New("types.pr: set the pr layout under pr, not under types")
		}
		if !typeNamePattern.MatchString(name) {
			return fmt.Errorf("types.%s: type names use lower case letters, digits, '-' and '_'", name)
		}
		if _, err := layout.ParseKey(types[name].Key); err != nil {
			return fmt.Errorf("types.%s.key: %w", name, err)
		}
	}
	return nil
}

// LoadRepoFile reads and validates a .prassets.yaml. A missing file returns
// nil and no error.
func LoadRepoFile(path string) (*RepoFile, error) {
	data, err := os.ReadFile(path) //nolint:gosec // the path is the repo root's .prassets.yaml
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil //nolint:nilnil // a missing repo file is normal
		}
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	return ParseRepoFile(path, data)
}

// ParseRepoFile decodes and validates .prassets.yaml data.
func ParseRepoFile(path string, data []byte) (*RepoFile, error) {
	var rf RepoFile
	if err := decodeStrict(data, &rf); err != nil {
		return nil, fmt.Errorf("%s: %w (it may hold only pr and types)", path, err)
	}
	if err := validateTypes(rf.PR, rf.Types); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return &rf, nil
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// PRType is the name of the built-in type put uses by default.
const PRType = "pr"

// Type sources, as config show and types print them.
const (
	SourceBuiltin  = "built-in"
	SourceRepoFile = RepoFileName
)

const builtinPRUsage = "screenshots for the current branch's pull request"

// Type is one resolved asset type.
type Type struct {
	Name     string
	Template *layout.Template
	Usage    string
	Source   string
}

// Resolved is the config for one repo after the merge.
type Resolved struct {
	ConfigPath     string
	Repo           gitctx.Repo
	Entry          string
	StoreName      string
	Store          Store
	Vaults         map[string]Vault
	PrefixTemplate string
	Prefix         string
	RepoFilePath   string
	Types          []Type
}

// Resolve picks the repo's entry and merges its types: the built-in pr type,
// then the repo file, then the global entry, each winning per type name.
// repoFile may be nil.
func (g *Global) Resolve(repo gitctx.Repo, repoFile *RepoFile, repoFilePath string) (*Resolved, error) {
	entry, entryName := g.Config.Repos[repo.ID()], "repos."+repo.ID()
	if _, ok := g.Config.Repos[repo.ID()]; !ok {
		if g.Config.Default == nil {
			return nil, fmt.Errorf("no config for repo %s; add it under repos: in %s", repo.ID(), g.Path)
		}
		entry, entryName = *g.Config.Default, "default"
	}

	prefixTmpl, err := layout.ParsePrefix(entry.Prefix)
	if err != nil {
		return nil, err
	}
	prefix, err := layout.RenderPrefix(prefixTmpl, repo.Host, repo.Owner, repo.Name)
	if err != nil {
		return nil, err
	}
	if prefix != "" {
		if err := layout.CheckKey(prefix + "x"); err != nil {
			return nil, fmt.Errorf("%s: prefix renders %q for %s: %w", entryName, prefix, repo.ID(), err)
		}
	}

	merged := map[string]Type{}
	pr, err := layout.ParseKey(layout.BuiltinPR)
	if err != nil {
		return nil, err
	}
	merged[PRType] = Type{Name: PRType, Template: pr, Usage: builtinPRUsage, Source: SourceBuiltin}

	layers := []struct {
		source string
		pr     *TypeDef
		types  map[string]TypeDef
	}{
		{source: SourceRepoFile, pr: nil, types: nil},
		{source: "config " + entryName, pr: entry.PR, types: entry.Types},
	}
	if repoFile != nil {
		layers[0].pr, layers[0].types = repoFile.PR, repoFile.Types
	}
	for _, l := range layers {
		defs := maps.Clone(l.types)
		if defs == nil {
			defs = map[string]TypeDef{}
		}
		if l.pr != nil {
			defs[PRType] = *l.pr
		}
		for name, def := range defs {
			t, err := layout.ParseKey(def.Key)
			if err != nil {
				return nil, fmt.Errorf("%s: type %s: %w", l.source, name, err)
			}
			usage := def.Usage
			if usage == "" && name == PRType {
				usage = builtinPRUsage
			}
			merged[name] = Type{Name: name, Template: t, Usage: usage, Source: l.source}
		}
	}

	types := make([]Type, 0, len(merged))
	for _, name := range sortedKeys(merged) {
		types = append(types, merged[name])
	}
	slices.SortStableFunc(types, func(a, b Type) int {
		switch {
		case a.Name == PRType:
			return -1
		case b.Name == PRType:
			return 1
		default:
			return 0
		}
	})

	return &Resolved{
		ConfigPath:     g.Path,
		Repo:           repo,
		Entry:          entryName,
		StoreName:      entry.Store,
		Store:          g.Config.Stores[entry.Store],
		Vaults:         g.Config.Vaults,
		PrefixTemplate: entry.Prefix,
		Prefix:         prefix,
		RepoFilePath:   repoFilePath,
		Types:          types,
	}, nil
}

// Type returns the named type.
func (r *Resolved) Type(name string) (Type, error) {
	names := make([]string, 0, len(r.Types))
	for _, t := range r.Types {
		if t.Name == name {
			return t, nil
		}
		names = append(names, t.Name)
	}
	return Type{}, fmt.Errorf("unknown type %q for %s (one of: %s)", name, r.Repo.ID(), strings.Join(names, ", "))
}

// Matcher returns the writable-key matcher for every type of the repo.
func (r *Resolved) Matcher() (*layout.Matcher, error) {
	templates := make([]*layout.Template, len(r.Types))
	for i, t := range r.Types {
		templates[i] = t.Template
	}
	return layout.NewMatcher(r.Prefix, r.Repo.Host, r.Repo.Owner, r.Repo.Name, templates...)
}

// Data returns key template data for the repo.
func (r *Resolved) Data(branch, slug, file string) layout.Data {
	return layout.Data{Host: r.Repo.Host, Owner: r.Repo.Owner, Repo: r.Repo.Name, Branch: branch, Slug: slug, File: file}
}

// VaultSpecs converts the vault config for the secrets package.
func VaultSpecs(vaults map[string]Vault) map[string]secrets.VaultSpec {
	out := make(map[string]secrets.VaultSpec, len(vaults))
	for name, v := range vaults {
		out[name] = secrets.VaultSpec{Path: v.Path, Format: v.Format, Identity: v.Identity}
	}
	return out
}
