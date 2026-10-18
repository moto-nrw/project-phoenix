# 4086 relocation inventory

Snapshot: round 2, based on `a70bde68787956afb3b95490e5d157cce6f4ff8a` plus the two round-1 commits on `refactor/4086-timetable-cluster`.



Integration: development `0f879e2de5dd49cfe39a918972269f0476e1d045` (#4321) was merged before publication. The importer lists below were refreshed from the integrated graph; package/LOC totals still describe the original 39-package relocation snapshot. The one legacy-manifest conflict used normalized relocation identities and base + both deltas: 76 base keys, 70 local, 62 incoming, 56 combined. Policy epoch 75 keeps active relocations above the integrated base epoch 74.

Scope: all production packages under `services/`, `database/repositories/`, `models/`, and every `legacy/` directory; every tracked `legacy_*.go` file is listed separately. Architecture fixture directories under `internal/architecture/testdata/` are checker inputs, not production packages. Test-only companion packages are called out below.

Result: **5 production packages moved (534 LOC), 32 blocked, 2 deferred** out of 39 production packages. The file-only moves are separate from the package/LOC total.

Method: the pinned Go toolchain produced `go list -e -tags=testhooks -json ./...` with `GOOS=linux GOARCH=amd64 CGO_ENABLED=0`. Owners and all production/internal-test/external-test roles come from `architecture/policy.json`. Importers below include production and both test scopes. Same-owner imports are necessary but not sufficient: Go-internal visibility, ADR 0020 file identity, and all existing ratchets must also pass. No role, rule, grant, exception or hook permission was added.

A moved package keeps its owner, production role and both test roles. Its existing exact debt follows its path through an ADR 0020 declaration. No filename-only move is presented as dissolving an implementation package that still sits outside `internal/`.

## Package decisions

| Source | Owner | Role | Production LOC | Decision | Destination or blocker |
|---|---|---|---:|---|---|
| `database/repositories/audit` | `audit-platform` | `postgres` | 2797 | Blocked | Rejected by the existing checker or quality ratchet. Exact output below. |
| `database/repositories/base` | `transaction-runtime` | `postgres` | 760 | Blocked | Foreign consumers cannot import the target Go `internal` package. Migrate the consumers listed below to the owner public contract first. |
| `database/repositories/config` | `settings-platform` | `postgres` | 675 | Blocked | Foreign consumers cannot import the target Go `internal` package. Migrate the consumers listed below to the owner public contract first. |
| `database/repositories/delivery` | `delivery-platform` | `postgres` | 388 | Moved | `modules/delivery/internal/adapters/deliverypostgres` |
| `database/repositories/parent` | `care-plan` | `postgres` | 801 | Deferred | #4082 group 3: Care Plan. Also subject to the role/consumer constraints below. |
| `database/repositories/pwausage` | `observability` | `postgres` | 74 | Blocked | Rejected by the existing checker or quality ratchet. Exact output below. |
| `database/repositories/users` | `people-directory` | `postgres` | 4723 | Blocked | Rejected by the existing checker or quality ratchet. Exact output below. |
| `database/repositories/workforce` | `workforce` | `postgres` | 113 | Moved | `modules/workforce/internal/adapters/workforcepostgres` |
| `models/activities` | `timetable-activities` | `legacy-model` | 1442 | Blocked | ADR 0057 pins this role to `models/`; ADR 0020 forbids changing it. Foreign consumers, if any, must use the public contract first. |
| `models/base` | `transaction-runtime` | `legacy-model` | 644 | Blocked | ADR 0057 pins this role to `models/`; ADR 0020 forbids changing it. Foreign consumers, if any, must use the public contract first. |
| `models/calendar` | `school-calendar` | `legacy-model` | 161 | Blocked | ADR 0057 pins this role to `models/`; ADR 0020 forbids changing it. Foreign consumers, if any, must use the public contract first. |
| `models/delivery` | `delivery-platform` | `legacy-model` | 181 | Blocked | ADR 0057 pins this role to `models/`; ADR 0020 forbids changing it. Foreign consumers, if any, must use the public contract first. |
| `models/enrollment` | `enrollment` | `legacy-model` | 779 | Blocked | ADR 0057 pins this role to `models/`; ADR 0020 forbids changing it. Foreign consumers, if any, must use the public contract first. |
| `models/facilities` | `facilities` | `legacy-model` | 79 | Blocked | ADR 0057 pins this role to `models/`; ADR 0020 forbids changing it. Foreign consumers, if any, must use the public contract first. |
| `models/iot` | `device-fleet` | `legacy-model` | 149 | Blocked | ADR 0057 pins this role to `models/`; ADR 0020 forbids changing it. Foreign consumers, if any, must use the public contract first. |
| `models/parent` | `care-plan` | `legacy-model` | 240 | Deferred | #4082 group 3: Care Plan. Also subject to the role/consumer constraints below. |
| `models/schedule` | `timetable-activities` | `legacy-model` | 3155 | Blocked | ADR 0057 pins this role to `models/`; ADR 0020 forbids changing it. Foreign consumers, if any, must use the public contract first. |
| `models/users` | `people-directory` | `legacy-model` | 2938 | Blocked | ADR 0057 pins this role to `models/`; ADR 0020 forbids changing it. Foreign consumers, if any, must use the public contract first. |
| `modules/devicefleet/compose/legacy` | `device-fleet` | `compose` | 143 | Blocked | Foreign consumers cannot import the target Go `internal` package. Migrate the consumers listed below to the owner public contract first. |
| `modules/emergencysnapshot/legacy` | `emergency-snapshot` | `adapter` | 11 | Moved | `modules/emergencysnapshot/internal/adapters/nameorder` |
| `modules/facilities/compose/legacy` | `facilities` | `compose` | 517 | Blocked | Foreign consumers cannot import the target Go `internal` package. Migrate the consumers listed below to the owner public contract first. |
| `modules/grouplive/legacy` | `group-live-view` | `adapter` | 11 | Moved | `modules/grouplive/internal/adapters/nameorder` |
| `modules/supervisiondashboard/legacy` | `calendar-view` | `adapter` | 11 | Moved | `modules/supervisiondashboard/internal/adapters/nameorder` |
| `modules/workforce/legacy` | `workforce` | `adapter` | 445 | Blocked | Rejected by the existing checker or quality ratchet. Exact output below. |
| `services/activities` | `timetable-activities` | `application` | 39 | Blocked | Foreign consumers cannot import the target Go `internal` package. Migrate the consumers listed below to the owner public contract first. |
| `services/audit` | `audit-platform` | `application` | 435 | Blocked | Rejected by the existing checker or quality ratchet. Exact output below. |
| `services/config` | `settings-platform` | `application` | 4116 | Blocked | Rejected by the existing checker or quality ratchet. Exact output below. |
| `services/config/defaults` | `settings-platform` | `application` | 3538 | Blocked | Rejected by the existing checker or quality ratchet. Exact output below. |
| `services/config/settingstest` | `settings-platform` | `test-support` | 178 | Blocked | Foreign consumers cannot import the target Go `internal` package. Migrate the consumers listed below to the owner public contract first. |
| `services/config/sideeffects` | `settings-platform` | `application` | 58 | Blocked | Rejected by the existing checker or quality ratchet. Exact output below. |
| `services/database` | `transaction-runtime` | `application` | 149 | Blocked | Rejected by the existing checker or quality ratchet. Exact output below. |
| `services/facilities` | `facilities` | `application` | 786 | Blocked | Rejected by the existing checker or quality ratchet. Exact output below. |
| `services/import` | `data-import` | `application` | 5245 | Blocked | Rejected by the existing checker or quality ratchet. Exact output below. |
| `services/import/ports` | `data-import` | `port` | 593 | Blocked | Rejected by the existing checker or quality ratchet. Exact output below. |
| `services/iot` | `device-fleet` | `application` | 373 | Blocked | Rejected by the existing checker or quality ratchet. Exact output below. |
| `services/iot/staffclock` | `staff-clock` | `application` | 384 | Blocked | Rejected by the existing checker or quality ratchet. Exact output below. |
| `services/listexport` | `document-rendering` | `application` | 3000 | Blocked | Foreign consumers cannot import the target Go `internal` package. Migrate the consumers listed below to the owner public contract first. |
| `services/parentmessaging` | `communication` | `application` | 562 | Blocked | Foreign consumers cannot import the target Go `internal` package. Migrate the consumers listed below to the owner public contract first. |
| `services/scheduler` | `scheduler-runtime` | `application` | 5364 | Blocked | Rejected by the existing checker or quality ratchet. Exact output below. |

## Importers and Wave E prerequisites by owner

For blocked packages, each foreign-owner group below is the concrete consumer set that must switch to the owner public contract. `Imports`, `TestImports` and `XTestImports` retain their Go-list meaning. A same-owner consumer outside the owner module still cannot cross Go `internal`. Deferred packages stay with their active track; they are not silently counted as movable.

### audit-platform

**`database/repositories/audit`: blocked.** Rejected by the existing checker or quality ratchet. Exact output below.

- `audit-platform` (same owner): `database/repositories/audit/integration [XTestImports]`, `modules/auditlog/compose [Imports]`.


Rejected attempt (reverted; output is from the unchanged check or ratchet):

```text
--- FAIL: TestModuleComposeAliasRatchet (0.14s)
    module_compose_alias_ratchet_test.go:122: Compose alias ratchet check failed (1 issue(s)):

        [compose alias] modules/auditlog/compose: 8 alias(es) of foreign packages, budget says 16.
          Nice — lower the budget to 8 so the reduction cannot regress.
--- FAIL: TestModuleFunctionLengthRatchet (0.79s)
    module_function_length_ratchet_test.go:247: Module function-length ratchet check failed (2 issue(s)):

        [func-length] modules/auditlog/internal/adapters/auditpostgres/appender.go:appendQuery has a 77-line body (limit 60), and is not in the allowlist.
          Split the function into named steps — do not add new entries.

        [func-length] modules/auditlog/internal/adapters/auditpostgres/booking_consistency.go:(*bookingConsistencyRepository).Audit has a 191-line body (limit 60), and is not in the allowlist.
          Split the function into named steps — do not add new entries.
FAIL
FAIL	github.com/moto-nrw/project-phoenix/test	1.360s
FAIL
```

**`services/audit`: blocked.** Rejected by the existing checker or quality ratchet. Exact output below.

- `audit-platform` (same owner): `modules/auditlog [Imports]`, `modules/auditlog/compose [Imports]`.


Rejected attempt (reverted; output is from the unchanged check or ratchet):

```text
--- FAIL: TestModulePassthroughRatchet (0.28s)
    module_passthrough_ratchet_test.go:424: Module-passthrough ratchet check failed (1 issue(s)):

        [module passthrough] modules/auditlog: 4 single-call delegation(s), budget 1.
          Delegations were added. A service method encapsulates (validation, orchestration, audit,
          tenant transaction, error mapping) or it does not exist (Rule 8). Never raise a budget.

        These counts measure a shape (four AST conditions), not the worth of any single method —
        a counted method may well be the right place for its logic. See "Known limitations" in
        the header of test/module_passthrough_ratchet_test.go before acting on an individual hit.
--- FAIL: TestModuleComposeAliasRatchet (0.04s)
    module_compose_alias_ratchet_test.go:122: Compose alias ratchet check failed (1 issue(s)):

        [compose alias] modules/auditlog/compose: 8 alias(es) of foreign packages, budget says 16.
          Nice — lower the budget to 8 so the reduction cannot regress.
FAIL
FAIL	github.com/moto-nrw/project-phoenix/test	0.727s
FAIL
```

### calendar-view

**`modules/supervisiondashboard/legacy`: moved.** `modules/supervisiondashboard/internal/adapters/nameorder`

- `calendar-view` (same owner): `modules/supervisiondashboard/compose [Imports]`.


### care-plan

**`database/repositories/parent`: deferred.** #4082 group 3: Care Plan. Also subject to the role/consumer constraints below.

- `care-plan` (same owner): `modules/careplan/compose [Imports]`, `modules/careplan/integration [TestImports]`.


**`models/parent`: deferred.** #4082 group 3: Care Plan. Also subject to the role/consumer constraints below.

- `care-plan` (same owner): `database/repositories/parent [Imports]`, `modules/careplan/compose [Imports]`, `modules/careplan/integration [TestImports]`, `modules/careplan/internal/adapters/postgres [Imports]`, `modules/careplan/internal/adapters/postgres [TestImports]`, `modules/careplan/internal/application [Imports]`.


Same-role relocation is rejected by `validateLegacyModelPackages` (`internal/architecture/target_rule.go`); the diagnostic for this destination is:

```text
package "modules/careplan/internal/domain/parent" is outside models/ and may not have role "legacy-model" (ADR 0057)
```

### communication

**`services/parentmessaging`: blocked.** Foreign consumers cannot import the target Go `internal` package. Migrate the consumers listed below to the owner public contract first.

- `care-plan` (foreign owner; public-contract prerequisite): `modules/careplan/contracttest [XTestImports]`.

- `communication` (same owner): `modules/communication/composition [Imports]`, `modules/communication/internal/parentmessages [Imports]`, `modules/communication/internal/parentmessages [XTestImports]`, `modules/communication/internal/staffannouncements [Imports]`.



### data-import

**`services/import`: blocked.** Rejected by the existing checker or quality ratchet. Exact output below.

- `data-import` (same owner): `modules/dataimport/compose/retained [Imports]`, `services/import/integration [TestImports]`.


Rejected attempt (reverted; output is from the unchanged check or ratchet):

```text
ratchet budgets passed: 11 constants, 29 budget registers, 2 exception registers
composition surface grew against base a70bde68787956afb3b95490e5d157cce6f4ff8a:
  internal_test|setter|modules/dataimport/internal/application/importservice|fakeClassList.CreateClassListEntry->fakeClassList.created at modules/dataimport/internal/application/importservice/owner_fakes_test.go:218:2
  internal_test|setter|modules/dataimport/internal/application/importservice|fakeClassList.CreateClassListEntry->fakeClassList.entries at modules/dataimport/internal/application/importservice/owner_fakes_test.go:217:2
  internal_test|setter|modules/dataimport/internal/application/importservice|fakeGuardians.AddGuardianPhone->fakeGuardians.phones at modules/dataimport/internal/application/importservice/owner_fakes_test.go:341:2
  internal_test|setter|modules/dataimport/internal/application/importservice|fakeGuardians.CreateGuardian->fakeGuardians.guardians at modules/dataimport/internal/application/importservice/owner_fakes_test.go:318:2
  internal_test|setter|modules/dataimport/internal/application/importservice|fakeGuardians.LinkGuardianToStudent->fakeGuardians.linked at modules/dataimport/internal/application/importservice/owner_fakes_test.go:358:2
  internal_test|setter|modules/dataimport/internal/application/importservice|fakeGuardians.LinkGuardianToStudent->fakeGuardians.links at modules/dataimport/internal/application/importservice/owner_fakes_test.go:357:2
  internal_test|setter|modules/dataimport/internal/application/importservice|fakeGuardians.UpdateGuardian->fakeGuardians.guardians at modules/dataimport/internal/application/importservice/owner_fakes_test.go:332:2
  internal_test|setter|modules/dataimport/internal/application/importservice|fakeGuardians.UpdateGuardianLink->fakeGuardians.updated at modules/dataimport/internal/application/importservice/owner_fakes_test.go:365:2
  internal_test|setter|modules/dataimport/internal/application/importservice|fakePersons.CreatePerson->fakePersons.created at modules/dataimport/internal/application/importservice/owner_fakes_test.go:46:2
  internal_test|setter|modules/dataimport/internal/application/importservice|fakePersons.CreatePerson->fakePersons.persons at modules/dataimport/internal/application/importservice/owner_fakes_test.go:45:2
  internal_test|setter|modules/dataimport/internal/application/importservice|fakePersons.UpdatePerson->fakePersons.persons at modules/dataimport/internal/application/importservice/owner_fakes_test.go:58:2
  internal_test|setter|modules/dataimport/internal/application/importservice|fakePersons.UpdatePerson->fakePersons.updated at modules/dataimport/internal/application/importservice/owner_fakes_test.go:59:2
remaining legacy violations: 70
exit status 1
```

**`services/import/ports`: blocked.** The import-service cluster is rejected below. Its same-owner `services/import` consumers remain outside `modules/dataimport/`, so the port cannot move independently across Go `internal`.

- `data-import` (same owner): `services/import [Imports]`, `services/import [TestImports]`, `services/import/integration [TestImports]`.


Rejected attempt (reverted; output is from the unchanged check or ratchet):

```text
ratchet budgets passed: 11 constants, 29 budget registers, 2 exception registers
composition surface grew against base a70bde68787956afb3b95490e5d157cce6f4ff8a:
  internal_test|setter|modules/dataimport/internal/application/importservice|fakeClassList.CreateClassListEntry->fakeClassList.created at modules/dataimport/internal/application/importservice/owner_fakes_test.go:218:2
  internal_test|setter|modules/dataimport/internal/application/importservice|fakeClassList.CreateClassListEntry->fakeClassList.entries at modules/dataimport/internal/application/importservice/owner_fakes_test.go:217:2
  internal_test|setter|modules/dataimport/internal/application/importservice|fakeGuardians.AddGuardianPhone->fakeGuardians.phones at modules/dataimport/internal/application/importservice/owner_fakes_test.go:341:2
  internal_test|setter|modules/dataimport/internal/application/importservice|fakeGuardians.CreateGuardian->fakeGuardians.guardians at modules/dataimport/internal/application/importservice/owner_fakes_test.go:318:2
  internal_test|setter|modules/dataimport/internal/application/importservice|fakeGuardians.LinkGuardianToStudent->fakeGuardians.linked at modules/dataimport/internal/application/importservice/owner_fakes_test.go:358:2
  internal_test|setter|modules/dataimport/internal/application/importservice|fakeGuardians.LinkGuardianToStudent->fakeGuardians.links at modules/dataimport/internal/application/importservice/owner_fakes_test.go:357:2
  internal_test|setter|modules/dataimport/internal/application/importservice|fakeGuardians.UpdateGuardian->fakeGuardians.guardians at modules/dataimport/internal/application/importservice/owner_fakes_test.go:332:2
  internal_test|setter|modules/dataimport/internal/application/importservice|fakeGuardians.UpdateGuardianLink->fakeGuardians.updated at modules/dataimport/internal/application/importservice/owner_fakes_test.go:365:2
  internal_test|setter|modules/dataimport/internal/application/importservice|fakePersons.CreatePerson->fakePersons.created at modules/dataimport/internal/application/importservice/owner_fakes_test.go:46:2
  internal_test|setter|modules/dataimport/internal/application/importservice|fakePersons.CreatePerson->fakePersons.persons at modules/dataimport/internal/application/importservice/owner_fakes_test.go:45:2
  internal_test|setter|modules/dataimport/internal/application/importservice|fakePersons.UpdatePerson->fakePersons.persons at modules/dataimport/internal/application/importservice/owner_fakes_test.go:58:2
  internal_test|setter|modules/dataimport/internal/application/importservice|fakePersons.UpdatePerson->fakePersons.updated at modules/dataimport/internal/application/importservice/owner_fakes_test.go:59:2
remaining legacy violations: 70
exit status 1
```

### delivery-platform

**`database/repositories/delivery`: moved.** `modules/delivery/internal/adapters/deliverypostgres`

- `delivery-platform` (same owner): `modules/delivery/compose [Imports]`, `modules/delivery/internal/adapters/postgres [XTestImports]`.


**`models/delivery`: blocked.** ADR 0057 pins this role to `models/`; ADR 0020 forbids changing it. Foreign consumers, if any, must use the public contract first.

- `delivery-platform` (same owner): `modules/delivery/application/notifications [Imports]`, `modules/delivery/application/notifications [TestImports]`, `modules/delivery/application/pwa [Imports]`, `modules/delivery/application/pwa [TestImports]`, `modules/delivery/compose [Imports]`, `modules/delivery/internal/adapters/deliverypostgres [Imports]`, `modules/delivery/internal/adapters/postgres [XTestImports]`.


Same-role relocation is rejected by `validateLegacyModelPackages` (`internal/architecture/target_rule.go`); the diagnostic for this destination is:

```text
package "modules/delivery/internal/domain/delivery" is outside models/ and may not have role "legacy-model" (ADR 0057)
```

### device-fleet

**`models/iot`: blocked.** ADR 0057 pins this role to `models/`; ADR 0020 forbids changing it. Foreign consumers, if any, must use the public contract first.

- `device-fleet` (same owner): `modules/devicefleet/compose/httpadapter [XTestImports]`, `modules/devicefleet/compose/legacy [Imports]`, `modules/devicefleet/compose/repositoryadapter [Imports]`, `services/iot [Imports]`.


Same-role relocation is rejected by `validateLegacyModelPackages` (`internal/architecture/target_rule.go`); the diagnostic for this destination is:

```text
package "modules/devicefleet/internal/domain/iot" is outside models/ and may not have role "legacy-model" (ADR 0057)
```

**`modules/devicefleet/compose/legacy`: blocked.** Foreign consumers cannot import the target Go `internal` package. Migrate the consumers listed below to the owner public contract first.

- `device-fleet` (same owner): `modules/devicefleet/compose/httpadapter [XTestImports]`.

- `root-composition` (foreign owner; public-contract prerequisite): `api [Imports]`, `api [TestImports]`.



**`services/iot`: blocked.** Rejected by the existing checker or quality ratchet. Exact output below.

- `device-fleet` (same owner): `modules/devicefleet/compose [Imports]`, `modules/devicefleet/compose/httpadapter [XTestImports]`.


Rejected attempt (reverted; output is from the unchanged check or ratchet):

```text
--- FAIL: TestModulePassthroughRatchet (0.29s)
    module_passthrough_ratchet_test.go:424: Module-passthrough ratchet check failed (1 issue(s)):

        [module passthrough] modules/devicefleet: 76 single-call delegation(s), budget 58.
          Delegations were added. A service method encapsulates (validation, orchestration, audit,
          tenant transaction, error mapping) or it does not exist (Rule 8). Never raise a budget.

        These counts measure a shape (four AST conditions), not the worth of any single method —
        a counted method may well be the right place for its logic. See "Known limitations" in
        the header of test/module_passthrough_ratchet_test.go before acting on an individual hit.
--- FAIL: TestModuleComposeAliasRatchet (0.04s)
    module_compose_alias_ratchet_test.go:122: Compose alias ratchet check failed (1 issue(s)):

        [compose alias] modules/devicefleet/compose: budget 1, but no alias is left (deleted, moved or renamed?).
          Remove the entry.
FAIL
FAIL	github.com/moto-nrw/project-phoenix/test	0.736s
FAIL
```

### document-rendering

**`services/listexport`: blocked.** Foreign consumers cannot import the target Go `internal` package. Migrate the consumers listed below to the owner public contract first.

- `document-rendering` (same owner): `modules/documentrendering/lists [Imports]`, `modules/planexport [Imports]`, `modules/planexport [TestImports]`.

- `facilities` (foreign owner; public-contract prerequisite): `modules/facilities/compose/httpadapter [Imports]`.



### emergency-snapshot

**`modules/emergencysnapshot/legacy`: moved.** `modules/emergencysnapshot/internal/adapters/nameorder`

- `emergency-snapshot` (same owner): `modules/emergencysnapshot/compose [Imports]`.


### enrollment

**`models/enrollment`: blocked.** ADR 0057 pins this role to `models/`; ADR 0020 forbids changing it. Foreign consumers, if any, must use the public contract first.

- `care-plan` (foreign owner; public-contract prerequisite): `modules/careplan/contracttest [XTestImports]`.

- `enrollment` (same owner): `modules/enrollment/http [XTestImports]`, `modules/enrollment/internal/application [Imports]`, `modules/enrollment/internal/application [TestImports]`.



Same-role relocation is rejected by `validateLegacyModelPackages` (`internal/architecture/target_rule.go`); the diagnostic for this destination is:

```text
package "modules/enrollment/internal/domain/enrollment" is outside models/ and may not have role "legacy-model" (ADR 0057)
```

### facilities

**`models/facilities`: blocked.** ADR 0057 pins this role to `models/`; ADR 0020 forbids changing it. Foreign consumers, if any, must use the public contract first.

- `facilities` (same owner): `modules/facilities/compose [Imports]`, `modules/facilities/compose/legacy [Imports]`, `modules/facilities/compose/legacy [TestImports]`, `modules/facilities/compose/repositoryadapter [Imports]`, `modules/facilities/compose/repositoryadapter [TestImports]`.


Same-role relocation is rejected by `validateLegacyModelPackages` (`internal/architecture/target_rule.go`); the diagnostic for this destination is:

```text
package "modules/facilities/internal/domain/facilities" is outside models/ and may not have role "legacy-model" (ADR 0057)
```

**`modules/facilities/compose/legacy`: blocked.** Foreign consumers cannot import the target Go `internal` package. Migrate the consumers listed below to the owner public contract first.

- `root-composition` (foreign owner; public-contract prerequisite): `api [Imports]`.


**`services/facilities`: blocked.** Rejected by the existing checker or quality ratchet. Exact output below.

- `facilities` (same owner): `modules/facilities/compose [Imports]`, `modules/facilities/compose/httpadapter [Imports]`, `modules/facilities/compose/legacy [Imports]`.


Rejected attempt (reverted; output is from the unchanged check or ratchet):

```text
--- FAIL: TestModuleComposeAliasRatchet (0.10s)
    module_compose_alias_ratchet_test.go:122: Compose alias ratchet check failed (2 issue(s)):

        [compose alias] modules/facilities/compose/legacy: budget 1, but no alias is left (deleted, moved or renamed?).
          Remove the entry.

        [compose alias] modules/facilities/compose: budget 11, but no alias is left (deleted, moved or renamed?).
          Remove the entry.
--- FAIL: TestModulePassthroughRatchet (0.14s)
    module_passthrough_ratchet_test.go:424: Module-passthrough ratchet check failed (1 issue(s)):

        [module passthrough] modules/facilities: 29 single-call delegation(s), budget 19.
          Delegations were added. A service method encapsulates (validation, orchestration, audit,
          tenant transaction, error mapping) or it does not exist (Rule 8). Never raise a budget.

        These counts measure a shape (four AST conditions), not the worth of any single method —
        a counted method may well be the right place for its logic. See "Known limitations" in
        the header of test/module_passthrough_ratchet_test.go before acting on an individual hit.
FAIL
FAIL	github.com/moto-nrw/project-phoenix/test	0.750s
FAIL
```

### group-live-view

**`modules/grouplive/legacy`: moved.** `modules/grouplive/internal/adapters/nameorder`

- `group-live-view` (same owner): `modules/grouplive/compose [Imports]`.


### observability

**`database/repositories/pwausage`: blocked.** Rejected by the existing checker or quality ratchet. Exact output below.

- `observability` (same owner): `database/repositories/pwausage [XTestImports]`, `observability/compose [Imports]`.


Rejected attempt (reverted; output is from the unchanged check or ratchet):

```text
load Go packages with GOOS=linux GOARCH=amd64 CGO_ENABLED=0: package github.com/moto-nrw/project-phoenix/observability/compose
	observability/compose/compose.go:7:8: use of internal package github.com/moto-nrw/project-phoenix/modules/observability/internal/adapters/pwausage not allowed
exit status 1
```

### people-directory

**`database/repositories/users`: blocked.** Rejected by the existing checker or quality ratchet. Exact output below.

- `people-directory` (same owner): `modules/peopledirectory/compose [Imports]`.


Rejected attempt (reverted; output is from the unchanged check or ratchet):

```text
--- FAIL: TestModuleLegacyBudgetRatchet (0.16s)
    module_legacy_budget_ratchet_test.go:302: Module legacy LOC budget ratchet check failed (4 issue(s)):

        [legacy_*.go LOC budget] database/repositories/users/legacy_composition_support.go: budget 25 LOC, but it holds no production .go code any more.
          Remove the entry.

        [legacy_*.go LOC budget] database/repositories/users/legacy_types.go: budget 14 LOC, but it holds no production .go code any more.
          Remove the entry.

        [legacy_*.go LOC budget] modules/peopledirectory/internal/adapters/peoplepostgres/legacy_composition_support.go: 25 LOC, but it has no budget entry.
          Code moved here from a counted place keeps counting; a new entry needs a justification and the total may not rise.

        [legacy_*.go LOC budget] modules/peopledirectory/internal/adapters/peoplepostgres/legacy_types.go: 14 LOC, but it has no budget entry.
          Code moved here from a counted place keeps counting; a new entry needs a justification and the total may not rise.
--- FAIL: TestModuleComposeAliasRatchet (0.05s)
    module_compose_alias_ratchet_test.go:122: Compose alias ratchet check failed (1 issue(s)):

        [compose alias] modules/peopledirectory/compose: budget 16, but no alias is left (deleted, moved or renamed?).
          Remove the entry.
--- FAIL: TestModuleFileSizeRatchet (0.13s)
    module_file_size_ratchet_test.go:80: Module file-size ratchet check failed (1 issue(s)):

        [file-size] modules/peopledirectory/internal/adapters/peoplepostgres/guardian_relationships.go has 1010 lines (limit 800), and is not in the allowlist.
          Split the file along a real seam — a new file over the limit does not get an entry.
--- FAIL: TestModuleComplexityRatchet (0.39s)
    module_complexity_ratchet_test.go:344: Module-complexity ratchet check failed (3 issue(s)):

        [gocognit] modules/peopledirectory/internal/adapters/peoplepostgres/guardian_profile.go:(*guardianProfileRepository).linkAccount scores 20 (limit 15), and is not in the allowlist.
          Move branching/orchestration behind a capability method (Rule 4) — do not add new entries.

        [gocognit] modules/peopledirectory/internal/adapters/peoplepostgres/guardian_relationships.go:(*GuardianRelationshipRepository).FilterAccountsWithStudentAccess scores 21 (limit 15), and is not in the allowlist.
          Move branching/orchestration behind a capability method (Rule 4) — do not add new entries.

        [gocognit] modules/peopledirectory/internal/adapters/peoplepostgres/guardian_relationships.go:(*GuardianRelationshipRepository).updateColumns scores 24 (limit 15), and is not in the allowlist.
          Move branching/orchestration behind a capability method (Rule 4) — do not add new entries.
--- FAIL: TestModuleFunctionLengthRatchet (0.39s)
    module_function_length_ratchet_test.go:247: Module function-length ratchet check failed (4 issue(s)):

        [func-length] modules/peopledirectory/internal/adapters/peoplepostgres/guardian_profile.go:(*guardianProfileRepository).LoadProfileWithChildren has a 75-line body (limit 60), and is not in the allowlist.
          Split the function into named steps — do not add new entries.

        [func-length] modules/peopledirectory/internal/adapters/peoplepostgres/guardian_profile.go:(*guardianProfileRepository).linkAccount has a 76-line body (limit 60), and is not in the allowlist.
          Split the function into named steps — do not add new entries.

        [func-length] modules/peopledirectory/internal/adapters/peoplepostgres/guardian_relationships.go:(*GuardianRelationshipRepository).FilterAccountsWithStudentAccess has a 68-line body (limit 60), and is not in the allowlist.
          Split the function into named steps — do not add new entries.

        [func-length] modules/peopledirectory/internal/adapters/peoplepostgres/student_snapshots.go:NativeStudentSnapshot has a 66-line body (limit 60), and is not in the allowlist.
          Split the function into named steps — do not add new entries.
FAIL
FAIL	github.com/moto-nrw/project-phoenix/test	0.757s
FAIL
```

**`models/users`: blocked.** ADR 0057 pins this role to `models/`; ADR 0020 forbids changing it. Foreign consumers, if any, must use the public contract first.

- `care-plan` (foreign owner; public-contract prerequisite): `modules/careplan/contracttest [XTestImports]`.

- `enrollment` (foreign owner; public-contract prerequisite): `modules/enrollment/http [XTestImports]`.

- `legacy-composition` (foreign owner; public-contract prerequisite): `models [TestImports]`.

- `people-directory` (same owner): `database/repositories/users [Imports]`, `database/repositories/users [TestImports]`, `modules/peopledirectory/compose [Imports]`, `modules/peopledirectory/compose/vocabulary [Imports]`, `modules/peopledirectory/contracttest [XTestImports]`, `modules/peopledirectory/internal/application [Imports]`, `modules/peopledirectory/ports [Imports]`.

- `timetable-activities` (foreign owner; public-contract prerequisite): `modules/timetable/compose [Imports]`, `modules/timetable/compose/httpadapter [Imports]`.






Same-role relocation is rejected by `validateLegacyModelPackages` (`internal/architecture/target_rule.go`); the diagnostic for this destination is:

```text
package "modules/peopledirectory/internal/domain/users" is outside models/ and may not have role "legacy-model" (ADR 0057)
```

### scheduler-runtime

**`services/scheduler`: blocked.** Rejected by the existing checker or quality ratchet. Exact output below.

- `scheduler-runtime` (same owner): `modules/schedulerruntime/compose [Imports]`.


Rejected attempt (reverted; output is from the unchanged check or ratchet):

```text
ratchet budgets passed: 11 constants, 29 budget registers, 2 exception registers
composition surface grew against base a70bde68787956afb3b95490e5d157cce6f4ff8a:
  internal_test|setter|modules/schedulerruntime/internal/application/scheduler|unitScheduler->Scheduler.allTenantIDsLoader at modules/schedulerruntime/internal/application/scheduler/runtime_test.go:129:3
  internal_test|setter|modules/schedulerruntime/internal/application/scheduler|unitScheduler->Scheduler.feedbackCleaner at modules/schedulerruntime/internal/application/scheduler/runtime_test.go:135:3
  internal_test|setter|modules/schedulerruntime/internal/application/scheduler|unitScheduler->Scheduler.minuteSnapshotLoader at modules/schedulerruntime/internal/application/scheduler/runtime_test.go:126:3
  internal_test|setter|modules/schedulerruntime/internal/application/scheduler|unitScheduler->Scheduler.registry at modules/schedulerruntime/internal/application/scheduler/runtime_test.go:132:3
  internal_test|setter|modules/schedulerruntime/internal/application/scheduler|unitScheduler->Scheduler.tenantRuntime at modules/schedulerruntime/internal/application/scheduler/runtime_test.go:123:3
  production|setter|modules/schedulerruntime/internal/application/scheduler|Scheduler.rotateOverdueCacheIfNewDay->Scheduler.overdueEmittedDay at modules/schedulerruntime/internal/application/scheduler/scheduler.go:2219:3
remaining legacy violations: 70
exit status 1
```

### school-calendar

**`models/calendar`: blocked.** ADR 0057 pins this role to `models/`; ADR 0020 forbids changing it. Foreign consumers, if any, must use the public contract first.

- `school-calendar` (same owner): `modules/schoolcalendar/portal/compose [XTestImports]`.


Same-role relocation is rejected by `validateLegacyModelPackages` (`internal/architecture/target_rule.go`); the diagnostic for this destination is:

```text
package "modules/schoolcalendar/internal/domain/calendar" is outside models/ and may not have role "legacy-model" (ADR 0057)
```

### settings-platform

**`database/repositories/config`: blocked.** Foreign consumers cannot import the target Go `internal` package. Migrate the consumers listed below to the owner public contract first.

- `school-setup` (foreign owner; public-contract prerequisite): `modules/schoolsetup/compose [Imports]`.

- `settings-platform` (same owner): `modules/settings/compose [Imports]`, `services/config [TestImports]`.



**`services/config`: blocked.** Rejected by the existing checker or quality ratchet. Exact output below.

- `settings-platform` (same owner): `modules/settings [Imports]`, `modules/settings/compose [Imports]`.


Rejected attempt (reverted; output is from the unchanged check or ratchet):

```text
ratchet budgets passed: 11 constants, 29 budget registers, 2 exception registers
backend architecture legacy ratchet failed:
new violations (6):
  production|contracts.internal-model|github.com/moto-nrw/project-phoenix/modules/settings|settings.DefinitionNotFoundError -- public contract exposes an internal model type
    at modules/settings/settings.go:185 (settings.DefinitionNotFoundError)
  production|contracts.internal-model|github.com/moto-nrw/project-phoenix/modules/settings|settings.InvalidValueError -- public contract exposes an internal model type
    at modules/settings/settings.go:186 (settings.InvalidValueError)
  production|contracts.internal-model|github.com/moto-nrw/project-phoenix/modules/settings|settings.PermissionDeniedError -- public contract exposes an internal model type
    at modules/settings/settings.go:187 (settings.PermissionDeniedError)
  production|contracts.internal-model|github.com/moto-nrw/project-phoenix/modules/settings|settings.SettingsError -- public contract exposes an internal model type
    at modules/settings/settings.go:184 (settings.SettingsError)
  production|contracts.internal-model|github.com/moto-nrw/project-phoenix/modules/settings|settings.Snapshot -- public contract exposes an internal model type
    at modules/settings/settings.go:171 (settings.Snapshot)
  production|contracts.internal-model|github.com/moto-nrw/project-phoenix/modules/settings|settings.WithSettingsSnapshot -- public contract exposes an internal model type
    at modules/settings/settings.go:228 (settings.WithSettingsSnapshot)
composition surface grew against base a70bde68787956afb3b95490e5d157cce6f4ff8a:
  production|setter|modules/settings/internal/application/settingsservice|TenantOperations.SetValueSetHook->TenantOperations.hook at modules/settings/internal/application/settingsservice/tenant_operations.go:47:2
  production|setter|modules/settings/internal/application/settingsservice|settingsService.SetClassRestrictionGuard->settingsService.classRestrictionGuard at modules/settings/internal/application/settingsservice/settings_service.go:122:2
  production|setter|modules/settings/internal/application/settingsservice|settingsService.SetGradeCapGuard->settingsService.gradeCapGuard at modules/settings/internal/application/settingsservice/settings_service.go:136:2
  production|setter|modules/settings/internal/application/settingsservice|settingsService.SetGradeRestrictionGuard->settingsService.gradeRestrictionGuard at modules/settings/internal/application/settingsservice/settings_service.go:129:2
  production|setter|modules/settings/internal/application/settingsservice|settingsService.SetLookupObserver->settingsService.observeLookup at modules/settings/internal/application/settingsservice/settings_service.go:55:2
  production|setter|modules/settings/internal/application/settingsservice|settingsService.SetRuntime->settingsService.runtime at modules/settings/internal/application/settingsservice/settings_service.go:58:57
  production|setter|modules/settings/internal/application/sideeffects|Registry.Register->Registry.handlers at modules/settings/internal/application/sideeffects/registry.go:43:2
  production|setter|modules/settings/internal/application/sideeffects|Registry.SetFailureObserver->Registry.observeFailure at modules/settings/internal/application/sideeffects/registry.go:32:2
remaining legacy violations: 76
exit status 1
```

**`services/config/defaults`: blocked.** The settings-service cluster is rejected below. Separately, its same-owner `services/config` test consumer remains outside `modules/settings/`, so an independent move would cross Go `internal`.

- `settings-platform` (same owner): `modules/settings/compose [Imports]`, `services/config [TestImports]`.


Rejected attempt (reverted; output is from the unchanged check or ratchet):

```text
ratchet budgets passed: 11 constants, 29 budget registers, 2 exception registers
backend architecture legacy ratchet failed:
new violations (6):
  production|contracts.internal-model|github.com/moto-nrw/project-phoenix/modules/settings|settings.DefinitionNotFoundError -- public contract exposes an internal model type
    at modules/settings/settings.go:185 (settings.DefinitionNotFoundError)
  production|contracts.internal-model|github.com/moto-nrw/project-phoenix/modules/settings|settings.InvalidValueError -- public contract exposes an internal model type
    at modules/settings/settings.go:186 (settings.InvalidValueError)
  production|contracts.internal-model|github.com/moto-nrw/project-phoenix/modules/settings|settings.PermissionDeniedError -- public contract exposes an internal model type
    at modules/settings/settings.go:187 (settings.PermissionDeniedError)
  production|contracts.internal-model|github.com/moto-nrw/project-phoenix/modules/settings|settings.SettingsError -- public contract exposes an internal model type
    at modules/settings/settings.go:184 (settings.SettingsError)
  production|contracts.internal-model|github.com/moto-nrw/project-phoenix/modules/settings|settings.Snapshot -- public contract exposes an internal model type
    at modules/settings/settings.go:171 (settings.Snapshot)
  production|contracts.internal-model|github.com/moto-nrw/project-phoenix/modules/settings|settings.WithSettingsSnapshot -- public contract exposes an internal model type
    at modules/settings/settings.go:228 (settings.WithSettingsSnapshot)
composition surface grew against base a70bde68787956afb3b95490e5d157cce6f4ff8a:
  production|setter|modules/settings/internal/application/settingsservice|TenantOperations.SetValueSetHook->TenantOperations.hook at modules/settings/internal/application/settingsservice/tenant_operations.go:47:2
  production|setter|modules/settings/internal/application/settingsservice|settingsService.SetClassRestrictionGuard->settingsService.classRestrictionGuard at modules/settings/internal/application/settingsservice/settings_service.go:122:2
  production|setter|modules/settings/internal/application/settingsservice|settingsService.SetGradeCapGuard->settingsService.gradeCapGuard at modules/settings/internal/application/settingsservice/settings_service.go:136:2
  production|setter|modules/settings/internal/application/settingsservice|settingsService.SetGradeRestrictionGuard->settingsService.gradeRestrictionGuard at modules/settings/internal/application/settingsservice/settings_service.go:129:2
  production|setter|modules/settings/internal/application/settingsservice|settingsService.SetLookupObserver->settingsService.observeLookup at modules/settings/internal/application/settingsservice/settings_service.go:55:2
  production|setter|modules/settings/internal/application/settingsservice|settingsService.SetRuntime->settingsService.runtime at modules/settings/internal/application/settingsservice/settings_service.go:58:57
  production|setter|modules/settings/internal/application/sideeffects|Registry.Register->Registry.handlers at modules/settings/internal/application/sideeffects/registry.go:43:2
  production|setter|modules/settings/internal/application/sideeffects|Registry.SetFailureObserver->Registry.observeFailure at modules/settings/internal/application/sideeffects/registry.go:32:2
remaining legacy violations: 76
exit status 1
```

**`services/config/settingstest`: blocked.** Foreign consumers cannot import the target Go `internal` package. Migrate the consumers listed below to the owner public contract first.

- `workforce` (foreign owner; public-contract prerequisite): `modules/workforce/internal/application/timetracking [XTestImports]`.


**`services/config/sideeffects`: blocked.** Rejected by the existing checker or quality ratchet. Exact output below.

- `settings-platform` (same owner): `modules/settings/compose [Imports]`.


Rejected attempt (reverted; output is from the unchanged check or ratchet):

```text
ratchet budgets passed: 11 constants, 29 budget registers, 2 exception registers
composition surface grew against base a70bde68787956afb3b95490e5d157cce6f4ff8a:
  production|setter|modules/settings/internal/application/sideeffects|Registry.Register->Registry.handlers at modules/settings/internal/application/sideeffects/registry.go:43:2
  production|setter|modules/settings/internal/application/sideeffects|Registry.SetFailureObserver->Registry.observeFailure at modules/settings/internal/application/sideeffects/registry.go:32:2
remaining legacy violations: 70
exit status 1
```

### staff-clock

**`services/iot/staffclock`: blocked.** Rejected by the existing checker or quality ratchet. Exact output below.

- `staff-clock` (same owner): `workflows/staffclock/compose [Imports]`.


Rejected attempt (reverted; output is from the unchanged check or ratchet):

```text
--- FAIL: TestModuleComposeAliasRatchet (0.04s)
    module_compose_alias_ratchet_test.go:122: Compose alias ratchet check failed (1 issue(s)):

        [compose alias] workflows/staffclock/compose: budget 19, but no alias is left (deleted, moved or renamed?).
          Remove the entry.
--- FAIL: TestModulePassthroughRatchet (0.19s)
    module_passthrough_ratchet_test.go:424: Module-passthrough ratchet check failed (1 issue(s)):

        [module passthrough] workflows/staffclock: 2 single-call delegation(s), but the module has no budget.
          New application package or composition — a new application package gets an entry with today's count
          and shrinks from there (Rule 8); code moved out of a counted tree keeps its delegations counted.

        These counts measure a shape (four AST conditions), not the worth of any single method —
        a counted method may well be the right place for its logic. See "Known limitations" in
        the header of test/module_passthrough_ratchet_test.go before acting on an individual hit.
FAIL
FAIL	github.com/moto-nrw/project-phoenix/test	0.879s
FAIL
```

### timetable-activities

**`models/activities`: blocked.** ADR 0057 pins this role to `models/`; ADR 0020 forbids changing it. Foreign consumers, if any, must use the public contract first.

- `care-plan` (foreign owner; public-contract prerequisite): `modules/careplan/contracttest [XTestImports]`.

- `device-fleet` (foreign owner; public-contract prerequisite): `modules/devicefleet/compose/legacy [Imports]`.

- `facilities` (foreign owner; public-contract prerequisite): `modules/facilities/compose/legacy [Imports]`.

- `process-device-scan` (foreign owner; public-contract prerequisite): `modules/devicescan/compose [Imports]`.

- `timetable-activities` (same owner): `modules/timetable/compose [Imports]`, `modules/timetable/compose [TestImports]`, `modules/timetable/compose/httpadapter [Imports]`, `modules/timetable/compose/httpadapter [TestImports]`, `modules/timetable/compose/httpintegration [XTestImports]`, `modules/timetable/internal/application [Imports]`, `modules/timetable/internal/application [TestImports]`, `modules/timetable/ports [Imports]`.






Same-role relocation is rejected by `validateLegacyModelPackages` (`internal/architecture/target_rule.go`); the diagnostic for this destination is:

```text
package "modules/timetable/internal/domain/activities" is outside models/ and may not have role "legacy-model" (ADR 0057)
```

**`models/schedule`: blocked.** ADR 0057 pins this role to `models/`; ADR 0020 forbids changing it. Foreign consumers, if any, must use the public contract first.

- `care-plan` (foreign owner; public-contract prerequisite): `modules/careplan/contracttest [XTestImports]`.

- `device-fleet` (foreign owner; public-contract prerequisite): `modules/devicefleet/compose/legacy [Imports]`.

- `process-device-scan` (foreign owner; public-contract prerequisite): `modules/devicescan/compose [Imports]`.

- `student-presence` (foreign owner; public-contract prerequisite): `modules/studentpresence/internal/application/presence [XTestImports]`.

- `timetable-activities` (same owner): `modules/timetable/compose [Imports]`, `modules/timetable/compose [TestImports]`, `modules/timetable/compose/httpintegration [XTestImports]`, `modules/timetable/internal/adapters/postgres [Imports]`, `modules/timetable/internal/application [Imports]`, `modules/timetable/internal/application [TestImports]`, `modules/timetable/ports [Imports]`.






Same-role relocation is rejected by `validateLegacyModelPackages` (`internal/architecture/target_rule.go`); the diagnostic for this destination is:

```text
package "modules/timetable/internal/domain/schedule" is outside models/ and may not have role "legacy-model" (ADR 0057)
```

**`services/activities`: blocked.** Foreign consumers cannot import the target Go `internal` package. Migrate the consumers listed below to the owner public contract first.

- `facilities` (foreign owner; public-contract prerequisite): `modules/facilities/compose/legacy [Imports]`.

- `process-device-scan` (foreign owner; public-contract prerequisite): `modules/devicescan/compose [Imports]`.

- `timetable-activities` (same owner): `modules/timetable/compose/httpadapter [Imports]`, `modules/timetable/compose/httpintegration [XTestImports]`.




### transaction-runtime

**`database/repositories/base`: blocked.** Foreign consumers cannot import the target Go `internal` package. Migrate the consumers listed below to the owner public contract first.

- `delivery-platform` (foreign owner; public-contract prerequisite): `modules/delivery/internal/adapters/deliverypostgres [Imports]`.


**`models/base`: blocked.** ADR 0057 pins this role to `models/`; ADR 0020 forbids changing it. Foreign consumers, if any, must use the public contract first.

- `delivery-platform` (foreign owner; public-contract prerequisite): `models/delivery [Imports]`, `modules/delivery/internal/adapters/deliverypostgres [Imports]`.

- `legacy-composition` (foreign owner; public-contract prerequisite): `models [TestImports]`.

- `transaction-runtime` (same owner): `database [Imports]`, `database/repositories/base [Imports]`, `database/repositories/base [TestImports]`, `modules/transactionruntime/internal/adapters/postgres [Imports]`.




Same-role relocation is rejected by `validateLegacyModelPackages` (`internal/architecture/target_rule.go`); the diagnostic for this destination is:

```text
package "modules/transactionruntime/internal/domain/base" is outside models/ and may not have role "legacy-model" (ADR 0057)
```

**`services/database`: blocked.** Rejected by the existing checker or quality ratchet. Exact output below.

- `transaction-runtime` (same owner): `modules/transactionruntime [Imports]`.


Rejected attempt (reverted; output is from the unchanged check or ratchet):

```text
composition surface passed: 1644 -> 1644 field/setter targets across production and test scopes
ratchet budgets passed: 11 constants, 29 budget registers, 2 exception registers
backend architecture legacy ratchet failed:
new violations (5):
  production|contracts.internal-model|github.com/moto-nrw/project-phoenix/modules/transactionruntime|transactionruntime.DatabaseStats -- public contract exposes an internal model type
    at modules/transactionruntime/database_stats.go:10 (transactionruntime.DatabaseStats)
  production|contracts.internal-model|github.com/moto-nrw/project-phoenix/modules/transactionruntime|transactionruntime.DatabaseStatsCapabilities -- public contract exposes an internal model type
    at modules/transactionruntime/database_stats.go:12 (transactionruntime.DatabaseStatsCapabilities)
  production|contracts.internal-model|github.com/moto-nrw/project-phoenix/modules/transactionruntime|transactionruntime.DatabaseStatsDependencies -- public contract exposes an internal model type
    at modules/transactionruntime/database_stats.go:11 (transactionruntime.DatabaseStatsDependencies)
  production|contracts.internal-model|github.com/moto-nrw/project-phoenix/modules/transactionruntime|transactionruntime.DatabaseStatsResponse -- public contract exposes an internal model type
    at modules/transactionruntime/database_stats.go:13 (transactionruntime.DatabaseStatsResponse)
  production|contracts.internal-model|github.com/moto-nrw/project-phoenix/modules/transactionruntime|transactionruntime.NewDatabaseStats -- public contract exposes an internal model type
    at modules/transactionruntime/database_stats.go:17 (transactionruntime.NewDatabaseStats)
remaining legacy violations: 75
exit status 1
```

### workforce

**`database/repositories/workforce`: moved.** `modules/workforce/internal/adapters/workforcepostgres`

- `workforce` (same owner): `modules/workforce/compose [Imports]`.


**`modules/workforce/legacy`: blocked.** Rejected by the existing checker or quality ratchet. Exact output below.

- `workforce` (same owner): `modules/workforce/compose [Imports]`.


Rejected attempt (reverted; output is from the unchanged check or ratchet):

```text
--- FAIL: TestModuleComplexityRatchet (0.33s)
    module_complexity_ratchet_test.go:344: Module-complexity ratchet check failed (2 issue(s)):

        [gocognit] modules/workforce/internal/adapters/substitutions/substitution_repository.go:(*groupSubstitutionRepository).List scores 18 (limit 15), and is not in the allowlist.
          Move branching/orchestration behind a capability method (Rule 4) — do not add new entries.

        [gocognit] modules/workforce/legacy/substitution_repository.go:(*groupSubstitutionRepository).List is allowlisted at 18 but now scores ≤ 15 (or was deleted/renamed).
          Remove the entry.
FAIL
FAIL	github.com/moto-nrw/project-phoenix/test	0.921s
FAIL
```

## Legacy-prefixed files

File moves below preserve bytes, declarations and assertions. Existing implementation files already inside `internal/` lose only the filename prefix. Public port DTOs remain in their standing public port package. Root host binding/test-helper files remain in the root composition. Implementation in public compositions is blocked: moving only a file would split receiver types or change a public construction contract; simply renaming such a file would not put that implementation under `internal/`.

| Source at snapshot | Owner | Decision | Destination or prerequisite |
|---|---|---|---|
| `api/legacy_builders_test_helpers.go` | `root-composition` | Renamed | `api/owner_builders_test_helpers.go` |
| `api/legacy_token_adapter.go` | `root-composition` | Renamed | `api/session_token_bindings.go` |
| `database/repositories/users/legacy_composition_support.go` | `people-directory` | Blocked | Containing People Directory repository move is blocked by the unchanged quality ratchets. |
| `database/repositories/users/legacy_types.go` | `people-directory` | Blocked | Containing People Directory repository move is blocked by the unchanged quality ratchets. |
| `internal/architecture/legacy_test.go` | `architecture-check` | Retained | Tests the exact legacy-debt manifest; already internal checker code, not a retained application implementation. |
| `modules/devicefleet/compose/httpadapter/legacy_session_device_ids_test.go` | `device-fleet` | Renamed | `modules/devicefleet/compose/httpadapter/session_device_ids_test.go` |
| `modules/grouplive/legacy/legacy_test.go` | `group-live-view` | Moved with package | `modules/grouplive/internal/adapters/nameorder/legacy_test.go`; ADR 0020 requires the original file set. |
| `modules/peopledirectory/compose/legacy_repository_runtime.go` | `people-directory` | Blocked | Requires composition/application contract work, not a filename change. Existing type/method coupling must be removed first. |
| `modules/peopledirectory/inbound/students/legacy_contact_contract_test.go` | `people-directory` | Deferred | #4082 School Structure or #4079 People Directory HTTP. |
| `modules/presenceprojection/legacy_rows.go` | `presence-legacy-view` | Blocked | Timetable composition consumes the public package; first switch that consumer to the projection public contract. |
| `modules/schoolstructure/compose/legacy_group_catalog.go` | `school-structure` | Deferred | #4082 School Structure or #4079 People Directory HTTP. |
| `modules/schoolstructure/compose/legacy_group_contract.go` | `school-structure` | Deferred | #4082 School Structure or #4079 People Directory HTTP. |
| `modules/schoolstructure/compose/legacy_group_management_test_helpers.go` | `school-structure` | Deferred | #4082 School Structure or #4079 People Directory HTTP. |
| `modules/schoolstructure/compose/legacy_repository_runtime.go` | `school-structure` | Deferred | #4082 School Structure or #4079 People Directory HTTP. |
| `modules/supervisiondashboard/legacy/legacy_test.go` | `calendar-view` | Moved with package | `modules/supervisiondashboard/internal/adapters/nameorder/legacy_test.go`; ADR 0020 requires the original file set. |
| `modules/timetable/compose/httpintegration/legacy_activity_exception_repo_test.go` | `timetable-activities` | Renamed | `modules/timetable/compose/httpintegration/activity_exception_repo_test.go` |
| `modules/timetable/compose/httpintegration/legacy_activity_group_repo_test.go` | `timetable-activities` | Renamed | `modules/timetable/compose/httpintegration/activity_group_repo_test.go` |
| `modules/timetable/compose/httpintegration/legacy_arrival_schedule_test.go` | `timetable-activities` | Renamed | `modules/timetable/compose/httpintegration/arrival_schedule_test.go` |
| `modules/timetable/compose/httpintegration/legacy_calendar_period_usage_repo_test.go` | `timetable-activities` | Renamed | `modules/timetable/compose/httpintegration/calendar_period_usage_repo_test.go` |
| `modules/timetable/compose/httpintegration/legacy_instance_staff_repo_test.go` | `timetable-activities` | Renamed | `modules/timetable/compose/httpintegration/instance_staff_repo_test.go` |
| `modules/timetable/compose/httpintegration/legacy_instance_student_repo_test.go` | `timetable-activities` | Renamed | `modules/timetable/compose/httpintegration/instance_student_repo_test.go` |
| `modules/timetable/compose/httpintegration/legacy_list_behavior_test.go` | `timetable-activities` | Renamed | `modules/timetable/compose/httpintegration/list_behavior_test.go` |
| `modules/timetable/compose/httpintegration/legacy_pickup_schedule_test.go` | `timetable-activities` | Renamed | `modules/timetable/compose/httpintegration/pickup_schedule_test.go` |
| `modules/timetable/compose/httpintegration/legacy_planning_track_repo_test.go` | `timetable-activities` | Renamed | `modules/timetable/compose/httpintegration/planning_track_repo_test.go` |
| `modules/timetable/compose/httpintegration/legacy_presence_cutover_composition_test.go` | `timetable-activities` | Renamed | `modules/timetable/compose/httpintegration/presence_cutover_composition_test.go` |
| `modules/timetable/compose/httpintegration/legacy_recurrence_rule_repository_test.go` | `timetable-activities` | Renamed | `modules/timetable/compose/httpintegration/recurrence_rule_repository_test.go` |
| `modules/timetable/compose/httpintegration/legacy_repository_support_test.go` | `timetable-activities` | Renamed | `modules/timetable/compose/httpintegration/repository_support_test.go` |
| `modules/timetable/compose/httpintegration/legacy_student_day_test.go` | `timetable-activities` | Renamed | `modules/timetable/compose/httpintegration/student_day_test.go` |
| `modules/timetable/compose/httpintegration/legacy_timeframe_repository_test.go` | `timetable-activities` | Renamed | `modules/timetable/compose/httpintegration/timeframe_repository_test.go` |
| `modules/timetable/compose/legacy_activity_service.go` | `timetable-activities` | Blocked | Requires composition/application contract work, not a filename change. Existing type/method coupling must be removed first. |
| `modules/timetable/compose/legacy_category_operations.go` | `timetable-activities` | Blocked | Requires composition/application contract work, not a filename change. Existing type/method coupling must be removed first. |
| `modules/timetable/compose/legacy_enrollment_operations.go` | `timetable-activities` | Blocked | Requires composition/application contract work, not a filename change. Existing type/method coupling must be removed first. |
| `modules/timetable/compose/legacy_errors.go` | `timetable-activities` | Blocked | Requires composition/application contract work, not a filename change. Existing type/method coupling must be removed first. |
| `modules/timetable/compose/legacy_group_update.go` | `timetable-activities` | Blocked | Requires composition/application contract work, not a filename change. Existing type/method coupling must be removed first. |
| `modules/timetable/compose/legacy_interface.go` | `timetable-activities` | Blocked | Requires composition/application contract work, not a filename change. Existing type/method coupling must be removed first. |
| `modules/timetable/compose/legacy_schedule_operations.go` | `timetable-activities` | Blocked | Requires composition/application contract work, not a filename change. Existing type/method coupling must be removed first. |
| `modules/timetable/compose/legacy_supervisor_operations.go` | `timetable-activities` | Blocked | Requires composition/application contract work, not a filename change. Existing type/method coupling must be removed first. |
| `modules/timetable/internal/application/legacy_composition_support.go` | `timetable-activities` | Renamed | `modules/timetable/internal/application/composition_support.go` |
| `modules/timetable/internal/application/legacy_composition_support_test.go` | `timetable-activities` | Renamed | `modules/timetable/internal/application/composition_support_test.go` |
| `modules/timetable/internal/application/legacy_composition_support_test_helpers.go` | `timetable-activities` | Renamed | `modules/timetable/internal/application/composition_support_test_helpers.go` |
| `modules/timetable/internal/application/legacy_repository_options.go` | `timetable-activities` | Renamed | `modules/timetable/internal/application/repository_options.go` |
| `modules/timetable/ports/legacy_repository_options.go` | `timetable-activities` | Renamed | `modules/timetable/ports/repository_options.go` |
| `modules/workforce/internal/planning/legacy_helpers.go` | `workforce` | Renamed | `modules/workforce/internal/planning/helpers.go` |

## Test-only and concurrent surfaces

- `database/repositories/audit/integration` (`audit-platform`, `adapter`, both test roles `adapter-test`) must accompany the Audit repository into its owner subtree. It was included in that rejected attempt. It has no production Go files.
- `services/import/integration` (`data-import`, `test-support`) must accompany the import service. It has no production Go files.
- `models` (`legacy-composition`, `legacy-model`) and `database/repositories/studentintegration` (`legacy-composition`, `compose`) currently contain tests only. They are not counted as production relocation candidates. Their imports are still counted as consumers above.
- Deferred active work: #4082 owns Care Plan, School Structure and `workflows/careexit`; #4079 owns People Directory, Student Presence and Timetable HTTP. No package relocation in this PR takes those active implementation packages. Test imports inside these packages may only receive mechanical import-path updates when an eligible dependency moves.

## Verification and limits

Round-1 dead-code repair: `database/repositories/base.AcquireXactLock` lost its last production caller when the Workforce lock binding moved to its own adapter. The unused helper is removed. Its existing contended-lock test moves to the Workforce composition and calls that live adapter, keeping every assertion and the same SQL lock/evidence contract. No dead-code exception or legacy key is added.

Round-1 lint repair: the Device Scan wrappers use Go-promoted embedded methods instead of redundant explicit embedded-field selectors (Staticcheck QF1008). This leaves calls and behavior unchanged. The syntax-based pass-through count falls from 55 to 48 as a result; its existing budget and total are lowered by seven. This is a scanner-shape change, not seven removed forwarding methods.

Each attempted owner cluster ran `scripts/backend-architecture.sh check`. Every retained package move also passed the unchanged module file-size, function-length, cognitive-complexity, facade-width, pass-through, legacy-budget, composition-alias and HTTP-ORM ratchets. Rejected attempts are absent from the final policy and source tree. Existing alias and legacy-file/tree budgets are only lowered or removed when their measured contents disappear.

Before publication: merge `origin/development`, rerun architecture checks, and run `/tmp/orch2580/heavy.sh scripts/run-go-toolchain.sh scripts/test-changed.sh origin/development` without `--fast`, using `GOCACHE=/tmp/gocache-4086`. PR review/CI results are recorded in the PR, not inferred from this inventory.
