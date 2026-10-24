---
status: historical
---

# Owner compositions construct Audit and Settings persistence once the repository Factory goes

> Historical record of a completed cutover. The rules that still hold
> are in [ADR 0068](0068-principle-owners-compose-themselves-and-the-roots-only-wire.md).

Accepted for [#3972](https://github.com/moto-nrw/project-phoenix/issues/3972),
which blocks [#2743](https://github.com/moto-nrw/project-phoenix/issues/2743),
under [#2580](https://github.com/moto-nrw/project-phoenix/issues/2580), following
the precedent of ADR 0041. The pull request review is where it can still be
rejected; the exception activates only in the policy epoch that review merges.

## Context

#2743 deletes `repositories.Factory` and `NewFactory` and removes the 15
baseline keys of `database/repositories` (`legacy-composition`/`compose`).
One of them targets `github.com/uptrace/bun`, so the keys fall only when the
whole root package is dissolved into the compositions that may own its parts.

Thirteen of the parts have an allowed destination: every other owner's
composition is reachable from `legacy-composition`/`compose` through a
`legacy-composition.to.<owner>-compose` rule. Two have none:

- **Audit Platform persistence** (`database/repositories/audit`,
  `audit-platform`/`postgres`): the retained audit repositories, the booking
  consistency audit and the operator audit ledger. `modules/auditlog/compose`
  may construct it (`audit-platform.native-corrections.compose.postgres`), but
  no composition that consumes it may reach `audit-platform`/`compose`:
  `legacy-composition`, `root-composition`/`compose` and
  `root-composition`/`cli` are all refused, as are the `public` and (for the
  root) `contract` roles.
- **Settings Platform persistence** (`database/repositories/config`,
  `settings-platform`/`postgres`): setting values, setting audit and home
  layout. `services` and `api` may reach `modules/settings/compose`, but the
  Settings composition may not construct its own Postgres adapter. Every
  other owner composition that holds persistence has that rule.

A probe of the two missing rules in a raised epoch fails PR mode with
`architecture policy loosening detected`, because all four role points exist
at the base. The edges are new, so they cannot be converted to exact debt.

The three options #3972 named:

1. **Reviewed exception** for the missing compose permissions.
2. **Owner contracts first.** This does not remove the gap. A contract cannot
   construct an adapter, and the root composition may not reach Audit
   Platform contracts either. Someone still has to build the instances, and
   no composition that consumes them is allowed to.
3. **Re-cut #2743.** The audit and config keys would outlive the Factory. The
   baseline forbids reassigning their issue, so #2743 could not close until
   #2747/#2751 had removed every consumer. This leaves the root package as a
   permanent carrier of exactly the construction it should give up.

The other mechanisms do not fit. A relocation (ADR 0020) renames keys and
removes none. A new package at an existing point gains nothing. A new role
point would pass the anchor check only by mislabelling a composition as
something else.

## Decision

Option 1. Each owner constructs its persistence in its own composition, and
the legacy composition reaches that composition the way it reaches every other
owner's. Permit exactly two production permissions in a reviewed epoch:

| Rule | Scope | Source owner/role | Target owner/role | `issue` |
|---|---|---|---|---|
| `legacy-composition.to.audit-platform-compose` | production | legacy-composition/compose | audit-platform/compose | #2747 |
| `settings-platform.compose.postgres` | production | settings-platform/compose | settings-platform/postgres | none |

The Audit rule is temporary. It exists because `services` still holds the
legacy consumers, it carries the #2747 cleanup issue, and `rules.stale` removes
it when `services` disappears. Its description must not contain the marker
phrases counted by `TestPolicyTemporaryRulesRatchet`. The Settings rule is the
target shape that every owner composition with persistence already has.

No rule is added for `root-composition` and none for any test scope. The root
gets the values it used to read from the Factory through the legacy
composition it already imports (`api -> services`, `cmd -> services`, both
#2750 debt):

- The booking consistency audit needs Enrollment, Care Plan and People
  Directory bindings. The root may reach none of them, and `services` may
  reach all of them, so `services` builds the audit and hands it to the
  scheduler port in `api/scheduler_ports.go` through a function. That
  function returns the scheduler-shaped value and adds no `services.Factory`
  field.
- The settings command repository (`cmd/settings.go`) and the retention
  summaries (`cmd/cleanup_helpers.go`) are served by functions in
  `services/cleanup_composition.go`, as `NewSettingsCommandRepository`
  already is.
- Home layouts keep coming from the settings service that `services` builds.

Tests get no new permission. They reach audit and settings persistence
through the production composition they already exercise, or move into the
owners' own suites (`database/repositories/audit/integration`,
`services/config`), as ADR 0047 did.

### Construction placement

- `modules/auditlog/compose` gains one constructor that builds the audit
  runtime and every retained repository, the booking consistency audit and
  the operator audit ledger. It returns a bundle declared in
  `database/repositories/audit`, whose fields keep their `models/audit`
  interface types. The composition may not import `audit-platform`/`domain`,
  so it does not declare the bundle itself. The People Directory alumni read
  and the Enrollment/Care Plan booking projection arrive as ports declared by
  the Audit composition (consumer-owned), not as foreign imports. The
  adapters that implement those ports from People Directory, Enrollment and
  Care Plan move from `database/repositories` (`student_projection.go`,
  `care_exit_enrollment_bookings.go`) into `services`.
- The operator ledger adapter (`operator_audit_log.go`, `models/audit` to
  `models/platform`) moves to `services`, whose imports of both packages are
  existing #2747 debt.
- `modules/settings/compose` gains the constructors for the setting value,
  setting audit, home layout and command repositories over
  `database/repositories/config`, returning `models/config` interfaces
  (`settings-platform.compose.domain`).
- `services.newFactory` replaces every `repos.<audit or settings field>`
  read with the bundles from these two constructors.

### Exception

The evaluator grants the two permissions only when:

1. the immutable base classifies `database/repositories` exactly as
   `legacy-composition`/`compose` with `module-internal-test` and
   `module-behavior-test`;
2. the candidate's policy epoch is higher than the base's;
3. the scope is `production` and the source/target pair is one of the two
   above.

No other owner, role or scope gains anything. Like ADR 0041 it forgives no
debt, changes no ownership, adds no external dependency and bypasses no
composition or runtime check. Because the policy classifies by owner and role,
the Audit rule also covers the other `legacy-composition`/`compose` packages
(`database/repositories` itself, `database/repositories/studentintegration`).
During the #2743 cut, `database/repositories` may call `modules/auditlog/compose`
for the People Directory's audit seams (note deletion audit, field audit,
consent history) until the slice that moves People Directory construction out
of the root package. At the end of #2743 the root package is gone, so the rule
then serves only `services` (and `database/repositories/studentintegration` if
it is still present). Once `database/repositories` is gone from the base, the
anchor no longer holds and the exception grants nothing. By then the two rules
are part of the base policy.

The cut that registers the rules must also remove the two keys
`database/repositories -> database/repositories/audit` and
`database/repositories -> database/repositories/config`. A rule that no import
uses fails `rules.stale`, so the rules can only land together with the move.

## Verification

`internal/architecture/repository_factory_cutover_test.go`, modelled on
`enrollment_application_cutover_test.go`, covers:

- the two granted pairs in production;
- refusal in `internal_test` and `external_test`;
- refusal of `root-composition` and of a foreign composition as source;
- refusal of `audit-platform`/`public`, `domain` and `application`, and of
  `settings-platform`/`application` and `domain` through the exception;
- refusal when the anchor fails: same epoch, `database/repositories` missing
  from the base or reclassified, or a foreign module path.

`scripts/backend-architecture.sh check` in PR mode reports exactly these
loosenings before the evaluator change, and none after it:
`services`, `database/repositories` and `database/repositories/studentintegration`
to `modules/auditlog/compose`, and `modules/settings/compose` to
`database/repositories/config`.

## Consequences

#2743 can remove all 15 keys and delete `Factory`/`NewFactory` in the same
change. The Audit compose rule falls with #2747. The root composition will
need its own `root-composition.to.audit-platform-compose` when #2747 moves the
audit consumers out of `services`. That is a separate decision, together with
the root's reach to Enrollment and Care Plan, and this ADR does not grant it.
