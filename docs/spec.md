# prassets: spec

`prassets` (binary `pra`) uploads screenshots and other public files for a repo to an S3-compatible bucket and prints stable URLs. It is a port of recipinned's `backend/cmd/tools/assets`, made general so any repo can use it with a few lines of config.

## Goals

- One global config that maps repos to buckets. A new repo needs a few lines of YAML, not a new tool.
- `pra put shot.png` in any checkout uploads to that repo's PR area for the current branch and prints the URL.
- Secrets come from env vars, commands or files, never from plain text in the config. Encrypted storage (a password manager, age, sops) stays with the user, who reads it through `cmd`.
- A repo can commit `.prassets.yaml` to share its key layout with collaborators.
- Keep the recipinned safety rules: a fixed key layout, no silent overwrites, immutable cache headers, and keys outside the layout are read-only.
- Agent-friendly: stdout carries only results (URLs, Markdown or JSON), logs go to stderr, errors are specific.

## Non-goals (v1)

- Editing PR bodies or posting comments. The tool prints Markdown; the user or `gh` puts it where it goes.
- Image conversion or resizing.
- Managing bucket lifecycle rules. The README documents a recommended rule (expire the PR prefix after N days).
- Non-S3 backends. R2, AWS S3, MinIO, B2 and Tigris all speak S3.

## Global config

Location: `$XDG_CONFIG_HOME/prassets/config.yaml` (default `~/.config/prassets/config.yaml`). Override with `--config` or `PRASSETS_CONFIG`.

```yaml
# yaml-language-server: $schema=https://raw.githubusercontent.com/hay-kot/prassets/main/schema.json

# Named buckets. Several repos can share one store.
stores:
  personal:
    endpoint: abc123.r2.cloudflarestorage.com   # host only, no scheme
    bucket: my-public
    region: auto                                # optional, default "auto"
    public_url: https://assets.example.com      # base URL that serves the bucket
    insecure: false                             # http, for a local MinIO
    # Secret fields are Go templates. A value with no {{ }} is a literal.
    access_key: '{{ cmd "op read op://Personal/r2/access-key" }}'
    secret_key: '{{ cmd "age -d -i ~/.config/age/keys.txt ~/.config/prassets/r2-secret.age" }}'
  work:
    endpoint: s3.us-east-1.amazonaws.com
    bucket: team-pr-assets
    region: us-east-1
    public_url: https://team-pr-assets.s3.amazonaws.com
    access_key: '{{ env "AWS_ACCESS_KEY_ID" }}'
    secret_key: '{{ cmd "op read op://Work/pr-assets/secret" }}'

# Used for any repo that has no entry under repos. Omit it to make
# unknown repos an error.
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

### Secret templates

Secret fields (`access_key`, `secret_key`, and `session_token` if present) are Go `text/template`s with these functions:

| Function | Returns |
|---|---|
| `env "NAME"` | The env var; an unset or empty var is an error. |
| `cmd "..."` | Trimmed stdout of `sh -c`; a non-zero exit is an error with the command's stderr. |
| `file "path"` | Trimmed contents of a file (`~` expanded). |

Secrets render only for commands that talk to the bucket, and only for the store the current repo uses. Rendered values never appear in logs or `config show`.

## Repo config: `.prassets.yaml`

An optional file at the repo root, committed, that defines the repo's layout. It may hold only `pr` and `types`; anything else is a load error. It cannot name a store, a prefix or a secret.

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

Merge order for a repo: built-in `pr` type, then `.prassets.yaml`, then the global `repos.<id>` entry, which may also define `pr` and `types` and wins per type name. `config show` prints which source each type came from.

The global prefix is always prepended, so a repo file cannot write outside the area the user's global config gives that repo.

## Key templates

Key templates are Go `text/template`s. They get the data below and only the pure functions `lower` and `slugify`. They never get `env`, `cmd` or `file`, because `.prassets.yaml` comes from the repo and is untrusted.

| Field | Value |
|---|---|
| `.Host`, `.Owner`, `.Repo` | From the repo id. |
| `.Branch` | Current branch, slugified (see below). |
| `.Slug` | `--slug`, validated. |
| `.File` | The file name after normalization. |

Rules:

- The built-in `pr` template is `pr/{{ .Branch }}/{{ .File }}`.
- A type whose template references `.Slug` requires `--slug`; one that does not refuses it. Detect this by walking the parsed template for the `.Slug` field, not by string search.
- A template must reference `.File`, and the rendered key must end with `/<file>` or equal `<file>`.
- The rendered key (prefix included) must not start with `/`, contain `//`, or have a `.` or `..` segment. Check this after rendering, every time.
- Templates parse at config load with `missingkey=error`, so a typo fails `config check` rather than the first upload.
- `slugify`: lower case, `/` and anything outside `[a-z0-9._-]` become `-`, runs of `-` collapse, leading and trailing `-` trim. `hay-kot/Recipe Wake` becomes `hay-kot-recipe-wake`.
- Detached HEAD is an error unless `--branch` is given.

## Repo resolution

- Repo id: `git remote get-url origin`, normalized from SSH or HTTPS to `host/owner/name` (lower case, no `.git`). `--repo` overrides it; `--remote` picks another remote.
- Entry: `repos[<id>]`, else `default`, else the error `no config for repo github.com/x/y; add it under repos: in <path>`.
- Validation at load: strict decode (unknown keys are errors) for both files, every referenced store exists, every template parses.
- JSON Schemas (`schema.json` for the global file, `schema.repo.json` for the repo file) are generated from the Go types and committed. A test fails when they drift.

## File names

Keys are case sensitive and URLs get retyped, so file names must match `^[a-z0-9][a-z0-9._-]*$`. `put` normalizes instead of refusing: the base name goes through `slugify` with the extension kept and lower cased, so `Screenshot 2026-11-23 at 10.42.13.png` becomes `screenshot-2026-11-23-at-10.42.13.png`. Each rename is logged to stderr. `--as <name>` names a single file explicitly and is validated, not rewritten. Slugs are validated, never rewritten. Two files that normalize to the same name are an error before any upload.

## Commands

Global flags: `--config`, `--repo`, `--remote`, `--branch`, `--log-level`, `--format url|md|json` (default `url`).

| Command | What it does |
|---|---|
| `put <file>...` | Upload to the `pr` type for the current branch. `-t/--type` and `-s/--slug` pick another type. `--force` replaces existing keys. `--as` renames one file. Prints one result per file. |
| `ls [prefix]` | List this branch's PR assets. `-t`/`-s` for another type, `--all` for everything under the repo prefix, or a raw prefix (still inside the repo prefix). Columns: size, URL. |
| `md` | `ls --format md`: Markdown for this branch's assets, ready to paste in a PR body. |
| `get <key>` | Download one object. `-o/--dir`. |
| `rm <key or file>...` | Delete objects. A bare file name resolves against the current branch's PR area. Keys that match none of the repo's types are refused. Asks on a TTY unless `--yes`. |
| `types` | The repo's types: name, resolved key template, source, usage. |
| `config path` | Print the global config path. |
| `config init` | Write a commented starter config if none exists. |
| `config show` | The resolved config for the current repo, secrets redacted. |
| `config check` | Resolve the repo, store, templates and secrets, then HEAD the bucket. Reports each step; non-zero exit on any failure. |
| `version` | Version and commit. |

### Output formats

- `url`: `https://assets.example.com/pr/feat-x/view.png`
- `md`: `![view](https://assets.example.com/pr/feat-x/view.png)`, alt text from the file name without extension.
- `json`: one array, `[{"key": "...", "url": "...", "size": 1234, "content_type": "image/png"}]`.

## Upload behavior (kept from recipinned)

- Check every key before the first upload, so one refused file does not leave the batch half uploaded.
- Refuse to overwrite an existing key without `--force`. The error names the key and suggests a new file name.
- `Cache-Control: public, max-age=31536000, immutable`. `--force` warns that caches may keep the old copy.
- `Content-Type` from the extension; an unknown extension is an error.
- `rm` deletes only keys that match one of the repo's type templates (`writable`). With templates, implement the match by turning each template into a pattern: static text literal, `.Branch`, `.Slug` and `.File` each one path segment of `[a-z0-9._-]+`.

## Exit codes

`0` success, `1` any error, `2` usage error.

## Implementation notes

- Go, urfave/cli v3, minio-go v7, a YAML library with strict decode, zerolog to stderr.
- Packages: `internal/config` (load, merge, validate, schema), `internal/layout` (key templates, key building, writable check, name normalization), `internal/secrets` (secret templates), `internal/store` (S3 put, list, get, delete, exists), `internal/gitctx` (remote and branch), `internal/commands`.
- `layout`, `config` and `secrets` are pure apart from file reads and get table tests; port the cases from recipinned's `layout_test.go`.
- `store` gets an integration test against MinIO (the scaffold's Docker integration test setup), skipped without Docker. It covers put, the overwrite refusal, ls, get and rm end to end.
- GoReleaser builds the `pra` binary and publishes to a Homebrew tap.

## Open questions

- Should `put` default the format to `md` when stdout is not a TTY? Decided: no, keep `url`.
