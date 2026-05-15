# SSH MCP Contract

## Scenario: SSH exec tool authentication defaults

### 1. Scope / Trigger
- Trigger: The SSH MCP server changed the request contract for `exec` and `sudo_exec` by making auth fields optional and introducing automatic private-key fallback from `~/.ssh`.

### 2. Signatures
- Tool: `exec(host, command, username?, password?, private_key?)`
- Tool: `sudo_exec(host, command, username?, password?, private_key?)`

### 3. Contracts
- Request fields:
  - `host`: required, non-empty SSH target.
  - `command`: required, non-empty remote command.
  - `username`: optional; defaults to `root` when omitted or empty.
  - `password`: optional.
  - `private_key`: optional path to a private key file.
- Connection sequencing:
  - Probe TCP reachability to `host:22` before building SSH auth methods.
  - If the TCP probe fails, stop immediately and return a network-oriented error.
- Authentication resolution order:
  - If `private_key` is provided, load that key directly.
  - If `password` is provided, add password auth.
  - If both `password` and `private_key` are empty, scan `~/.ssh` and only consider regular files with permission bits exactly `0400`.
  - Fallback keys are attempted in sorted filename order.
- Cache contract:
  - Requests with implicit fallback auth use the `auto_private_key` cache mode keyed by `host + username`.

### 4. Validation & Error Matrix
- Empty `host` -> return validation error.
- Empty `command` -> return validation error for tool requests.
- TCP probe to `host:22` fails -> return TCP reachability error before auth construction.
- Explicit `private_key` unreadable -> return error from key read.
- Explicit `private_key` unparsable -> return error from key parse.
- No explicit auth and `~/.ssh` missing or containing no `0400` private keys -> return auth resolution error.
- No explicit auth and discovered keys all fail to parse -> return auth resolution error listing parse failures.

### 5. Good / Base / Bad Cases
- Good: `host + command + password` succeeds without consulting `~/.ssh`.
- Good: `host + command + private_key` succeeds with explicit key auth.
- Base: `host + command` succeeds by defaulting `username=root` and using discovered `0400` keys from `~/.ssh`.
- Bad: `host` omitted.
- Bad: `command` omitted for tool calls.
- Bad: only non-`0400` files exist under `~/.ssh` when fallback auth is needed.

### 6. Tests Required
- Unit test `ValidateOptions` accepts `host` without explicit auth.
- Unit test `validateExecToolParams` accepts `host + command` without explicit auth.
- Unit test `connectSSH` returns on TCP probe failure before calling auth method construction.
- Unit test fallback discovery filters `~/.ssh` to regular files with permission `0400` only.
- Unit test fallback auth builder returns one auth method per discovered valid private key.
- Regression test cache keys distinguish explicit private-key auth from implicit auto-private-key auth.

### 7. Wrong vs Correct
#### Wrong
- Rejecting tool requests unless `password` or `private_key` is explicitly provided.
- Building auth methods before confirming the target is reachable on TCP.
- Scanning `~/.ssh` without filtering file permissions.
- Loading fallback keys in nondeterministic order.

#### Correct
- Require `host` and `command`, default `username` to `root`, and let auth fallback resolve at connection time.
- Fail fast on TCP unreachability before any auth-building work.
- Only consider `0400` regular files under `~/.ssh` for implicit private-key auth.
- Keep explicit password/private-key behavior unchanged while adding deterministic fallback.