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
