package config

// Starter is the commented config that pra config init writes.
const Starter = `# yaml-language-server: $schema=https://raw.githubusercontent.com/hay-kot/prassets/main/schema.json
#
# prassets config. Run "pra config check" after editing.

# Encrypted YAML files that secret templates can read with vault. Create one
# with "pra vault init personal", then "pra vault edit personal".
# vaults:
#   personal:
#     path: ~/.config/prassets/secrets.age.yaml
#     format: age
#     identity: ~/.config/sops/age/keys.txt

stores:
  personal:
    endpoint: <account-id>.r2.cloudflarestorage.com   # host only, no scheme
    bucket: my-public-assets
    region: auto
    public_url: https://assets.example.com             # base URL that serves the bucket
    # Secret fields are Go templates with env, cmd, vault and file.
    # A value with no {{ }} is a literal.
    access_key: '{{ env "PRASSETS_ACCESS_KEY" }}'
    secret_key: '{{ env "PRASSETS_SECRET_KEY" }}'
    # access_key: '{{ vault "personal" "r2.access_key" }}'
    # secret_key: '{{ cmd "op read op://Personal/r2/secret" }}'

# Used for any repo that has no entry under repos. Remove it to make unknown
# repos an error.
default:
  store: personal
  prefix: "github/{{ .Owner }}/{{ .Repo }}/"

# Keyed by repo id: host/owner/name, from the origin remote.
# repos:
#   github.com/acme/widgets:
#     store: personal
#     prefix: "widgets/"
#     pr:
#       key: "pr/{{ .Branch }}/{{ .File }}"
#     types:
#       blog:
#         key: "blog/{{ .Slug }}/{{ .File }}"
#         usage: images for one blog post
`
