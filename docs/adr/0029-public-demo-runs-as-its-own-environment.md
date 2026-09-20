---
status: accepted
---

# The public demo runs as its own environment

The design interview for [#3194](https://github.com/moto-nrw/project-phoenix/issues/3194)
settled this on 2026-09-20. Acceptance records the agreed design, not an
implementation.

The öffentliche Demo gives every Interessent an own, fully editable
Demo-Schule with synthetic data and simulated live activity. It runs as a third
environment `demo` next to staging and production: own compose project, own
PostgreSQL, own frontend image, tenant domain `demo.moto-app.de`. Inside that
environment the seeder and the simulator, which are dev-only everywhere else,
are allowed to run against the internal host `server`. The environment gets its
own `APP_ENV` value `demo`; the dev-only allowlists name it explicitly instead
of the demo pretending to be `development`.

We chose this because a public, unauthenticated flow that creates schools and
sends e-mail must not share a database with the data of real children, and
because reusing seeder and simulator is only safe where no real data exists.
The rule "the seeder is dev-only" therefore still holds for staging and
production.

## Considered options

- **Demo-Schulen as tenants in production.** Least infrastructure. Rejected:
  it needs the dev-only guards opened in production and puts a public
  provisioning endpoint next to customer data.
- **Demo-Schulen in staging.** Rejected: every push to `development` stops
  staging for backup and migration, and staging runs unreleased code.
- **A restored, anonymised production backup as demo data.** Rejected: no
  anonymiser exists, release backups refuse cross-environment restores by
  design, and a residual re-identification risk for children's and health data
  remains. Demo data is synthetic; production may at most contribute aggregated
  behaviour figures to tune the simulator.
- **One shared Demo-Schule for all visitors.** Kept only as the fallback if
  per-request creation is not ready in time. Rejected as the target because
  visitors should be able to change things without seeing each other's changes.

## Consequences

- The frontend needs a third image build, because `NEXT_PUBLIC_*` hostnames are
  inlined at build time.
- `environments/demo.sops.env` must carry exactly the keys of staging and
  production; `scripts/env-check.sh` enforces this.
- The demo is deployed manually via `workflow_dispatch` from `main`, so it
  shows released code and is never restarted unplanned during an event.
- Tenants cannot be hard-deleted today. Expired Demo-Schulen are soft-deleted;
  the demo database may be rebuilt from scratch when that accumulates.
- Timetables, pickup and arrival times exist Monday to Friday only. On
  weekends a Demo-Schule shows presence without a day plan. Accepted for now.
