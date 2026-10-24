---
status: accepted
---

# The guardian portal HTTP composition reaches the standing HTTP runtimes

Decision for [#3421](https://github.com/moto-nrw/project-phoenix/issues/3421),
under [#2580](https://github.com/moto-nrw/project-phoenix/issues/2580). The user
chose this path for the ticket's second stage. The implementation remains
subject to PR review.

## Context

`modules/careplan/inbound/parent` (`inbound-parent`/`http`) is the guardian
portal's HTTP adapter. Its compatibility permissions became exact debt in
#3421 stage 1; 20 `legacy.jsonl` entries remained. Seventeen of them name
implementation targets the package can leave: the calendar wrapper, retained
models, the ORM, the legacy composition and the settings application. The
other three name runtime points every HTTP adapter of the backend reaches:

| Target | Why the routes need it |
| --- | --- |
| `inbound-common/http` (`api/common`) | Error rendering, request binding, portal gates and the guardian profile response; the shared HTTP runtime of ADR 0046 |
| `identity-access/adapter` (`legacy/jwt`) | The parent-scope session: authenticator, claims and the unauthorized sentinel; the permanent session adapter of ADR 0031 |
| `tenant-runtime/public` (`tenant`) | The enrollment routes' tenant and administrative transactions, opened on the runtime the request carries |

Every other HTTP point holds these reaches as standing rules (for example
`enrollment.http.inbound-common`, `people-directory.http.identity-adapter`,
`enrollment.http.tenant-runtime`). For `inbound-parent` they exist only as
debt, because its rules were compatibility permissions when it moved. Its
handler suites likewise need the session adapter to put claims on requests
and mint tokens, as `people-directory.adapter-test.identity-adapter` and
`inbound-common.adapter-test.identity-access-adapter` allow their suites.
PR mode rejects any new rule whose selector is not anchored to a
candidate-created point, so these debts could never fall.

## Decision

Policy epoch 39 adds exactly four rules for `inbound-parent`:

| Rule | Source role | Target | Scopes |
| --- | --- | --- | --- |
| `inbound-parent.http.inbound-common` | `http` | `inbound-common/http` | production |
| `inbound-parent.http.identity-adapter` | `http` | `identity-access/adapter` | production |
| `inbound-parent.http.tenant-runtime` | `http` | `tenant-runtime/public` | production |
| `inbound-parent.adapter-test.identity-adapter` | `adapter-test` | `identity-access/adapter` | internal_test, external_test |

These are standing target rules, not compatibility allowances. The
epoch-gated comparison (`reviewedParentHTTPRule`) accepts only this finite
selector set in an increased epoch. Another source owner, an owner kind,
another role or target, an external class, `same_owner`, or a wider scope
stays a loosening.

The other 17 debts fall in code, without a rule:

- Calendar dates come from `sharedkernel/calendar`.
- Retained People Directory, Care Plan and Enrollment values the workflow
  hands back are named through the workflow port's vocabulary
  (`workflows/parentportal`), declared once in the flow package that returns
  them, as the port already names its other results.
- The enrollment profile port returns the HTTP input values the response
  renders; the root translates People Directory's loader result.
- The locale check uses Delivery's notification copy, the owner of
  `localization`.
- The enrollment routes open transactions through `tenant.WithinTenant` and
  `tenant.WithinAdmin`; the resource no longer holds a `*bun.DB`.
- The handler suites take setting keys from Settings Platform's public
  contract, double the workflow's settings through the port's name, compose
  through `api/testutil` instead of `services`, and rely on the fixture
  package's pinned JWT configuration instead of writing viper.

No route, status code, error body, authorization check, tenant scoping or
transaction boundary changes. No owner, table, projection grant or package
classification changes.

## Verification and rollback

The package's route suites, the workflow suites, the calendar end-to-end
flow, the production router golden and the architecture checks are the
acceptance surface. Positive and negative selector tests keep the policy
change bounded, and a check against the unchanged epoch reports all four
rules as loosenings. Revert the code-only change if a hidden caller appears;
no data migration or cleanup is required.
