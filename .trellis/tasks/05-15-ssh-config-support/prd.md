# brainstorm: ssh config support

## Goal

Support Linux-style `~/.ssh/config` resolution in the SSH MCP server so callers can pass a host alias and let the server resolve effective `HostName`, `User`, `Port`, and `IdentityFile` values before probing TCP reachability and building SSH auth methods.

## What I already know

* The current SSH connection path is implemented in `internal/ssh/cache.go`.
* The runtime already defaults `username` to `root`, probes TCP reachability, and falls back to `~/.ssh` private keys when no explicit auth is provided.
* To match common Linux SSH behavior, host aliases should resolve through `~/.ssh/config` before connection and auth setup.

## Assumptions (temporary)

* Explicit MCP arguments should override values from `~/.ssh/config`.
* Initial support only needs `HostName`, `User`, `Port`, and `IdentityFile`.
* `ProxyJump`, `ProxyCommand`, and host-key policy changes are out of scope for this task.

## Open Questions

* None.

## Requirements (evolving)

* Resolve `host` through `~/.ssh/config` before connecting.
* Support `HostName`, `User`, `Port`, and `IdentityFile` from SSH config.
* Explicit `username` and `private_key` arguments must override SSH config values.
* TCP precheck must use the resolved target address, including resolved port.
* Cache keys must use resolved target/auth details rather than only the raw alias.

## Acceptance Criteria (evolving)

* [ ] Unit test verifies alias resolution to `HostName`, `User`, `Port`, and `IdentityFile`.
* [ ] Unit test verifies explicit args override SSH config values.
* [ ] Existing SSH package tests continue to pass.
* [ ] `go test ./...` passes.

## Definition of Done (team quality bar)

* SSH config behavior is documented.
* Tests cover the new resolution boundary.
* No regression in previous auth fallback or TCP precheck behavior.

## Out of Scope (explicit)

* `ProxyJump` / `ProxyCommand` support.
* `Match`-specific advanced execution semantics beyond what the parser already resolves.
* Host key verification changes.

## Technical Notes

* Runtime touchpoints are `internal/ssh/cache.go` and `internal/ssh/client.go`.
* The implementation may add a small SSH config parsing dependency.
