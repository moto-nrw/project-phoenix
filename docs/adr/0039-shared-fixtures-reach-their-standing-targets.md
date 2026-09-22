---
status: accepted
---

# Shared fixtures reach their standing targets

Accepted for [#2748](https://github.com/moto-nrw/project-phoenix/issues/2748)
in the policy epoch that registers the rules, following ADR 0014. The pull
request review is where it can still be rejected; the exception activates
only in the epoch that review merges.

## Context

The shared test support — the fixture catalog `test`, the API helpers
`api/testutil`, the database lifecycle `internal/testdb` with its bootstrap
and sweep commands, the end-to-end suites under `test/e2e` and the retained
mocks in `configtest` and `userstest` — is one owner, `test-support`, that
every module's tests compose through. Its remaining 51 baseline entries after
the first cut of #2748 fall into two kinds.

Ten name things the fixtures need by construction and that no owner is
dissolving: the fixture catalog opens the test database lifecycle and the
API helpers build on the catalog; rows are created inside a tenant through
the tenant runtime; DATE columns are mandated to use the calendar-date type in
`internal/timezone`; tokens carry the permission constants the routes check.
Individual owners already hold each of these grants for their own test
support (`communication`, `inbound-timetable`, `care-plan`,
`student-presence`, `settings-platform`). Withholding them from the shared
fixtures only produced the first cut's wire strings for permissions: a
visible key traded for a silent duplicate of the constant.

The other forty-one name packages other carriers are dissolving: `models/*`
(#2729, #2742, #2733), `services/users` (#2728),
`database/repositories/*` (#2727), the settings service and models, the
device authenticator and the root composition (#2747, #2750), and the
timetable rows the flows still insert (#3424). The fixtures import them
because they still exist, not because a test role needs a special right. A
rule for any of them would hide the debt in the policy instead of shrinking
it, and would have to be withdrawn again.

## Decision

Permit, in a reviewed epoch, exactly four grants for the shared fixture
owner, each as a rule with `source_owner` `test-support` and one of its two
roles: `test-support` in the production scope, `e2e-test` in the test
scopes. A rule without a role would overlap the owner-agnostic test-role
rules and is refused with them.

| Target | Why | Keys |
|---|---|---|
| `test-support/test-support` | own tooling: `internal/testdb/cmd/*` → `internal/testdb`, `test` → `internal/testdb`, `api/testutil` → `test` | 4 |
| `tenant-runtime/public` | rows are created inside a tenant; isolation is verified through the runtime | 3 |
| `legacy-shared/domain` | the mandated calendar-date type | 3 |
| `security-runtime/contract` | permission constants instead of wire strings | 0, restores the constants |

The comparison in `backend/internal/architecture/policy_compare.go` admits
this shape, and only this shape, under a reviewed epoch. Other source owners,
owner kinds, external classes, the implementation roles of the granted
owners and every other target stay under the ordinary loosening guards. No
rule is granted for anything a carrier is dissolving; those keys stay exact
debt on #2748 and fall with the carriers.

This decision changes no runtime write owner, table, HTTP route, status code
or error string.

## Policy registration

Epoch 24 registers `shared-fixtures.own-tooling`,
`shared-fixtures.tenant-runtime-public`,
`shared-fixtures.e2e.tenant-runtime-public`, `shared-fixtures.calendar-date`,
`shared-fixtures.e2e.calendar-date` and
`shared-fixtures.e2e.permission-constants`, removes the ten baseline entries
they resolve, and returns the calendar and timetable end-to-end suites to the
permission constants.

## Verification

`internal/architecture/shared_fixture_rule_test.go` covers the shape (each
grant accepted; a missing role, a role in the wrong scope, another owner, an
owner kind, a dissolving domain, an implementation role of a granted owner,
the authorization application, an external class and the same-owner flag all
refused), the epoch anchor (the repository policy passes against its own
base only with the epoch increase) and the guard (a widened rule of any of
those shapes is still reported in the reviewed epoch).
