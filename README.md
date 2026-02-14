# openai-accounts-cli (oa)

`oa` means **OpenAI Accounts CLI**. It is a terminal-first tool for managing account auth metadata and fetching OpenAI usage limits in one place.

## What oa does

- stores per-account auth references in `~/.codex/accounts.toml`
- stores secrets using `pass` first, with file fallback at `~/.codex/secrets`
- supports API key and ChatGPT OAuth token auth records
- fetches daily and weekly usage limits from OpenAI and persists snapshots
- renders human-readable or JSON account status output

## Current command surface

```text
oa auth set|remove
oa login browser|device
oa usage [--account <id>] [--json]
oa status [--account <id>] [--json]   # alias of usage
oa account list
oa version
```

## Quickstart

```bash
# build and run
go build ./...
go run . version

# set auth for an account (0 or empty auto-assigns next ID)
go run . auth set \
  --account 0 \
  --method chatgpt \
  --secret-key openai://1/oauth_tokens \
  --secret-value '{"access_token":"...","id_token":"..."}'

# fetch limits and render status
go run . usage --account 1

# same command via alias
go run . status --account 1 --json
```

## Configuration

- `OA_AUTH_ISSUER` (default: `https://auth.openai.com`)
- `OA_AUTH_CLIENT_ID` (default embedded in source)
- `OA_AUTH_LISTEN` (default: `127.0.0.1:1455`)
- `OA_USAGE_BASE_URL` (default: `https://chatgpt.com/backend-api`)

## Project layout

- `cmd/`: Cobra command definitions and app wiring
- `internal/application/`: orchestration and use-case logic
- `internal/domain/`: entities, validation, and business rules
- `internal/ports/`: repository, secret-store, and clock interfaces
- `internal/adapters/`: TOML, secret stores, rendering, and auth adapters

## Development

```bash
go test ./...
```
