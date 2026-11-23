# AGENTS.md

Guidance for agents that work on prassets. To use `pra` in another repo, run `pra onboard` there instead.

## What this is

`pra` uploads files for a repo to an S3-compatible bucket and prints stable URLs. `docs/spec.md` is the source of truth for behavior. When a change alters behavior, update the spec, `README.md` and the onboarding guide (`internal/commands/onboard.md.tmpl`) in the same change.

## Layout

| Path | Contents |
|---|---|
| `cmd/pra` | `main`: build info, exit codes (2 for `commands.UsageError`, 1 for other errors) |
| `internal/commands` | One file per command, `app.go` builds the CLI. `session.go` resolves the config, repo and store for a run. |
| `internal/config` | Global config and `.prassets.yaml`: strict decode, validation, merge (`Resolve`), JSON schemas, the starter config |
| `internal/layout` | Key templates, key checks, file name normalization, the writable-key matcher |
| `internal/secrets` | Secret templates (`env`, `cmd`, `file`) |
| `internal/store` | The S3 calls (minio-go) |
| `internal/gitctx` | Repo id from the remote, current branch |
| `internal/devstack`, `dev/` | RustFS dev stack and bucket setup |
| `test/integration` | Tests that run the built binary against RustFS (build tag `integration`) |

## Tasks

```sh
mise run fmt           # gofumpt + goimports through golangci-lint
mise run lint          # golangci-lint, including integration-tagged files
mise run test          # unit tests
mise run schema        # regenerate schema.json and schema.repo.json
mise run dev:up        # RustFS on 127.0.0.1:9000 with the prassets-dev bucket
mise run integration   # integration tests (uses the dev stack when PRASSETS_TEST_S3_ENDPOINT is set)
mise run dev:down
```

Run fmt, lint and the unit tests before each commit. Run the integration tests when a change touches `store`, `commands` or anything that talks to the bucket. Docker must run for them; without Docker the S3 tests skip.

## Invariants

- **Key templates are untrusted.** `.prassets.yaml` comes from the repo, so key templates get only `lower` and `slugify`. Never give them `env`, `cmd`, `file` or any other function with side effects. `.prassets.yaml` may hold only `pr` and `types`.
- **Secrets never leave the process.** No rendered secret in logs, errors, `config show` or `onboard`. text/template and yaml.v3 errors quote their input, which can hold a literal secret, so `secrets` and `config` replace those errors. Keep it that way when you add error paths.
- **Every rendered key passes `layout.CheckKey`**, and `rm` deletes only keys that `Matcher.Writable` accepts.
- **`put` checks everything before the first upload**: names, duplicates, content types, keys, then the overwrite check for every key.
- **stdout carries only results.** Logs go to stderr through zerolog. Usage mistakes return a `UsageError` (exit 2).
- **Config decodes strictly.** A new config field needs a schema regeneration (`mise run schema`); `TestSchemaDrift` fails otherwise.

## Conventions

- `exhaustruct` is on for this module's types, so struct literals set every field.
- Tests use testify. Table tests for `layout`, `config` and `secrets`; in-process command tests through `NewApp` in `internal/commands/commands_test.go`.
- New commands: `scaffold new command` writes `internal/commands/<name>.go` and registers it in `app.go`.
- Dependencies and tools: only versions at least 7 days old. `mise.toml` pins the tools and `mise.lock` holds their checksums.
