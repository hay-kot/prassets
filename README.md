# prassets

`pra` uploads screenshots and other public files for a repo to an S3-compatible bucket and prints stable URLs. One global config maps repos to buckets, so a new repo needs a few lines of YAML. It works with R2, AWS S3, MinIO, RustFS, B2 and Tigris.

```sh
pra put "Screenshot 2026-11-23 at 10.42.13.png"
# https://assets.example.com/github/acme/widgets/pr/feat-login/screenshot-2026-11-23-at-10.42.13.png

pra md   # Markdown for every asset of this branch, ready for the PR body
```

stdout carries only results (URLs, Markdown or JSON). Logs go to stderr. Exit codes: `0` success, `1` error, `2` usage error.

## Install

Download a binary for your platform from the [releases page](https://github.com/hay-kot/prassets/releases) and put `pra` on your `PATH`. Or build it with Go:

```sh
go install github.com/hay-kot/prassets/cmd/pra@latest
```

Then create a config and check it:

```sh
pra config init    # writes ~/.config/prassets/config.yaml
$EDITOR "$(pra config path)"
pra config check   # resolves the repo, store, templates and secrets, then HEADs the bucket
```

## Global config

Location: `$XDG_CONFIG_HOME/prassets/config.yaml` (default `~/.config/prassets/config.yaml`). Override it with `--config` or `PRASSETS_CONFIG`. Unknown keys are errors. The [JSON schema](schema.json) gives editor completion through the `yaml-language-server` comment.

```yaml
# yaml-language-server: $schema=https://raw.githubusercontent.com/hay-kot/prassets/main/schema.json

stores:
  personal:
    endpoint: abc123.r2.cloudflarestorage.com   # host only, no scheme
    bucket: my-public
    region: auto                                # default "auto"
    public_url: https://assets.example.com      # base URL that serves the bucket
    access_key: '{{ cmd "op read op://Personal/r2/access-key" }}'
    secret_key: '{{ cmd "op read op://Personal/r2/secret-key" }}'
  work:
    endpoint: s3.us-east-1.amazonaws.com
    bucket: team-pr-assets
    region: us-east-1
    public_url: https://team-pr-assets.s3.amazonaws.com
    access_key: '{{ env "AWS_ACCESS_KEY_ID" }}'
    secret_key: '{{ cmd "op read op://Work/pr-assets/secret" }}'

# Used for any repo without an entry under repos. Omit it to make unknown repos an error.
default:
  store: personal
  prefix: "github/{{ .Owner }}/{{ .Repo }}/"

# Keyed by repo id: host/owner/name, from the origin remote.
repos:
  github.com/hay-kot/recipinned:
    store: personal
    prefix: ""
    pr:
      key: "github/type=pr/branch={{ .Branch }}/{{ .File }}"
  github.com/acme/widgets:
    store: work
```

Secret fields (`access_key`, `secret_key`, `session_token`) are Go templates:

| Function | Returns |
|---|---|
| `env "NAME"` | The env var. Unset or empty is an error. |
| `cmd "..."` | Trimmed stdout of `sh -c`. A non-zero exit is an error with the command's stderr. |
| `file "path"` | Trimmed contents of a file (`~` expanded). |

A value with no `{{ }}` is a literal. Secrets render only for commands that talk to the bucket, and only for the store the current repo uses. They never appear in logs, errors or `pra config show`.

## Repo config: `.prassets.yaml`

A repo can commit `.prassets.yaml` at its root to share its key layout with collaborators. It may hold only `pr` and `types`. It cannot name a store, a prefix or a secret, and the global prefix is always prepended, so a repo file cannot write outside the area your global config gives that repo.

```yaml
# yaml-language-server: $schema=https://raw.githubusercontent.com/hay-kot/prassets/main/schema.repo.json
pr:
  key: "screenshots/{{ .Branch }}/{{ .File }}"
types:
  blog:
    key: "website/type=blog/post={{ .Slug }}/{{ .File }}"
    usage: images for one blog post; the slug is the .mdx filename
  brand:
    key: "brand/{{ .File }}"
    usage: logos shared by email, the site and the app
```

Types merge in this order, later wins per type name: the built-in `pr` type (`pr/{{ .Branch }}/{{ .File }}`), then `.prassets.yaml`, then the global `repos.<id>` (or `default`) entry. `pra types` and `pra config show` print where each type came from.

Key templates get `.Host`, `.Owner`, `.Repo`, `.Branch` (slugified), `.Slug` (from `--slug`) and `.File`, plus the functions `lower` and `slugify`. They never get `env`, `cmd` or `file`, because `.prassets.yaml` is untrusted repo content. A template must use `.File` as its last segment. A type that uses `.Slug` needs `--slug`; one that does not refuses it.

```sh
pra put -t blog -s how-to-share hero.webp
pra put -t brand --as logo-banner.png ~/Downloads/Logo\ Final.PNG
```

## Commands

Global flags: `--config`, `--repo host/owner/name`, `--remote` (default `origin`), `--branch`, `--format url|md|json`, `--log-level`.

| Command | What it does |
|---|---|
| `put <file>...` | Upload to the `pr` type for the current branch. `-t/--type` and `-s/--slug` pick another type, `--as` renames one file, `--force` replaces existing keys. |
| `ls [prefix]` | List this branch's PR assets (size, URL). `-t`/`-s` for another type, `--all` for the whole repo prefix, or a prefix inside it. |
| `md` | `ls --format md`. |
| `get <key>` | Download one object into `-o/--dir`. |
| `rm <key>...` | Delete objects. A bare file name resolves against the current branch's PR area. Keys that match none of the repo's types are refused. Asks on a TTY unless `--yes`. |
| `types` | The repo's types: name, key template, source, usage. |
| `config path\|init\|show\|check` | Find, create, inspect and verify the global config. |
| `onboard` | Explain this repo's setup, the next step, and where each setting lives. Written for agents and new users. |
| `schema [global\|repo]` | Print the JSON schema of the global config or of `.prassets.yaml`. |
| `version` | Version and commit. |

`get` and `rm` also take the URLs that `put` and `ls` print.

Safety rules kept from the recipinned tool: file names are normalized to `^[a-z0-9][a-z0-9._-]*$` (each rename is logged), every key is checked before the first upload, an existing key is refused unless `--force`, uploads get `Cache-Control: public, max-age=31536000, immutable`, and an unknown extension is an error.

## Secrets from encrypted files

`pra` has no secret store of its own. Keep credentials in a password manager or an encrypted file, and read them with `cmd`:

```yaml
# 1Password
secret_key: '{{ cmd "op read op://Personal/r2/secret-key" }}'
# an age-encrypted file that holds only the value
secret_key: '{{ cmd "age -d -i ~/.config/age/keys.txt ~/.config/prassets/r2-secret.age" }}'
# one key of a sops-encrypted YAML file (inside a single-quoted YAML string, write '' for ')
secret_key: '{{ cmd "sops decrypt --extract ''[\"r2\"][\"secret_key\"]'' ~/.config/prassets/secrets.sops.yaml" }}'
```

`cmd` runs only for commands that talk to the bucket, and only for the store the current repo uses.

## For agents

Run `pra onboard` in a repo. It reports the state of the checkout (global config, repo id, entry, `.prassets.yaml`, types) and the next step. It also explains which settings belong in the global config and which in `.prassets.yaml`, and the rules agents follow: the global config is the user's, `--force` needs the user's consent, and `rm` does not prompt without a terminal. `pra --format json onboard` gives the same state as JSON, and `pra schema global` or `pra schema repo` prints the schema to check an edit against.

To work on prassets itself, read [AGENTS.md](AGENTS.md).

## Expiring PR assets

`pra` does not manage bucket lifecycle rules. PR screenshots are rarely needed after a PR merges, so add a rule that expires the PR area after some days. S3 lifecycle filters match a key prefix, so add one rule per repo PR prefix, for example `github/acme/widgets/pr/` with the default layout above. Run `pra types` to see the full prefix.

```json
{
  "Rules": [
    {
      "ID": "expire-pr-assets-widgets",
      "Status": "Enabled",
      "Filter": { "Prefix": "github/acme/widgets/pr/" },
      "Expiration": { "Days": 90 }
    }
  ]
}
```

```sh
aws s3api put-bucket-lifecycle-configuration \
  --bucket my-public --lifecycle-configuration file://lifecycle.json
# For R2, add: --endpoint-url https://<account-id>.r2.cloudflarestorage.com
```

To cover every repo with one rule, give the PR type a shared top-level prefix in the global config, for example `prefix: ""` with `pr: {key: "pr/{{ .Owner }}/{{ .Repo }}/{{ .Branch }}/{{ .File }}"}`, and expire `pr/`. The tradeoff: with an empty prefix, a repo's `.prassets.yaml` types can write anywhere in the bucket.

Expired keys stay in browser and CDN caches for up to the `max-age`, and old PR bodies will show broken images after expiry.

## Development

```sh
mise run test          # unit tests
mise run lint          # golangci-lint, including integration-tagged files
mise run schema        # regenerate schema.json and schema.repo.json after changing config types
mise run dev:up        # start a local RustFS S3 server on 127.0.0.1:9000 and create the bucket
mise run integration   # integration tests against RustFS
mise run dev:down      # stop RustFS and drop its data
```

`mise run integration` uses the dev stack when `PRASSETS_TEST_S3_ENDPOINT=127.0.0.1:9000` is set, and otherwise starts a throwaway RustFS container. Without Docker the S3 tests skip. To try `pra` by hand against the dev stack:

```sh
PRASSETS_CONFIG=dev/config.yaml go run ./cmd/pra put shot.png
```
