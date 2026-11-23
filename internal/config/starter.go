package config

// Starter is the commented config that pra config init writes.
const Starter = `# yaml-language-server: $schema=https://raw.githubusercontent.com/hay-kot/prassets/main/schema.json
#
# prassets config. Run "pra config check" after editing.

stores:
  personal:
    endpoint: <account-id>.r2.cloudflarestorage.com   # host only, no scheme
    bucket: my-public-assets
    region: auto
    public_url: https://assets.example.com             # base URL that serves the bucket
    # Secret fields are Go templates with env, cmd and file.
    # A value with no {{ }} is a literal.
    access_key: '{{ env "PRASSETS_ACCESS_KEY" }}'
    secret_key: '{{ env "PRASSETS_SECRET_KEY" }}'
    # access_key: '{{ cmd "op read op://Personal/r2/access-key" }}'
    # secret_key: '{{ cmd "age -d -i ~/.config/age/keys.txt ~/.config/prassets/r2-secret.age" }}'

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
