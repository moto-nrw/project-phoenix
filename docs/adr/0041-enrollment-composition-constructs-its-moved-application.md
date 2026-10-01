---
status: accepted
---

# Enrollment composition may construct the application moved out of services/enrollment

Accepted for [#3562](https://github.com/moto-nrw/project-phoenix/issues/3562)
in the implementing change, following the precedent of ADR 0030 and ADR 0035
to ADR 0038. The pull request review is where it can still be rejected; the
exception activates only in the policy epoch that review merges.

## Context

The #2733 set dissolves `backend/services/enrollment`, classified as
`enrollment`/`application`, group by group into its owners. Group E1 (#3562)
moves phases, form schemas, the phase-response overview, the phase-expiry
warnings, the approved-offering projection, captcha verification and the
parent mail renderers and decision notifications into
`modules/enrollment/internal/application`. The issue classifies the new
package at the same point, `enrollment`/`application`, so the existing
Enrollment application rules carry over by owner and role.

Every other module constructs its application services in its composition
through an `<owner>.compose.application` rule (`care-plan`, `meal-plan`,
`session-end`, ...). Enrollment has none: its composition only reached the
public contract, the Postgres adapter and the tenant runtime. The rule
`enrollment.compose.application` is the target shape, but the PR-mode
strictness check rejects it. A new rule passes only when it is anchored on a
point the candidate creates, and the `enrollment`/`application` point already
exists at the base because `services/enrollment` holds it. Converting the
edge to exact debt is ruled out too: the composition importing its own
application is not debt.

## Decision

Permit the one permission in reviewed policy epochs, under these conditions:

1. The immutable base classifies `services/enrollment` exactly as
   `enrollment`/`application` with the test roles `module-internal-test`
   and `module-behavior-test`.
2. The candidate classifies `modules/enrollment/internal/application` at
   exactly the same point, and its policy epoch is higher than the base's.
3. The source is the `enrollment`/`compose` role, the target the
   `enrollment`/`application` role, the scope `production`. No other owner,
   role or scope gains anything; test suites keep composing the owner
   through `modules/enrollment/enrollmenttest`.

The exception applies to import strictness only. It cannot change data
ownership, forgive a new debt key, permit an external dependency or bypass
composition and runtime checks. Because the policy classifies by owner and
role, the rule would also let the composition import the retained
`services/enrollment`; the composition does not, and the package disappears
with #3565.

Epoch 31 uses it for exactly this rule:

| Scope | Source owner/role | Target owner/role |
|---|---|---|
| production | enrollment/compose | enrollment/application |

The same epoch registers the captcha provider adapter
`modules/enrollment/internal/adapters/turnstile` as `enrollment`/`adapter`
with the rule `enrollment.compose.adapter`, anchored on the point the
candidate creates, and deletes the two parent-portal rules the move emptied:
`parent-portal.compose.enrollment.application` and
`parent-portal.integration-test.enrollment-application`. The guardian portal
now reads care periods and offering history through its own ports.

With `services/enrollment` gone from the base (#3565) the anchor no longer
holds and the exception grants nothing; `enrollment.compose.application` then
stands on its own like every other module's composition rule.

## Verification

`internal/architecture/enrollment_application_cutover_test.go` covers the
granted permission (production only; no lending to the test scopes; no other
Enrollment role; no foreign application), the refused sources (other
Enrollment roles and foreign compositions and adapters) and the anchor (same
epoch, a missing or reclassified retained package, a missing or
reclassified module package and a foreign module all refuse).
