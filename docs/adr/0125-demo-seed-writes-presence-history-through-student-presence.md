---
status: accepted
---

# The demo seed writes the presence history through Student Presence

Decision of 2026-10-27 for
[#5031](https://github.com/moto-nrw/project-phoenix/issues/5031). It adds no
owner, no policy rule, no route, no command-effect entry and no runtime
exception. It raises the compose LOC budget of
`modules/studentpresence/compose` by exactly 7 LOC (ADR 0057 point 5).

## Context

The Statistik counts a child present on a care day when `active.attendance`
holds a row for that day, and reads room use from `active.visits`. The seeder
wrote both only for the seed day, so a fresh demo school showed attendance
rates near zero for the last 30 days. No API records presence after the
fact, and #5031 rules out opening one: the server's router stays unchanged.

Student Presence owns `active.attendance`, `active.visits` and
`active.groups` (`policy.json`). The dev tools binary may compose it
(`dev-tools.compose.student-presence`, ADR 0060) and open the command
database (`dev-tools.compose.transaction-runtime`).

## Decision

1. Student Presence gains the capability `PresenceHistory`
   (`RecordPresenceHistory`), built by `compose.NewPresenceHistory`. Its
   writer is a type of its own beside the module's `Service`: it shares the
   store and the transaction, but neither the operation observer nor the
   injectable clock, so no production operation gains a caller or a new
   binding. It writes finished care days before today in one tenant
   transaction through the owner's existing store commands for sessions,
   visits and attendance, and rejects a day that is not in the past or a
   visit outside its session or attendance. It is not part of `Owner`; no
   server root composes it. `compose.NewPresenceHistory` and the two type
   aliases of its input are the 7 compose lines this ADR approves.
2. `cmd/devtools seed` composes it on the database of `DB_DSN` and hands it to
   the seeder, which plans the days through the API's calendar and records
   sick and excused days through the existing status-day endpoint. Without
   `DB_DSN`, as in the sidecar of the public demo, the seed skips the history.

## Considered options

1. **A seed-only route behind the local seed guard**, like the billing
   key-date snapshot. Rejected by #5031: it is a production route for
   backdated presence, and the router golden would change.
2. **Writing grants for the demo database role.** Rejected: `phoenix_demo`
   exists in every environment and reads only the standing demo's rows; a
   write grant on presence tables widens a production credential.
3. **A simulated clock.** Rejected: the server stamps presence with its own
   clock, and moving the host clock backwards is not available outside CI.

## Consequences

- `devtools seed` locally and in CI (seed smoke, product screenshots,
  nightly performance) fills the 29 days before the seed day, inside the
  30-day visit retention of the seeded consents.
- The public demo keeps showing only the seed day until its sidecar has a
  write path. That needs its own decision.
