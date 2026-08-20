# Tongs

Tongs is an agent-native command-line client for pull and merge requests. It provides stable JSON, non-interactive behavior, safe mutations, and first-class review-thread operations.

It started because `gh` kept failing on unrelated GraphQL queries. Tongs uses REST by default and isolates GraphQL to operations that genuinely require it.

GitHub is the first provider. The domain model, repository detection, commands, and output schema are provider-neutral so GitLab, Gitea, Codeberg, and other forges can be added as adapters.

## Installation

Once published at `github.com/ducks/tongs`:

```console
go install github.com/ducks/tongs/cmd/tongs@latest
```

Version tags can be installed explicitly, for example `@v0.1.0`. GitHub release binaries and package-manager definitions can use the same build.

## Development

Enter the Nix development shell:

```console
nix develop
```

Build and test:

```console
go test ./...
go build ./cmd/tongs
```

Or build directly with Nix:

```console
nix build
./result/bin/tongs --version
```

## Authentication

Credentials are resolved without displaying them, in this order:

1. `PR_TOKEN`
2. Provider-specific variables such as `PR_GITHUB_TOKEN`
3. Conventional variables such as `GH_TOKEN` or `GITHUB_TOKEN`
4. The Git credential helper for the selected host
5. Existing `gh` authentication as a migration fallback for GitHub

The GitHub fallback is optional. Tongs does not otherwise invoke `gh`.

## Repository and provider detection

Known public hosts are detected automatically:

| Host | Provider |
| --- | --- |
| `github.com` | `github` |
| `gitlab.com` | `gitlab` |
| `codeberg.org` | `gitea` |

Self-hosted forges use explicit overrides:

```console
tongs --provider github --host github.example.com --repo acme/widget inspect 42
tongs --provider gitlab --repo gitlab.example.com/acme/widget inspect 42
```

GitLab and Gitea repository/PR parsing is implemented, but their API adapters are not yet implemented. Requests fail explicitly with `provider_not_implemented` rather than falling through to GitHub.

## Commands

Global flags precede the command. A PR may be a number or a full URL. If omitted, Tongs finds the open PR for the current git branch.

```console
tongs --pretty inspect 74
tongs reviews https://github.com/discourse-org/jenkins-job-definitions/pull/74
tongs checks 74

tongs edit --body-file description.md 74
tongs edit --body-file - 74 < description.md

tongs reply --thread PRRT_node_id --body-file reply.md 74
tongs resolve --thread PRRT_node_id 74

tongs --dry-run merge --method squash 74
tongs merge --method squash --yes 74
```

Mutating commands support `--dry-run`. Merge additionally requires `--yes` and sends the current head SHA unless an explicit `--sha` is supplied, preventing a stale merge.

## Output contract

Success:

```json
{"schema_version":"1","ok":true,"data":{}}
```

Failure:

```json
{"schema_version":"1","ok":false,"error":{"code":"not_found","message":"Not Found","status_code":404}}
```

Exit codes:

| Code | Meaning |
| --- | --- |
| `0` | Command succeeded |
| `1` | Provider, network, or internal error |
| `2` | Invalid command usage |
| `3` | Checks completed with failures |

## Adding a provider

Adapters implement [`provider.Provider`](internal/provider/provider.go) and map forge-specific responses into the shared models in [`models.go`](internal/provider/models.go). Register the adapter in [`internal/cli/cli.go`](internal/cli/cli.go).

Provider-specific GraphQL or REST details stay inside the adapter. GitHub, for example, uses REST for ordinary PR operations and narrowly scoped GraphQL queries only for review-thread replies and resolution, which GitHub does not expose through REST.
