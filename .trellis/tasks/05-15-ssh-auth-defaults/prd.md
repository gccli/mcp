# brainstorm: ssh auth defaults

## Goal

Adjust the SSH MCP server authentication behavior so callers only need to provide host. Username becomes optional and defaults to root. If password is empty and private_key is not provided, the server should automatically iterate candidate private key files under ~/.ssh that have permission mode 0400 until authentication succeeds.

## What I already know

* SSH tool parameter validation currently rejects requests unless password or private_key is provided.
* Username already defaults to root in the conversion layer, but the validation layer still enforces explicit credentials.
* SSH connection setup only loads a private key when opts.PrivateKey is explicitly set.
* The likely implementation surface is internal/ssh/options.go, internal/ssh/server.go, and internal/ssh/cache.go.

## Assumptions (temporary)

* "轮询 ~/.ssh/ 目录下权限为 0o400 的 private_key 文件" means trying eligible private key files one by one until one works.
* The fallback should only trigger when both password and private_key are absent.
* Existing password and explicit private_key behavior should remain unchanged.

## Open Questions

* None at the moment; the requested behavior is specific enough to implement directly.

## Requirements (evolving)

* The exec-related SSH MCP capabilities must only require host from the caller for authentication fields.
* username must be optional and default to root when omitted.
* password must be optional.
* private_key must be optional.
* When password is empty and private_key is not provided, the program must scan ~/.ssh and only consider files whose permission bits are exactly 0400.
* The program must try eligible private keys until one authenticates successfully or all candidates fail.
* Validation and runtime behavior must be aligned so requests are not rejected before fallback key discovery runs.

## Acceptance Criteria (evolving)

* [ ] Requests with only host and command are accepted by validation.
* [ ] Requests without username use root.
* [ ] Requests without password/private_key attempt fallback key discovery from ~/.ssh.
* [ ] Only 0400 private key files are considered during fallback.
* [ ] Existing password and explicit private_key authentication flows still work.
* [ ] Unit tests cover validation, fallback discovery, and authentication selection behavior.

## Definition of Done (team quality bar)

* Tests added or updated for new fallback behavior.
* go test ./... passes.
* No existing explicit-auth behavior regresses.

## Out of Scope (explicit)

* Changing host key verification policy.
* Supporting passphrase-protected private keys unless already supported.
* Changing the public MCP tool names or command semantics beyond auth defaults.

## Technical Notes

* Existing validation failures are in internal/ssh/options.go and internal/ssh/server.go.
* SSH auth method construction lives in internal/ssh/cache.go.
* Cache keys currently include auth mode and may need to account for fallback selection behavior.
