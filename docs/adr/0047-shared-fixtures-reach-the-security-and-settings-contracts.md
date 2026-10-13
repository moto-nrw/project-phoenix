---
status: accepted
---

# Shared fixtures reach the security and settings contracts

Accepted for [#3944](https://github.com/moto-nrw/project-phoenix/issues/3944)
(part of [#2748](https://github.com/moto-nrw/project-phoenix/issues/2748)) in
the policy epoch that registers the rules, following ADR 0039. The pull
request review is where it can still be rejected; the exception activates
only in the epoch that review merges.

## Context

ADR 0039 granted the shared fixture owner `test-support` four standing
targets and left the rest of its imports as exact debt on #2748, expecting it
to fall with the carriers that dissolve the imported packages. That
expectation was wrong for most of them. `auth/authorize`, `services/config`,
`database/repositories/config`, `database/repositories/audit` and
`models/config` are implementation roles of owners that keep them; no carrier
deletes them. The fixtures leave them only by reaching the owners' contracts
or test support instead.

`modules/securityruntime` (`security-runtime/public`) serves the guardian role
presets and student access decisions to production consumers. The fixtures
apply the same presets when they create guardian links and decide
`CanUpdateStudent` in the request-review policy double. `modules/settings`
(`settings-platform/public`) is the Settings Platform contract and names the
setting keys and their values.

Three internal tests of the fixture package drive owner implementations
directly and do not belong there:

- `TestOperationalOverviewScopeIsTenantScoped` builds the real settings
  service from its repository to prove that one school's overview scope does
  not reach another. `modules/settings` has no constructor for that service,
  and the settings suite in `services/config` already builds it the same way
  for the neighbouring attendance-scope test.
- `TestOperationalOverviewNeverCrossesTenants` asserts the live-group query
  of Student Presence and only stores the overview scope as arrangement.
- `booking_consistency_audit_test.go` composes the audit repository with the
  Enrollment and Care Plan test support. The Audit Platform already has an
  integration suite, `database/repositories/audit/integration`.

## Decision

Permit, in a reviewed epoch, two more targets for the shared fixture owner
and three pinned rules for the owner suites that receive the moved tests.
Epoch 35 registers five rules under them.

| Rule | Source | Scope | Target | Why |
|---|---|---|---|---|
| `shared-fixtures.security-runtime-public` | `test-support`/`test-support` | production | `security-runtime/public` | guardian presets and `CanUpdateStudent` |
| `shared-fixtures.e2e.settings-platform-public` | `test-support`/`e2e-test` | internal_test | `settings-platform/public` | the live-group isolation test names the overview key and values |
| `settings-platform.internal-test.security-runtime-public` | `settings-platform`/`module-internal-test` | internal_test | `security-runtime/public` | the moved scope test decides through `CanViewOperationalOverview` |
| `audit-platform.adapter-test.enrollment-support` | `audit-platform`/`adapter-test` | external_test | `enrollment/test-support` | the moved booking audit suite composes Enrollment |
| `audit-platform.adapter-test.care-plan-support` | `audit-platform`/`adapter-test` | external_test | `care-plan/test-support` | the moved booking audit suite composes Care Plan |

`security-runtime/public` and `settings-platform/public` join the shared
fixture targets of ADR 0039, under the same shape: the `test-support` role in
production or the `e2e-test` role in the test scopes. The other three rules
are pinned exactly; a wider scope, another source role or another target is
still a loosening.

`modules/securityruntime` gains aliases for the remaining guardian role
presets and the three guardian permissions the fixtures name, and no
behaviour. The fixtures apply a preset through the existing
`StudentGuardianRolePreset` and read a stored permission through
`GuardianPermissionGranted`; the contract exposes no guardian-link interface,
because that interface carries the stored permission map.

`configtest.Mock` stays on `services/config`. It implements the full settings
service, whose schema carries the Settings Platform's `models/config` field,
policy and validation types; a contract alias for it would expose them
(`contracts.internal-model`). Its two keys stay on #2748 until the schema
leaves `models/config` or the double moves to the owner.

The moved tests keep their assertions. The scope test runs in the
`services/config` internal suite with the real settings service and the
security contract. The live-group test stays in `test` and stores the override
through the shared `SetTestTenantSetting` fixture. The booking audit tests
move into the Audit integration suite with the composition of their
repository; the scenarios they assert stay in the fixture package as exported
`VerifyBookingConsistencyAudit*` functions, because they arrange Enrollment
and Audit rows the fixtures already write.

Seven keys fall. The ten keys that stay on #2748 name packages that are
either dissolved by their carrier (`api/testutil -> services`: #2747/#2751;
`modules/identityaccess/legacy/jwt`: #3487), wait for a decision on who
composes the tenant runtime (`database`, the `models/audit` context
adapters: #2750), wait for the port owner of a type (`models/enrollment`:
#2747, `models/platform`: the outbox port owner), or wait for People
Directory contract vocabulary (`models/users`, both scopes), or wait for a
settings schema without model types (`configtest -> services/config`, both
scopes).

This decision changes no runtime write owner, table, HTTP route, status code
or error string.

## Policy registration

Epoch 35 registers the five rules above, removes the seven baseline entries
they resolve, and deletes the three rules only the moved booking audit test
used: `test-support.e2e.enrollment-support`, `test-support.to.care-plan-public`
and `test-support.to.care-plan-support`.

## Verification

`internal/architecture/shared_fixture_rule_test.go` extends the shape cases to
the two new targets (each accepted in its role and scope; the settings
application and repository refused). `TestReviewedOwnerSuiteRuleShape` pins
the three owner-suite rules (each accepted; a wider scope, production, another
role, another source owner, another target, an owner-agnostic source and an
owner-kind target refused). The epoch anchor fails every new rule without the
epoch increase, and `TestSharedFixtureEpochGrantsNothingElse` refuses a
widened shared-fixture or owner-suite rule in the reviewed epoch.

## Addendum (#3965)

The `configtest` keys fell without either exit named above. No consumer read
the schema, so every consumer that took the full settings service declares a
narrow port of its own, each test package keeps a local func-field double with
only the resolvers those ports need, and `configtest` is deleted. No rule was
needed. The ten keys above become eight.

## Addendum (dependency cut of 2026-10-12)

The `database` key and the context-adapter part of the `models/audit` key no
longer wait for #2750. Waiting closed a cycle: #2748 waited on #2750, #2750 is
blocked by #2749, and #2749 by #2748. The fixtures and the composition root
build the same Postgres tenant runtime (unit of work, lock-wait hook, retry
classification, detacher and the audit context adapters), so
[#3970](https://github.com/moto-nrw/project-phoenix/issues/3970) gives that
bootstrap an owner seam and moves `test` onto it first; #2750 later moves
`api` and `cmd` onto the same seam. The edge #2749 ← #2748 stays. If the seam
needs a fixture target beyond `tenant-runtime/public`, #3970 registers it in a
reviewed epoch with its own addendum here. The rest of the audit vocabulary and
`models/platform` fall through #3967, the `models/users` keys through #3966,
where the fixtures return their own row types.

## Addendum (#3970)

The bootstrap needed a target beyond `tenant-runtime/public`. `tenant` is the
runtime's public seam and knows no adapter; offering a constructor port there
would still leave the fixtures to import the adapter that fills it. The
bootstrap therefore became a composition point of the Transaction Runtime,
`modules/transactionruntime/compose`. `NewTenantRuntime` builds from a
`*bun.DB` the `tenant.UnitOfWork` the fixtures and the composition roots run
under: unit of work, retry classification, advisory locks, the repository
detacher and context adapters, and the Audit context adapters, which it
reaches through `modules/auditlog` (`audit-platform/public`), not
`models/audit`. `NewDemoTenantRuntime` is the variant of the standing demo,
`ObserveLockWaits` attaches the lock-wait hook, and `OpenPool` and
`OpenServePool` open the pools. The Audit detacher the roots install is not
repeated: detaching hands the context adapters nil, which already masks the
Audit transaction.

Policy epoch 37 registers `shared-fixtures.transaction-runtime-compose`. The
bootstrap joins the shared fixture targets in a narrower shape than the
targets above: only the fixture package's `test-support` role in production
may reach it (`sharedFixtureBootstrapTargets`); the end-to-end suites take the
runtime from the fixtures. The seam's own rules, to its Postgres adapter, the
tenant runtime, the Audit contract and the ORM, and the rule for its
integration suite, are anchored to the new package and registered in the same
epoch.

`test` builds its runtimes and pools through the bootstrap, and its contexts
carry the Audit tenant through the runtime they are bound to instead of
setting it themselves. `test -> database` falls. `test -> models/audit` stays
for model types only, which #3967 removes. The two assertions that the fixture
contexts carry the Audit tenant moved to the Audit integration suite, which
reads the Audit context directly.

The composition roots move onto the seam in the same change, because the
dead-code gate admits no production function that only tests reach. `api`
builds its serve pool, lock-wait hook, tenant runtime and demo runtime there,
`cmd` its cleanup and parent-seeding runtimes and the pools of `seed-parents`
and `backfill`, under two rules anchored to the new package
(`root-composition.to.transaction-runtime-compose`,
`root-composition.cli.transaction-runtime-compose`).
`services.BindTenantRuntime` and its two observer bridges are deleted. `api ->
database` and `cmd -> database` stay on #2750 for the other database functions
the roots call. The Audit readers that only tests need live in
`_test_helpers.go` files. No runtime write owner, table, route, status code or
error string changes.
