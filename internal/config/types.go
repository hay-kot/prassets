// Package config loads, validates and merges the global config and a repo's
// .prassets.yaml.
package config

// Config is the global config file.
type Config struct {
	Vaults  map[string]Vault `yaml:"vaults,omitempty"  jsonschema_description:"Encrypted YAML files that secret templates can read with vault. Decrypted only when a template calls vault for them, at most once per run."`
	Stores  map[string]Store `yaml:"stores,omitempty"  jsonschema_description:"Named S3-compatible buckets. Several repos can share one store."`
	Default *Entry           `yaml:"default,omitempty" jsonschema_description:"Used for any repo that has no entry under repos. Omit it to make unknown repos an error."`
	Repos   map[string]Entry `yaml:"repos,omitempty"   jsonschema_description:"Keyed by repo id: host/owner/name, from the origin remote."`
}

// Vault is an encrypted YAML file of nested maps with string leaves.
type Vault struct {
	Path     string `yaml:"path"               jsonschema:"required"                                                                                                                   jsonschema_description:"Path to the encrypted file. ~ is expanded."`
	Format   string `yaml:"format"             jsonschema:"required,enum=age,enum=sops"                                                                                                jsonschema_description:"age: the whole file is age-encrypted. sops: a sops-encrypted YAML file with age recipients."`
	Identity string `yaml:"identity,omitempty" jsonschema_description:"age identity file. Default: $PRASSETS_AGE_IDENTITY, then $SOPS_AGE_KEY_FILE, then ~/.config/sops/age/keys.txt."`
}

// Store is an S3-compatible bucket. The credential fields are secret
// templates.
type Store struct {
	Endpoint     string `yaml:"endpoint"                jsonschema:"required"                                                        jsonschema_description:"S3 endpoint host, with no scheme."`
	Bucket       string `yaml:"bucket"                  jsonschema:"required"                                                        jsonschema_description:"Bucket name."`
	Region       string `yaml:"region,omitempty"        jsonschema_description:"Bucket region. Default: auto."`
	PublicURL    string `yaml:"public_url"              jsonschema:"required"                                                        jsonschema_description:"Base URL that serves the bucket."`
	Insecure     bool   `yaml:"insecure,omitempty"      jsonschema_description:"Use http instead of https, for a local MinIO."`
	AccessKey    string `yaml:"access_key"              jsonschema:"required"                                                        jsonschema_description:"Secret template: a literal, or a Go template with env, cmd, vault and file."`
	SecretKey    string `yaml:"secret_key"              jsonschema:"required"                                                        jsonschema_description:"Secret template: a literal, or a Go template with env, cmd, vault and file."`
	SessionToken string `yaml:"session_token,omitempty" jsonschema_description:"Optional secret template for temporary credentials."`
}

// Entry maps a repo, or every repo without an entry, to a store.
type Entry struct {
	Store  string             `yaml:"store"            jsonschema:"required"                                                                                           jsonschema_description:"Name of a store under stores."`
	Prefix string             `yaml:"prefix,omitempty" jsonschema_description:"Key prefix for every key of the repo. A template that may use .Host, .Owner and .Repo."`
	PR     *TypeDef           `yaml:"pr,omitempty"     jsonschema_description:"Overrides the key layout of the pr type."`
	Types  map[string]TypeDef `yaml:"types,omitempty"  jsonschema_description:"Extra asset types. Wins over .prassets.yaml per type name."`
}

// TypeDef is one place assets can live.
type TypeDef struct {
	Key   string `yaml:"key"             jsonschema:"required"                                              jsonschema_description:"Key template. Fields: .Host .Owner .Repo .Branch .Slug .File. Functions: lower, slugify. Must reference .File."`
	Usage string `yaml:"usage,omitempty" jsonschema_description:"What the type is for, shown by pra types."`
}

// RepoFile is a repo's .prassets.yaml. It is untrusted repo content, so it
// may only define key layouts.
type RepoFile struct {
	PR    *TypeDef           `yaml:"pr,omitempty"    jsonschema_description:"Overrides the key layout of the pr type."`
	Types map[string]TypeDef `yaml:"types,omitempty" jsonschema_description:"Asset types for this repo."`
}
