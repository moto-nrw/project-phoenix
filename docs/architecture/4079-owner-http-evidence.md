# Owner HTTP cutover evidence

> **Historical record.** State at `ae21d934d7` (2026-10-18). #2580 and #4069 are closed;
> counts, budgets and open issues below describe that state, not today.
> Final state: [`backend/architecture/WORKPLAN.md`](../../backend/architecture/WORKPLAN.md#endstand).

Base: `c093a60605bda8abd07f2ea803e64acf89740419` (merged #4318).
Branch created from fresh `origin/development` after verifying that merge.

## Contract preservation

- All 14 original exact #4079 edges are absent, with no replacement baseline entries.
- Root composition: 1,644 → 1,644 against integrated development (1,650 → 1,650 against the original #4318 base) across production and test scopes.
- Original student-suite inventory: 671 tests, 3,437 assertions. AST call
  comparison is identical after only native qualifiers and fixture-name
  normalization. The manual-status notification subtest follows Care Plan;
  the planned-status notification subtest follows Student Presence.
- `TestFullProductionRouterGolden` passed. Route and middleware goldens are byte-identical to the #4318 base; normalized diff is empty with zero substitutions.
  Route SHA-256: `bd2a1e176eb9f77f5f8170294440134edc0cc3e560a0cc4ece052a1450136b61`.
  Middleware SHA-256: `8aac8ecab2b289d4672a132a762e8dea6b5f6d2400495c262c178cf322eece45`.
- Architecture check passed: 76 → 62 legacy keys; root composition 1,644 unchanged. Full lint passed with zero issues. The required no-fast changed-package gate passed across 178 packages. Backend quality passed: production build, source ratchets, migration validation/collisions, dead-code guards and license policy.
- No app servers or production requests are required.


Development was integrated locally through `a70bde6878`; register-only conflicts
were resolved as base plus both deltas, retaining all unrelated retirements.

## Policy rules

| Added rule | Source package | Target | Scopes |
|---|---|---|---|
| `group-live-view.http.group-live-view.public` | `modules/grouplive/http` | `group-live-view/public` | production |
| `group-live-view.http.tenant-runtime.http` | `modules/grouplive/http` | `api/common` | production |
| `group-live-view.http.security-runtime.contract` | `modules/grouplive/http` | `security-runtime/contract` | production |
| `calendar-view.http.calendar-view.public` | `modules/supervisiondashboard/http` | `calendar-view/public` | production |
| `calendar-view.http.tenant-runtime.http` | `modules/supervisiondashboard/http` | `api/common` | production |
| `open-room-move.http.open-room-move.public` | `workflows/openroommove/http` | `open-room-move/public` | production |
| `open-room-move.http.tenant-runtime.http` | `workflows/openroommove/http` | `api/common` | production |
| `open-room-move.http.student-presence.public` | `workflows/openroommove/http` | `student-presence/public` | production |
| `care-plan.http.tenant-runtime.http` | `modules/careplan/http` | `api/common` | production |
| `care-plan.http.care-plan.contract` | `modules/careplan/http` | `care-plan/contract` | production |
| `care-plan.http.identity-access.http` | `modules/careplan/http` | `modules/identityaccess/sessiontoken` | production |
| `care-plan.http.tenant-runtime.public` | `modules/careplan/http` | `tenant-runtime/public` | production |
| `care-plan.http.security-runtime.public` | `modules/careplan/http` | `security-runtime/public` | production |
| `care-plan.http.delivery-platform.public` | `modules/careplan/http` | `delivery-platform/public` | production |
| `care-plan.http.people-directory.public` | `modules/careplan/http` | `people-directory/public` | production |
| `care-plan.http.security-runtime.contract` | `modules/careplan/http` | `security-runtime/contract` | production |
| `care-plan.http.people-directory.contract` | `modules/careplan/http` | `people-directory/contract` | production |
| `care-plan.http.settings-platform.public` | `modules/careplan/http` | `settings-platform/public` | production |
| `care-plan.http.identity-access.contract` | `modules/careplan/http` | `identity-access/contract` | production |
| `presence-legacy-view.http.document-rendering.contract` | `modules/presenceprojection/http` | `document-rendering/contract` | production |
| `presence-legacy-view.http.people-directory.public` | `modules/presenceprojection/http` | `people-directory/public` | production |
| `presence-legacy-view.http.people-directory.contract` | `modules/presenceprojection/http` | `people-directory/contract` | production |
| `presence-legacy-view.http.student-presence.public` | `modules/presenceprojection/http` | `student-presence/public` | production |
| `presence-legacy-view.http.tenant-runtime.public` | `modules/presenceprojection/http` | `tenant-runtime/public` | production |
| `presence-legacy-view.http.tenant-runtime.http` | `modules/presenceprojection/http` | `api/common` | production |
| `presence-legacy-view.http.care-plan.contract` | `modules/presenceprojection/http` | `care-plan/contract` | production |
| `presence-legacy-view.http.identity-access.http` | `modules/presenceprojection/http` | `modules/identityaccess/sessiontoken` | production |
| `presence-legacy-view.http.security-runtime.public` | `modules/presenceprojection/http` | `security-runtime/public` | production |
| `presence-legacy-view.http.care-plan.public` | `modules/presenceprojection/http` | `care-plan/public` | production |
| `presence-legacy-view.http.security-runtime.contract` | `modules/presenceprojection/http` | `security-runtime/contract` | production |
| `presence-legacy-view.http.settings-platform.public` | `modules/presenceprojection/http` | `settings-platform/public` | production |
| `student-presence.http.people-directory.public` | `modules/studentpresence/http` | `people-directory/public` | production |
| `student-presence.http.identity-access.http` | `modules/studentpresence/http` | `modules/identityaccess/sessiontoken` | production |
| `student-presence.http.care-plan.contract` | `modules/studentpresence/http` | `care-plan/contract` | production |
| `student-presence.http.tenant-runtime.public` | `modules/studentpresence/http` | `tenant-runtime/public` | production |
| `student-presence.http.delivery-platform.public` | `modules/studentpresence/http` | `delivery-platform/public` | production |
| `student-presence.http.security-runtime.contract` | `modules/studentpresence/http` | `security-runtime/contract` | production |
| `student-presence.http.security-runtime.public` | `modules/studentpresence/http` | `security-runtime/public` | production |
| `student-presence.http.settings-platform.public` | `modules/studentpresence/http` | `settings-platform/public` | production |
| `request-review-view.http.tenant-runtime.http` | `modules/requestreview/http` | `api/common` | production |
| `request-review-view.http.security-runtime.contract` | `modules/requestreview/http` | `security-runtime/contract` | production |
| `request-review-view.http.identity-access.contract` | `modules/requestreview/http` | `identity-access/contract` | production |
| `request-review-view.http.identity-access.http` | `modules/requestreview/http` | `modules/identityaccess/sessiontoken` | production |
| `request-review-view.http.security-runtime.public` | `modules/requestreview/http` | `security-runtime/public` | production |
| `request-review-view.http.people-directory.public` | `modules/requestreview/http` | `people-directory/public` | production |
| `request-review-view.http.care-plan.public` | `modules/requestreview/http` | `care-plan/public` | production |
| `request-review-view.http.care-plan.contract` | `modules/requestreview/http` | `care-plan/contract` | production |
| `request-review-view.http.request-review-view.public` | `modules/requestreview/http` | `request-review-view/public` | production |
| `student-deletion.http.tenant-runtime.http` | `workflows/studentdeletion/http` | `api/common` | production |
| `student-deletion.http.care-plan.public` | `workflows/studentdeletion/http` | `care-plan/public` | production |
| `student-deletion.http.identity-access.http` | `workflows/studentdeletion/http` | `modules/identityaccess/sessiontoken` | production |
| `student-deletion.http.tenant-runtime.public` | `workflows/studentdeletion/http` | `tenant-runtime/public` | production |
| `student-deletion.http.student-deletion.public` | `workflows/studentdeletion/http` | `student-deletion/public` | production |
| `student-deletion.http.people-directory.public` | `workflows/studentdeletion/http` | `people-directory/public` | production |
| `student-deletion.http.security-runtime.public` | `workflows/studentdeletion/http` | `security-runtime/public` | production |
| `student-deletion.http.security-runtime.contract` | `workflows/studentdeletion/http` | `security-runtime/contract` | production |
| `care-plan.adapter-test.tenant-runtime.http.http` | `modules/careplan/http` | `api/common` | external_test, internal_test |
| `care-plan.adapter-test.identity-access.http.http` | `modules/careplan/http` | `modules/identityaccess/sessiontoken` | external_test, internal_test |
| `group-live-view.adapter-test.people-directory.public.http` | `modules/grouplive/http` | `people-directory/public` | external_test |
| `presence-legacy-view.adapter-test.tenant-runtime.http.http` | `modules/presenceprojection/http` | `api/common` | external_test, internal_test |
| `presence-legacy-view.adapter-test.care-plan.public.http` | `modules/presenceprojection/http` | `care-plan/public` | external_test, internal_test |
| `presence-legacy-view.adapter-test.care-plan.contract.http` | `modules/presenceprojection/http` | `care-plan/contract` | external_test, internal_test |
| `presence-legacy-view.adapter-test.enrollment.public.http` | `modules/presenceprojection/http` | `enrollment/public` | external_test |
| `presence-legacy-view.adapter-test.identity-access.http.http` | `modules/presenceprojection/http` | `modules/identityaccess/sessiontoken` | external_test |
| `presence-legacy-view.adapter-test.people-directory.public.http` | `modules/presenceprojection/http` | `people-directory/public` | external_test, internal_test |
| `presence-legacy-view.adapter-test.people-directory.contract.http` | `modules/presenceprojection/http` | `people-directory/contract` | external_test, internal_test |
| `presence-legacy-view.adapter-test.student-presence.public.http` | `modules/presenceprojection/http` | `student-presence/public` | external_test, internal_test |
| `request-review-view.adapter-test.tenant-runtime.http.http` | `modules/requestreview/http` | `api/common` | external_test |
| `request-review-view.adapter-test.care-plan.public.http` | `modules/requestreview/http` | `care-plan/public` | external_test, internal_test |
| `request-review-view.adapter-test.care-plan.contract.http` | `modules/requestreview/http` | `care-plan/contract` | external_test, internal_test |
| `request-review-view.adapter-test.enrollment.public.http` | `modules/requestreview/http` | `enrollment/public` | external_test |
| `request-review-view.adapter-test.identity-access.http.http` | `modules/requestreview/http` | `modules/identityaccess/sessiontoken` | external_test, internal_test |
| `student-presence.adapter-test.identity-access.http.http` | `modules/studentpresence/http` | `modules/identityaccess/sessiontoken` | external_test, internal_test |
| `student-deletion.adapter-test.tenant-runtime.http.http` | `workflows/studentdeletion/http` | `api/common` | external_test, internal_test |
| `student-deletion.adapter-test.identity-access.http.http` | `workflows/studentdeletion/http` | `modules/identityaccess/sessiontoken` | external_test |
| `presence-legacy-view.adapter-test.timetable-activities.public.http` | `modules/presenceprojection/http` | `timetable-activities/public` | internal_test |
| `request-review-view.adapter-test.identity-access.contract.http` | `modules/requestreview/http` | `identity-access/contract` | internal_test |

Deleted rules:

- `people-directory.adapter-test.student-presence-compose`
- `domain.adapter-test.projection.public`
- `people-directory.http.identity-access-contract`
- `people-directory.adapter-test.student-deletion-compose`
- `people-directory.adapter-test.request-review-compose`
- `people-directory.http.delivery-public`
- `people-directory.http.contract`
- `people-directory.http.settings-public`
