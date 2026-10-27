---
status: accepted
---

# The public demo writes the presence history through a demo-only operator route

Decision of 2026-10-27 for
[#5029](https://github.com/moto-nrw/project-phoenix/issues/5029). It
completes [ADR 0125](0125-demo-seed-writes-presence-history-through-student-presence.md),
which left the public demo without a write path for its past presence. It
adds no owner, no policy rule, no data object and no compose line.

## Context

ADR 0125 lets `devtools seed` write the past attendance and room visits
through Student Presence's `PresenceHistory` on the database of `DB_DSN`. The
sidecar of the public demo (`devtools demo`, `environments/demo.compose.yml`)
has no `DB_DSN`: it holds only `DEMO_DB_DSN` for the role `phoenix_demo`,
which reads presence and writes nothing but the demo's own state. So the
public demo shows the attendance log, the day log, the room history and the
room statistics as switched on (#5105) but empty.

ADR 0125 named the two ways out and rejected both for #5031. For the public
demo one of them is needed.

## Decision

1. The server mounts `POST /operator/schools/{id}/demo-presence-history`
   under `APP_ENV=demo` only (`newDemoPresenceHistoryRoute`). Every other
   environment, production included, builds the operator routes without it.
2. The route sits behind the operator session like every other operator
   route. In the public demo only the sidecar holds the operator credentials;
   visitors sign in as school accounts.
3. Student Presence owns the handler
   (`studentpresencehttp.DemoPresenceHistoryHandler`). It binds the school of
   the path as the tenant and hands the days to the existing
   `PresenceHistory` in one tenant transaction. The writer's checks stay as
   ADR 0125 states them: only finished days before today, visits inside
   their session and attendance, a retry reuses the seeded rows. A request
   carries at most 1000 days and 1 MiB. A day the writer rejects answers 400.
4. The tenant-composite foreign keys refuse a child or kiosk of another
   school on `active.attendance`, a child or session on `active.visits` and
   a room on `active.groups`. The batch runs in one transaction, so a wrong
   school in the path writes nothing.
5. `devtools demo` sets `SeedOptions.PresenceHistoryRoute` under
   `APP_ENV=demo`. Without a bound database writer the seed step then sends
   the planned days in chunks of 500 as the operator. `devtools seed` keeps
   the database writer of ADR 0125.

## Considered options

1. **A write grant for `phoenix_demo`.** Rejected, as in ADR 0125: the role
   exists in every environment, and its policies know only the standing demo
   school, not one school per demo access.
2. **A route on the tenant API.** Rejected: visitors sign in as the demo
   school's administrator and would reach it. The operator route keeps the
   write with the sidecar.
3. **Leaving the public demo without history.** Rejected by #5029: the
   switched-on histories must show data.

## Consequences

- A new demo school fills the 29 days before its seed day, as `devtools seed`
  does locally and in CI, inside the 30-day visit retention of its consents.
- The route is a production code path that only `APP_ENV=demo` mounts. A
  change to that gate must keep it out of every other environment;
  the `demo presence history` case of `TestRouter`
  (`api/operator_router_test.go`) asserts it.
- The router golden and the core-action table stay unchanged: the operator
  routes are outside both, and the golden is built outside the demo.
