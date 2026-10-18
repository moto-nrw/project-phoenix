---
status: accepted
---

# Unused account erasure is a database workflow

Decision for [#4043](https://github.com/moto-nrw/project-phoenix/issues/4043)
under [#2580](https://github.com/moto-nrw/project-phoenix/issues/2580). It is
the further cross-owner database workflow that
[ADR 0062](0062-demo-visitor-erasure-is-a-database-workflow.md) requires a
decision of its own for.

## Context

An account (`auth.accounts`) serves several schools. Whether a school still
needs it is recorded in tables of four owners: Identity & Access (school
access, roles, invitations, demo accesses), People Directory (persons and
parent contacts) and Enrollment (enrollment requests). The nightly cleanup
runs per school in a tenant transaction and cannot see the other schools.

Hard deletion is not possible: 19 foreign keys without an action and 4 with
`RESTRICT` point at an account (messages, protocols, grade transitions,
announcements). They are records of the school, with retention windows of
their own, and must keep their reference.

## Decision

`auth.erase_unused_accounts()` is the single database workflow that erases
accounts. The hourly token cleanup calls it through the Identity & Access
capability `EraseUnusedAccounts` in its administrative transaction. The
function runs as its owner with a fixed `pg_catalog` search path; only
`phoenix_admin` may call it.

An account is unused when no school needs it: no pending school access, no
active school access unless guardian is its only role there (an access
without any role counts as used), no person, no parent contact, no
enrollment request, no open invitation for its address and no demo access.
Such an account loses every row that would go with it through
`ON DELETE CASCADE` (sessions, credentials, roles, school mappings, profile,
preferences, push subscriptions, read markers, its import protocol); a
migration test keeps that list equal to the tables the retention guard
classifies as going with the account. It keeps its row as a tombstone:
placeholder address, no username, no password, inactive,
`personal_data_erased_at`. The function locks its candidates with
`SKIP LOCKED` and checks them again with a fresh snapshot before it erases.

The workflow only reads the tables of People Directory and Enrollment. It
writes Identity & Access tables and, in other owners' tables, only the rows
that would cascade from the account anyway. The `data_objects` entries of the policy remain the
authority for table ownership; this workflow is no new owner.

## Consequences

- No retention window applies: the windows of the data that named the
  account (staff after their exit, children, parent contacts) have already
  run. A school that still needs the account keeps it.
- Messages, requests, protocols and other records of the school keep the
  account ID until their own window ends; the account then names nobody.
- An account with access to another school keeps an empty guardian role at a
  school whose parent contact was deleted. It names no child there.
- The migration tests verify the conditions, the execution right and the
  rollback; the root test runs the capability in an administrative
  transaction.
