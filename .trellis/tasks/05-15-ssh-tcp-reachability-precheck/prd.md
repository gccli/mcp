# brainstorm: ssh tcp reachability precheck

## Goal

Ensure the SSH connection flow checks raw TCP reachability before attempting authentication work, so network failures surface early and are not reported as authentication-related connection failures.

## What I already know

* `connectSSH` in `internal/ssh/cache.go` currently builds auth methods and then calls `gossh.Dial`.
* This means key discovery and auth preparation can run even when the target host is unreachable on TCP.
* The user wants the TCP connectivity check to happen before any subsequent authentication attempt.

## Assumptions (temporary)

* The precheck targets the same `host:22` endpoint used by the SSH connection.
* Existing auth behavior should remain unchanged once the TCP probe succeeds.
* The desired outcome is clearer network failure handling and skipping unnecessary auth work when the network is down.

## Open Questions

* None; the requested behavior is local and specific.

## Requirements (evolving)

* `connectSSH` must probe TCP reachability before building SSH auth methods.
* When the TCP probe fails, `connectSSH` must return a network-oriented error immediately.
* Authentication building must not run after a failed TCP probe.
* Existing password/private-key authentication behavior must stay unchanged after a successful probe.

## Acceptance Criteria (evolving)

* [ ] A focused unit test proves TCP probe failure returns before auth method construction.
* [ ] Existing `internal/ssh` tests still pass.
* [ ] Full `go test ./...` still passes.

## Definition of Done (team quality bar)

* Tests updated first for the new sequencing behavior.
* No regression in explicit auth flows.
* Error behavior is documented in spec if it changes externally.

## Out of Scope (explicit)

* Changing SSH port handling.
* Changing host key verification policy.
* Retrying or backoff for network probe failures.

## Technical Notes

* The narrow change surface is `internal/ssh/cache.go` and `internal/ssh/cache_test.go`.
* Testability may require injectable dial/auth builder hooks local to the package.
