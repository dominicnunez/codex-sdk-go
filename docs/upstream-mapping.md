# Upstream Mapping

This repository keeps `sdk/` as the app-server protocol package and mirrors selected upstream Codex login/runtime behavior in companion packages.

## Package Boundaries

| Local package | Upstream owner | Scope |
| --- | --- | --- |
| `sdk/` | `codex-rs/app-server-protocol/src`, `codex-rs/app-server-protocol/schema` | JSON-RPC methods, protocol types, notifications, approvals, and schema coverage. |
| `login/` | `codex-rs/login/src/server.rs`, `codex-rs/login/src/pkce.rs`, `codex-rs/rmcp-client/src/perform_oauth_login.rs` | PKCE, authorize URL construction, callback handling, manual code parsing, token exchange, and refresh. |
| `login/auth/` | `codex-rs/login/src/auth/manager.rs`, `codex-rs/login/src/auth/storage.rs`, `codex-rs/login/src/token_data.rs`, `codex-rs/app-server-protocol/src/protocol/v2/account.rs`, `codex-rs/app-server-protocol/src/protocol/common.rs` | Credential persistence, token claims, redaction, and `chatgptAuthTokens` login/refresh payload helpers. |
| `appserver/` | `codex-rs/app-server/src`, `codex-rs/app-server-client/src`, `codex-rs/app-server-transport/src/transport` | `codex app-server --listen stdio://` process startup, initialize lifecycle, and shutdown behavior. |
| `appserver/protocol/` | `codex-rs/app-server-protocol/src`, `codex-rs/app-server-protocol/schema` | Method and notification name constants used by runtime helpers. |
| `appserver/transport/` | `codex-rs/app-server-transport/src/transport`, `codex-rs/app-server-client/src/remote.rs` | Newline-delimited JSON-RPC framing, request/response matching, notification delivery, queue limits, and stdio I/O. |
| `exec/` | `codex-rs/exec/src`, `codex-rs/app-server-client/src`, `codex-rs/app-server/src` | Single-turn `Run`, streamed turns, conversation helpers, event projection, and collab tracking. |
| `internal/process/` | Go-only support code | Process-tree and signal handling around local child processes. |

## Watch Paths

Use these upstream paths when checking whether this SDK needs updates:

- `codex-rs/login/src/pkce.rs`
- `codex-rs/login/src/server.rs`
- `codex-rs/login/src/token_data.rs`
- `codex-rs/login/src/auth/manager.rs`
- `codex-rs/login/src/auth/storage.rs`
- `codex-rs/rmcp-client/src/perform_oauth_login.rs`
- `codex-rs/exec/src`
- `codex-rs/app-server-client/src`
- `codex-rs/app-server/src`
- `codex-rs/app-server-protocol/src`
- `codex-rs/app-server-protocol/schema`
- `codex-rs/app-server-transport/src/transport`

## Current Protocol Facts

- `account/login/start` accepts `LoginAccountParams::ChatgptAuthTokens` with wire type `chatgptAuthTokens`.
- `account/chatgptAuthTokens/refresh` is `ServerRequest::ChatgptAuthTokensRefresh`.
- Upstream marks `chatgptAuthTokens` as experimental/internal and stores externally supplied tokens in memory.
- Codex-managed OAuth currently requests `openid profile email offline_access api.connectors.read api.connectors.invoke`.
- ChatGPT token refresh posts JSON with `client_id`, `grant_type`, and `refresh_token` to `https://auth.openai.com/oauth/token`.
- App-server stdio startup uses `codex app-server --listen stdio://`.
