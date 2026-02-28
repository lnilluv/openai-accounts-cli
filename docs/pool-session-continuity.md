# Pool Session Continuity

`oa run` provides pool-level continuity metadata so tools can keep context while switching accounts.

## How it works

- `oa pool activate` creates/enables `default-openai` and auto-syncs OpenAI accounts.
- `oa run --pool default-openai -- <cmd>` picks the best non-exhausted account.
- `oa` resolves a logical session ID from `workspace_root + OA_WINDOW_FINGERPRINT`.
- `oa` maps that logical session to an account-scoped provider session ID in `~/.codex/pool_runtime.toml`.
- When account selection changes, the same logical session is reused and a provider session mapping is attached for the target account.

## Environment variables injected by `oa run`

- `OA_POOL_ID`: selected pool ID.
- `OA_ACTIVE_ACCOUNT`: selected account ID.
- `OA_LOGICAL_SESSION_ID`: stable cross-account logical session key.
- `OA_PROVIDER_SESSION_ID`: account-scoped provider session key.

## Limits

- Provider-native conversation IDs are account-scoped and cannot be shared directly across accounts.
- Continuity is modeled as logical-session linkage with account-session mapping.
- `OA_WINDOW_FINGERPRINT` should be unique per active window to avoid context collisions.
