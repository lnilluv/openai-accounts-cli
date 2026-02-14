# openai-accounts-cli (oa)

**oa** helps keeping track of usage across multiple OpenAI accounts.

## What it does

- Stores per-account auth references in `~/.codex/accounts.toml`
- Stores secrets via `pass`, with file fallback at `~/.codex/secrets`
- Supports API key and ChatGPT OAuth token auth
- Fetches daily and weekly usage limits from OpenAI
- Renders human-readable or JSON output

## Install

```bash
go install github.com/bnema/openai-accounts-cli/cmd/oa@latest
```

### Auth setup

```bash
oa auth login browser
```

### Usage

```bash
# Fetch limits for all accounts
oa usage

# Fetch limits for specific account
oa usage --account 1

# JSON output
oa status --account 1 --json
```

## Commands

| Command | Description |
|---------|-------------|
| `oa auth set\|remove` | Manage authentication |
| `oa auth login browser\|device` | Login flows |
| `oa usage [--account <id>] [--json]` | Fetch usage limits (all accounts if no ID specified) |
| `oa status [--account <id>] [--json]` | Alias for usage |
| `oa account list` | List accounts |
| `oa version` | Print version |

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `OA_AUTH_ISSUER` | `https://auth.openai.com` | Auth issuer endpoint |
| `OA_AUTH_CLIENT_ID` | Embedded in source | OAuth client identifier |
| `OA_AUTH_LISTEN` | `127.0.0.1:1455` | Local listener address |
| `OA_USAGE_BASE_URL` | `https://chatgpt.com/backend-api` | Usage API base URL |

## Project layout

- **`cmd/`** — Cobra command definitions and app wiring
- **`internal/application/`** — Orchestration and use-case logic
- **`internal/domain/`** — Entities, validation, and business rules
- **`internal/ports/`** — Repository, secret-store, and clock interfaces
- **`internal/adapters/`** — TOML, secret stores, rendering, and auth adapters

## Development

```bash
go test ./...
```
