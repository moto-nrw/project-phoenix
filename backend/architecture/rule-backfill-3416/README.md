# Rule issue backfill review (#3416)

**Status: assignments approved by the user in this implementation session. Cleanup issues #3444–#3448 created.**

Evidence commit: `4aa36c315fc19e8ae8dda50193ffa34843e25575`.

Baseline: 2,040 rules, 504 compatibility descriptions, 682 legacy entries, policy epoch 17. The fixed-build graph reports 167 stale rules, including 19 compatibility rules. Deletion leaves 1,873 rules and 485 temporary permissions. The legacy manifest is unchanged.

## Review decisions

The user approved the cleanup mappings and five new tickets below. Every compatibility rule has a row and rationale in [resolution.jsonl](resolution.jsonl). Its original description and extracted historical issue numbers are in [proposal.jsonl](proposal.jsonl). These historical mentions are not cleanup assignments. All 167 deleted IDs are in [stale-rules.txt](stale-rules.txt).

| Cleanup owner | Rules |
|---|---:|
| delete | 19 |
| https://github.com/moto-nrw/project-phoenix/issues/2725 | 48 |
| https://github.com/moto-nrw/project-phoenix/issues/2736 | 33 |
| https://github.com/moto-nrw/project-phoenix/issues/2748 | 1 |
| https://github.com/moto-nrw/project-phoenix/issues/2750 | 3 |
| https://github.com/moto-nrw/project-phoenix/issues/3351 | 83 |
| https://github.com/moto-nrw/project-phoenix/issues/3352 | 4 |
| https://github.com/moto-nrw/project-phoenix/issues/3418 | 41 |
| https://github.com/moto-nrw/project-phoenix/issues/3421 | 30 |
| https://github.com/moto-nrw/project-phoenix/issues/3422 | 58 |
| https://github.com/moto-nrw/project-phoenix/issues/3424 | 104 |
| https://github.com/moto-nrw/project-phoenix/issues/3427 | 46 |
| https://github.com/moto-nrw/project-phoenix/issues/3444 | 2 |
| https://github.com/moto-nrw/project-phoenix/issues/3445 | 7 |
| https://github.com/moto-nrw/project-phoenix/issues/3446 | 19 |
| https://github.com/moto-nrw/project-phoenix/issues/3447 | 5 |
| https://github.com/moto-nrw/project-phoenix/issues/3448 | 1 |

## Created cleanup tickets

Each new ticket is a child of #2580 and follows up #3416. Tickets stay open until all listed permissions and any exact debt converted from them are removed.

### class-day-tests (#3444)

Replace retained repositories passed to schedule services in class-day integration tests.

Exit criteria: remove these implementation dependencies through public capabilities or consumer-owned ports; delete the permissions; preserve behavior and tenant isolation; pass affected behavior tests, the architecture check and issue audit. Any conversion to exact debt must retain this ticket as owner until no rule or legacy entry references it.

Rules:

- `class-day-view.integration-test.people-directory-postgres`
- `class-day-view.integration-test.enrollment-application`

### file-storage (#3445)

Replace seven retained File Storage composition and test dependencies.

Exit criteria: remove these implementation dependencies through public capabilities or consumer-owned ports; delete the permissions; preserve behavior and tenant isolation; pass affected behavior tests, the architecture check and issue audit. Any conversion to exact debt must retain this ticket as owner until no rule or legacy entry references it.

Rules:

- `file-storage.compose.settings-domain`
- `file-storage.compose.audit-domain`
- `file-storage.compose.security-application`
- `file-storage.compose.security-contract`
- `file-storage.compose.delivery-adapter`
- `file-storage.compose.document-rendering-postgres`
- `file-storage.compose.document-rendering-domain`

### identity-behavior (#3446)

Remove cross-owner implementation dependencies from the relocated Identity behavior suites.

Exit criteria: remove these implementation dependencies through public capabilities or consumer-owned ports; delete the permissions; preserve behavior and tenant isolation; pass affected behavior tests, the architecture check and issue audit. Any conversion to exact debt must retain this ticket as owner until no rule or legacy entry references it.

Rules:

- `identity-access.behaviour.identity-public`
- `identity-access.behaviour.identity-adapter`
- `identity-access.behaviour.identity-application`
- `identity-access.behaviour.identity-domain`
- `identity-access.behaviour.security-application`
- `identity-access.behaviour.security-domain`
- `identity-access.behaviour.security-public`
- `identity-access.behaviour.delivery-adapter`
- `identity-access.behaviour.audit-domain`
- `identity-access.behaviour.transaction-domain`
- `identity-access.behaviour.settings-domain`
- `identity-access.behaviour.settings-application`
- `identity-access.behaviour.enrollment-domain`
- `identity-access.behaviour.enrollment-public`
- `identity-access.behaviour.care-plan-domain`
- `identity-access.behaviour.organization-tenancy-domain`
- `identity-access.behaviour.people-directory-domain`
- `identity-access.behaviour.delivery-domain`
- `identity-access.behaviour.tenant-runtime-public`

### presence-http (#3447)

Replace shared helper, date-type and settings implementation imports in Presence HTTP and adapter tests.

Exit criteria: remove these implementation dependencies through public capabilities or consumer-owned ports; delete the permissions; preserve behavior and tenant isolation; pass affected behavior tests, the architecture check and issue audit. Any conversion to exact debt must retain this ticket as owner until no rule or legacy entry references it.

Rules:

- `student-presence.http.inbound-common`
- `student-presence.http.legacy-shared-domain`
- `student-presence.adapter-test.inbound-common`
- `student-presence.adapter-test.legacy-shared-domain`
- `student-presence.adapter-test.settings-platform-domain`

### settings-test-dates (#3448)

Settings test support must stop exposing the retained date type.

Exit criteria: remove these implementation dependencies through public capabilities or consumer-owned ports; delete the permissions; preserve behavior and tenant isolation; pass affected behavior tests, the architecture check and issue audit. Any conversion to exact debt must retain this ticket as owner until no rule or legacy entry references it.

Rules:

- `settings-platform.test-support.legacy-shared-domain`

## Reproduction

Run `scripts/propose-rule-issues.mjs` against the evidence commit's policy. Run `scripts/resolve-rule-issues-3416.mjs` to reproduce the reviewed resolution. Neither script edits policy or creates tickets.

Schema changes to 3; policy epoch stays 17. Existing cleanup tickets were confirmed open during preparation. The final audit must recheck the union of rule and legacy issues.
