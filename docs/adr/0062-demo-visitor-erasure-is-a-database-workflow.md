---
status: accepted
---

# Demo visitor erasure is a database workflow

Decision for [#4048](https://github.com/moto-nrw/project-phoenix/issues/4048)
under [#2580](https://github.com/moto-nrw/project-phoenix/issues/2580). It
applies only to the public-demo environment of ADR 0029.

## Context

The public demo keeps each visitor in a separate Demo-Schule. When its
Demo-Zugang has expired, the school is soft-deleted and the visitor's account
IDs and credentials must be removed from `platform.demo_school_states`. The
seed copied the visitor identity into data owned by People Directory,
Enrollment, Audit Platform, Communication and Delivery Platform. The hourly
demo process runs on the isolated `phoenix_demo` role; no HTTP request or
tenant session exists when it cleans up.

The policy continues to assign every affected table to its existing write
owner. Assigning those tables to Organisation & Tenancy would make the policy
wrong. Calling five Go capabilities would require a privileged, cross-owner
application graph only to perform one atomic cleanup in the separate demo
database; it would also give that graph access to credentials that only the
demo role may read.

## Decision

`platform.erase_closed_demo_visitors` is the single cross-owner database
workflow for this deletion. It may run only as `phoenix_demo`, has a fixed
`pg_catalog` search path, and has no table grants for its caller. It uses the
stored caregiver and parent account IDs and the school-ID history, including
abandoned seed attempts, never a visitor name. It changes only the hidden
Demo-Schulen recorded for the order. It removes sessions, roles, relationship
permissions, credentials and school mappings, then tombstones an account only
when it has no reference outside those recorded schools.

The existing `data_objects` policy entries remain the authority for table
ownership. This workflow is not a new owner and does not permit Organisation
& Tenancy code to write another owner's table. Any further cross-owner
database workflow needs its own architecture decision and a similarly narrow
runtime authorization.

## Consequences

- A repeated seed retains its former visitor account IDs and school IDs in the
  order state until the state row is deleted, so the cleanup can identify its
  abandoned school without matching another visitor with the same name or
  changing another school's data.
- A final failed order retains its school ID until the same cleanup, allowing
  the workflow to restrict itself to the correct Demo-Träger.
- The database function remains the only caller-facing capability for this
  cleanup. Its migration tests verify execution rights and its queue test
  verifies both repeated seeds and equal visitor names.

## Addendum 2026-11-09: the retirement ends the school's sessions (#6305)

Vorgeschlagen am 09.11.2026 für
[#6305](https://github.com/moto-nrw/project-phoenix/issues/6305).

Every request checks the session family its access token names, not the
school (#5194). A session the hourly run left behind therefore kept a hidden
Demo-Schule open until its access token expired, and a school switch could
start from it; the erasure removes only the visitor's own sessions, and a lock
can defer it to a later run.

`platform.retire_expired_demo_schools` now ends every session of a school, of
every account and portal, in the subtransaction that hides it, through
`auth.revoke_school_sessions`, and writes one log line per retired school.
That function belongs to Identity & Access; its `RevokeTokensByTenantID`
(operator soft delete, demo restart) calls the same function, so the database
holds one revocation. It takes its locks in the order of a sign-in: the
accounts of the sessions first, as refresh, switch and logout do, then the
school row, which it holds until the caller commits; only then does it delete.
Every caller ends the sessions before it hides or deletes the school. Tenant
and parent sign-ins and refreshes read the school under a share lock after the
account lock, so none mints a session at a school whose sessions are ending.

The function runs with its caller's rights. Only `phoenix_admin` may execute
it; the demo role reaches it only through the retirement and gets no grant on
`auth.tokens`. A lock in use defers the whole order under the retirement's
`lock_timeout`, school and sessions unchanged. This is no new cross-owner
workflow: it is a step of this one, limited to the school the run hides. The
retirement writes no `token_revoked` audit event, because that evidence
carries a keyed family fingerprint only the application can compute; its log
line names the school, the order and the number of ended sessions.
