# Command Examples

All examples assume you run from repository root with `go run .`.

## Auth

Set API key auth:

```bash
go run . auth set \
  --account 1 \
  --method api_key \
  --secret-key openai://1/api_key \
  --secret-value sk-test-value
```

Set ChatGPT OAuth tokens:

```bash
go run . auth set \
  --account 1 \
  --method chatgpt \
  --secret-key openai://1/oauth_tokens \
  --secret-value '{"access_token":"access-token","id_token":"id-token"}'
```

Remove auth:

```bash
go run . auth remove --account 1
```

## Accounts

List accounts loaded from `~/.codex/accounts.toml`:

```bash
go run . account list
```

## Usage and Status

Fetch usage limits and render status:

```bash
go run . usage --account 1
```

Fetch all accounts:

```bash
go run . usage
```

JSON output:

```bash
go run . usage --account 1 --json
```

`status` is an alias of `usage`:

```bash
go run . status --account 1
```

## Login

Start browser login flow:

```bash
go run . login browser --account 1
```

`login device` exists but is not implemented yet:

```bash
go run . login device --account 1
```
