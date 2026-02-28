# openai-accounts-cli (oa)

**oa** helps keeping track of usage across multiple OpenAI accounts.

## What it does

- Stores per-account auth references in `~/.codex/accounts.toml`
- Stores secrets via `pass`, with file fallback at `~/.codex/secrets`
- Supports API key and ChatGPT OAuth token auth
- Fetches daily and weekly usage limits from OpenAI
- Shows subscription renewal countdown (when the subscription renews or expires)
- Highlights accounts with exhausted weekly limits
- Recommends which account to use first (prioritizes by weekly usage pressure)
- Creates and manages a default OpenAI account pool for auto-switching
- Runs child tools (like opencode) with pool-selected account/session env
- Renders human-readable or JSON output

## Install

```bash
go install github.com/bnema/openai-accounts-cli/cmd/oa@latest
```

### Add `oa` to your `PATH`

`go install` places the binary in `$(go env GOPATH)/bin` (usually `$HOME/go/bin`) unless `GOBIN` is set.

- `bash`
  ```bash
  echo 'export PATH="$HOME/go/bin:$PATH"' >> ~/.bashrc
  source ~/.bashrc
  ```
- `zsh`
  ```bash
  echo 'export PATH="$HOME/go/bin:$PATH"' >> ~/.zshrc
  source ~/.zshrc
  ```
- `fish`
  ```fish
  fish_add_path $HOME/go/bin
  ```
- `PowerShell`
  ```powershell
  $goBin = (go env GOPATH) + "\\bin"
  [Environment]::SetEnvironmentVariable("Path", $env:Path + ";$goBin", "User")
  ```

Verify installation:

```bash
which oa
oa version
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

# Activate default pool (auto-discovers OpenAI accounts)
oa pool activate

# Show pool status
oa pool status

# Switch manually to next eligible account
oa pool next

# Switch to specific account by ID or name
oa pool switch --account 2

# Interactive switch (shows numbered eligible accounts)
oa pool switch

# Run opencode with auto-selected account from pool
oa run --pool default-openai -- opencode

# Optional: isolate continuity per terminal/window
OA_WINDOW_FINGERPRINT=project-a-window-1 oa run --pool default-openai -- opencode
```

## Commands

| Command | Description |
|---------|-------------|
| `oa auth set\|remove` | Manage authentication |
| `oa auth login browser\|device` | Login flows |
| `oa usage [--account <id>] [--json]` | Fetch usage limits and subscription renewal info (all accounts if no ID specified) |
| `oa status [--account <id>] [--json]` | Alias for usage |
| `oa account list` | List accounts |
| `oa pool activate\|deactivate\|status\|next\|switch` | Manage default OpenAI pool state and selected account |
| `oa run --pool <id> -- <cmd>` | Run a command with pool-selected account and session env |
| `oa version` | Print version |

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `OA_AUTH_ISSUER` | `https://auth.openai.com` | Auth issuer endpoint |
| `OA_AUTH_CLIENT_ID` | Embedded in source | OAuth client identifier |
| `OA_AUTH_LISTEN` | `127.0.0.1:1455` | Local listener address |
| `OA_USAGE_BASE_URL` | `https://chatgpt.com/backend-api` | Usage API base URL |
| `OA_WINDOW_FINGERPRINT` | `default` | Window/session fingerprint for pool continuity |

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

## Roadmap Notes

- See `docs/TODO.md` for planned improvements (including custom pool creation for personal/pro account rotation).
