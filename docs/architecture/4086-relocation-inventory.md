# 4086 relocation inventory

Snapshot: round 3, based on development `41f264ba26dba4581c74571870217c6924d335dc`.

Integration: importer lists are refreshed from the round-3 candidate; the final integration base is recorded in the PR.

Scope: all production packages under `services/`, `database/repositories/`, `models/`, and every `legacy/` directory; every tracked `legacy_*.go` file is listed separately. Architecture fixture directories under `internal/architecture/testdata/` are checker inputs, not production packages. Test-only companion packages are called out below.

Result: **10 production packages moved cumulatively (9,658 source LOC), 27 blocked, 2 deferred** out of 39 production packages. The file-only moves are separate from the package/LOC total.

Method: the pinned Go toolchain produced `go list -e -tags=testhooks -json ./...` with `GOOS=linux GOARCH=amd64 CGO_ENABLED=0`. Owners and all production/internal-test/external-test roles come from `architecture/policy.json`. Importers below include production and both test scopes. Same-owner imports are necessary but not sufficient: Go-internal visibility, ADR 0020 file identity, and all existing ratchets must also pass. No role, rule, grant, exception or hook permission was added.

A moved package keeps its owner, production role and both test roles. Its existing exact debt follows its path through an ADR 0020 declaration. No filename-only move is presented as dissolving an implementation package that still sits outside `internal/`.


## Timetable contract follow-up

The [Timetable contract-cluster inventory](timetable-contract-cluster.md)
records every foreign importer and selected symbol at development
`caee1cbd5b`, the three production-owner cutovers, seven deleted #4004 keys,
and the two unchanged #4076 test-consumer blockers. The alias-only
`services/activities` package is deleted. Both model packages remain because
ADR 0057 and ADR 0020 do not permit a role-preserving internal relocation;
the native-fixture migration is also still needed. No model package or
repository is claimed as newly relocated by that follow-up.

## Round 3

This round moves **5 production packages, 9,124 source LOC**, plus the Audit test-only integration package. The 56 exact debt keys remain 56, with no rule, alias, role, grant, exception or hook allowance added. The untouched foreign-consumer and legacy-model decisions remain Wave E work; only their importer lists are refreshed. Care Plan remains deferred while its active prerequisite is open.

Measured reductions:

- Compose aliases: Audit 16 → 8; Device Fleet 1 → 0; Facilities 11 → 0 and its legacy composition 1 → 0; People Directory 16 → 0. Total 253 → 216.
- Legacy tree: Workforce 445 → 0; total 1,105 → 660.
- Legacy-prefixed files: People Directory 25 → 0 and 14 → 0; total 2,992 → 2,953.
- Compose LOC: Facilities 394 → 273; total 33,202 → 33,081.
- Cognitive complexity: delete the stale Workforce `List` score-18 entry; no replacement entry.
- Pass-through budgets do not rise: Device Fleet stays 58, Facilities stays 19. The 28 deleted delegations offset the 28 newly scanned retained methods, rather than hiding them through moved paths or aliases.

## Package decisions

| Source | Owner | Role | Production LOC | Decision | Destination or blocker |
|---|---|---|---:|---|---|
| `database/repositories/audit` | `audit-platform` | `postgres` | 2797 | Moved | `modules/auditlog/internal/adapters/auditpostgres` |
| `database/repositories/base` | `transaction-runtime` | `postgres` | 760 | Blocked | Foreign consumers cannot import the target Go `internal` package. Migrate the consumers listed below to the owner public contract first. |
| `database/repositories/config` | `settings-platform` | `postgres` | 675 | Blocked | Foreign consumers cannot import the target Go `internal` package. Migrate the consumers listed below to the owner public contract first. |
| `database/repositories/delivery` | `delivery-platform` | `postgres` | 388 | Moved | `modules/delivery/internal/adapters/deliverypostgres` |
| `database/repositories/parent` | `care-plan` | `postgres` | 801 | Deferred | #4082 group 3: Care Plan. Also subject to the role/consumer constraints below. |
| `database/repositories/pwausage` | `observability` | `postgres` | 74 | Blocked | Go internal visibility: `observability/compose` is outside `modules/observability/` and cannot import its internal adapter. |
| `database/repositories/users` | `people-directory` | `postgres` | 4723 | Moved | `modules/peopledirectory/internal/adapters/peoplepostgres` |
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
| `modules/workforce/legacy` | `workforce` | `adapter` | 445 | Moved | `modules/workforce/internal/adapters/substitutions` |
| `services/activities` | `timetable-activities` | `application` | 39 | Blocked | Foreign consumers cannot import the target Go `internal` package. Migrate the consumers listed below to the owner public contract first. |
| `services/audit` | `audit-platform` | `application` | 435 | Blocked | Application/adapter contract split required: `UnregisteredTagScanService.DeleteOlderThan(int)` and `DeleteExpired()` implement the retained days-based API over the timestamp-based Device Fleet repository. Their unchanged method/test contract cannot be replaced by embedding that target. Even after exposing Append's observation orchestration, these two adapter delegations exceed the existing owner budget of one. Retaining them elsewhere in the application scan would hide debt; moving them to an adapter needs a role/contract split. |
| `services/config` | `settings-platform` | `application` | 4116 | Blocked | Public contract/role split required: `modules/settings` exposes six implementation types/functions (`Snapshot`, `WithSettingsSnapshot`, and four error types). Relocation also exposes six mutable service hooks to the composition guard. |
| `services/config/defaults` | `settings-platform` | `application` | 3538 | Blocked | Go internal visibility: the retained `services/config` tests import defaults from outside `modules/settings/`. The combined settings move also exposes the six public-contract violations listed below. |
| `services/config/settingstest` | `settings-platform` | `test-support` | 178 | Blocked | Foreign consumers cannot import the target Go `internal` package. Migrate the consumers listed below to the owner public contract first. |
| `services/config/sideeffects` | `settings-platform` | `application` | 58 | Blocked | Mutable composition hooks: `Registry.Register -> handlers` and `Registry.SetFailureObserver -> observeFailure` become two new path-keyed setter targets. An immutable construction contract is required; no hook allowance is carried by ADR 0020. |
| `services/database` | `transaction-runtime` | `application` | 149 | Blocked | Public contract/role split required: the Transaction Runtime facade exposes `DatabaseStats`, `DatabaseStatsDependencies`, `DatabaseStatsCapabilities`, `DatabaseStatsResponse`, and `NewDatabaseStats` from the moved implementation. |
| `services/facilities` | `facilities` | `application` | 786 | Moved | `modules/facilities/internal/application/roomservice` |
| `services/import` | `data-import` | `application` | 5245 | Blocked | Test composition hooks: twelve path-keyed setter targets in the unchanged `fakePersons`, `fakeClassList`, and `fakeGuardians` fakes. Moving them needs a test-support/role split or hook allowances; both are outside this relocation. |
| `services/import/ports` | `data-import` | `port` | 593 | Blocked | Go internal visibility: retained `services/import` and its integration suite cannot import `modules/dataimport/internal/ports/imports`. The combined service move needs the test-support/role split described below. |
| `services/iot` | `device-fleet` | `application` | 373 | Moved | `modules/devicefleet/internal/application/deviceservice` |
| `services/iot/staffclock` | `staff-clock` | `application` | 384 | Blocked | Public error contract/role split required: the retained `StampError{Kind, Cause}` must preserve `Error() == Cause.Error()` and its existing literals/tests. After sharing request admission, this required Error delegation remains in an application owner with no budget. Removing it needs extraction to a public contract/adapter role and corresponding import rules, not a new budget or scan exception. |
| `services/listexport` | `document-rendering` | `application` | 3000 | Blocked | Foreign consumers cannot import the target Go `internal` package. Migrate the consumers listed below to the owner public contract first. |
| `services/parentmessaging` | `communication` | `application` | 562 | Blocked | Foreign consumers cannot import the target Go `internal` package. Migrate the consumers listed below to the owner public contract first. |
| `services/scheduler` | `scheduler-runtime` | `application` | 5364 | Blocked | Mutable composition hooks: five unchanged `unitScheduler` test injections plus the production `rotateOverdueCacheIfNewDay -> overdueEmittedDay` setter become new path-keyed targets. Constructor/test-support separation is required, not an increased allowance. |

## Importers and Wave E prerequisites by owner

For blocked packages, each foreign-owner group below is the concrete consumer set that must switch to the owner public contract. `Imports`, `TestImports` and `XTestImports` retain their Go-list meaning. A same-owner consumer outside the owner module still cannot cross Go `internal`. Deferred packages stay with their active track; they are not silently counted as movable.

### audit-platform

**`database/repositories/audit`: moved.** `modules/auditlog/internal/adapters/auditpostgres`

- `audit-platform` (same owner): `modules/auditlog/compose [Imports]`, `modules/auditlog/internal/adapters/auditpostgres/integration [XTestImports]`.

`appendQuery` now normalizes public event rows in named helpers; `Audit` uses the identical SQL in a package constant. Function-length checks pass without new entries. Its test-only integration package moved with it.

**`services/audit`: blocked.** Application/adapter contract split required: `UnregisteredTagScanService.DeleteOlderThan(int)` and `DeleteExpired()` implement the retained days-based API over the timestamp-based Device Fleet repository. Their unchanged method/test contract cannot be replaced by embedding that target. Even after exposing Append's observation orchestration, these two adapter delegations exceed the existing owner budget of one. Retaining them elsewhere in the application scan would hide debt; moving them to an adapter needs a role/contract split.

- `audit-platform` (same owner): `modules/auditlog [Imports]`, `modules/auditlog/compose [Imports]`.

Round-3 contract probe reverted cleanly; assertions, roles, rules and budgets remain unchanged.

```text
--- FAIL: TestModulePassthroughRatchet (0.20s)
    module_passthrough_ratchet_test.go:424: Module-passthrough ratchet check failed (2 issue(s)):

        [module passthrough] modules/auditlog: 3 single-call delegation(s), budget 1.
          Delegations were added. A service method encapsulates (validation, orchestration, audit,
          tenant transaction, error mapping) or it does not exist (Rule 8). Never raise a budget.

        [module passthrough] workflows/staffclock: 1 single-call delegation(s), but the module has no budget.
          New application package or composition — a new application package gets an entry with today's count
          and shrinks from there (Rule 8); code moved out of a counted tree keeps its delegations counted.

        These counts measure a shape (four AST conditions), not the worth of any single method —
        a counted method may well be the right place for its logic. See "Known limitations" in
        the header of test/module_passthrough_ratchet_test.go before acting on an individual hit.
FAIL
FAIL	github.com/moto-nrw/project-phoenix/test	1.288s
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

**`services/import`: blocked.** Test composition hooks: twelve path-keyed setter targets in the unchanged `fakePersons`, `fakeClassList`, and `fakeGuardians` fakes. Moving them needs a test-support/role split or hook allowances; both are outside this relocation.

- `data-import` (same owner): `modules/dataimport/compose/retained [Imports]`, `services/import/integration [TestImports]`.

Round-3 attempt reverted cleanly. Exact remaining diagnostic:

```text
ratchet budgets passed: 11 constants, 29 budget registers, 2 exception registers
composition surface grew against base 41f264ba26dba4581c74571870217c6924d335dc:
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
remaining legacy violations: 56
exit status 1
```

**`services/import/ports`: blocked.** Go internal visibility: retained `services/import` and its integration suite cannot import `modules/dataimport/internal/ports/imports`. The combined service move needs the test-support/role split described below.

- `data-import` (same owner): `services/import [Imports]`, `services/import [TestImports]`, `services/import/integration [TestImports]`.

Round-3 attempt reverted cleanly. Exact remaining diagnostic:

```text
load Go packages with GOOS=linux GOARCH=amd64 CGO_ENABLED=0: package github.com/moto-nrw/project-phoenix/services/import
	services/import/batches.go:12:2: use of internal package github.com/moto-nrw/project-phoenix/modules/dataimport/internal/ports/imports not allowed
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

- `device-fleet` (same owner): `modules/devicefleet/compose/httpadapter [XTestImports]`, `modules/devicefleet/compose/legacy [Imports]`, `modules/devicefleet/compose/repositoryadapter [Imports]`, `modules/devicefleet/internal/application/deviceservice [Imports]`.


Same-role relocation is rejected by `validateLegacyModelPackages` (`internal/architecture/target_rule.go`); the diagnostic for this destination is:

```text
package "modules/devicefleet/internal/domain/iot" is outside models/ and may not have role "legacy-model" (ADR 0057)
```

**`modules/devicefleet/compose/legacy`: blocked.** Foreign consumers cannot import the target Go `internal` package. Migrate the consumers listed below to the owner public contract first.

- `device-fleet` (same owner): `modules/devicefleet/compose/httpadapter [XTestImports]`.

- `root-composition` (foreign owner; public-contract prerequisite): `api [Imports]`, `api [TestImports]`.


**`services/iot`: moved.** `modules/devicefleet/internal/application/deviceservice`

- `device-fleet` (same owner): `modules/devicefleet/compose [Imports]`, `modules/devicefleet/compose/httpadapter [XTestImports]`.

Embedded the existing native Administration target for six identical methods. Four lookup adapter/service pairs and two list/count pairs are collapsed into the Engine operations; the six redundant Service methods are gone. Eighteen counted delegations were removed, offsetting the eighteen newly scanned retained methods.

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


**`services/facilities`: moved.** `modules/facilities/internal/application/roomservice`

- `facilities` (same owner): `modules/facilities/compose [Imports]`, `modules/facilities/compose/httpadapter [Imports]`, `modules/facilities/compose/legacy [Imports]`.

Removed six adapter/service query pairs. The native Engine now owns the observed query plus public result/error conversion; six redundant Service methods are gone. The composed Engine is embedded directly. Existing behavior tests and assertions are unchanged. Ten counted delegations were removed, offsetting the ten newly scanned retained methods.

### group-live-view

**`modules/grouplive/legacy`: moved.** `modules/grouplive/internal/adapters/nameorder`

- `group-live-view` (same owner): `modules/grouplive/compose [Imports]`.


### observability

**`database/repositories/pwausage`: blocked.** Go internal visibility: `observability/compose` is outside `modules/observability/` and cannot import its internal adapter.

- `observability` (same owner): `database/repositories/pwausage [XTestImports]`, `observability/compose [Imports]`.

Round-3 attempt reverted cleanly. Exact remaining diagnostic:

```text
load Go packages with GOOS=linux GOARCH=amd64 CGO_ENABLED=0: package github.com/moto-nrw/project-phoenix/observability/compose
	observability/compose/compose.go:7:8: use of internal package github.com/moto-nrw/project-phoenix/modules/observability/internal/adapters/pwausage not allowed
exit status 1
```

### people-directory

**`database/repositories/users`: moved.** `modules/peopledirectory/internal/adapters/peoplepostgres`

- `people-directory` (same owner): `modules/peopledirectory/compose [Imports]`.

Split guardian relationship writes from reads, profile account locking/unlinking, primary-phone/child loading, access-input validation/stable ordering, and departure snapshot mapping. The two legacy-prefixed files become `composition_support.go` and `compatibility_types.go`. This package has no baseline key in either direction, so ordinary same-owner/same-role classification suffices; no ADR 0020 key remapping or file-set exception is needed for the file splits/renames.

**`models/users`: blocked.** ADR 0057 pins this role to `models/`; ADR 0020 forbids changing it. Foreign consumers, if any, must use the public contract first.

- `care-plan` (foreign owner; public-contract prerequisite): `modules/careplan/contracttest [XTestImports]`.

- `enrollment` (foreign owner; public-contract prerequisite): `modules/enrollment/http [XTestImports]`.

- `legacy-composition` (foreign owner; public-contract prerequisite): `models [TestImports]`.

- `people-directory` (same owner): `modules/peopledirectory/compose [Imports]`, `modules/peopledirectory/compose/vocabulary [Imports]`, `modules/peopledirectory/contracttest [XTestImports]`, `modules/peopledirectory/internal/adapters/peoplepostgres [Imports]`, `modules/peopledirectory/internal/adapters/peoplepostgres [TestImports]`, `modules/peopledirectory/internal/application [Imports]`, `modules/peopledirectory/ports [Imports]`.

- `timetable-activities` (foreign owner; public-contract prerequisite): `modules/timetable/compose [Imports]`, `modules/timetable/compose/httpadapter [Imports]`.


Same-role relocation is rejected by `validateLegacyModelPackages` (`internal/architecture/target_rule.go`); the diagnostic for this destination is:

```text
package "modules/peopledirectory/internal/domain/users" is outside models/ and may not have role "legacy-model" (ADR 0057)
```

### scheduler-runtime

**`services/scheduler`: blocked.** Mutable composition hooks: five unchanged `unitScheduler` test injections plus the production `rotateOverdueCacheIfNewDay -> overdueEmittedDay` setter become new path-keyed targets. Constructor/test-support separation is required, not an increased allowance.

- `scheduler-runtime` (same owner): `modules/schedulerruntime/compose [Imports]`.

Round-3 attempt reverted cleanly. Exact remaining diagnostic:

```text
ratchet budgets passed: 11 constants, 29 budget registers, 2 exception registers
composition surface grew against base 41f264ba26dba4581c74571870217c6924d335dc:
  internal_test|setter|modules/schedulerruntime/internal/application/scheduler|unitScheduler->Scheduler.allTenantIDsLoader at modules/schedulerruntime/internal/application/scheduler/runtime_test.go:129:3
  internal_test|setter|modules/schedulerruntime/internal/application/scheduler|unitScheduler->Scheduler.feedbackCleaner at modules/schedulerruntime/internal/application/scheduler/runtime_test.go:135:3
  internal_test|setter|modules/schedulerruntime/internal/application/scheduler|unitScheduler->Scheduler.minuteSnapshotLoader at modules/schedulerruntime/internal/application/scheduler/runtime_test.go:126:3
  internal_test|setter|modules/schedulerruntime/internal/application/scheduler|unitScheduler->Scheduler.registry at modules/schedulerruntime/internal/application/scheduler/runtime_test.go:132:3
  internal_test|setter|modules/schedulerruntime/internal/application/scheduler|unitScheduler->Scheduler.tenantRuntime at modules/schedulerruntime/internal/application/scheduler/runtime_test.go:123:3
  production|setter|modules/schedulerruntime/internal/application/scheduler|Scheduler.rotateOverdueCacheIfNewDay->Scheduler.overdueEmittedDay at modules/schedulerruntime/internal/application/scheduler/scheduler.go:2219:3
remaining legacy violations: 56
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


**`services/config`: blocked.** Public contract/role split required: `modules/settings` exposes six implementation types/functions (`Snapshot`, `WithSettingsSnapshot`, and four error types). Relocation also exposes six mutable service hooks to the composition guard.

- `settings-platform` (same owner): `modules/settings [Imports]`, `modules/settings/compose [Imports]`.

Round-3 attempt reverted cleanly. Exact remaining diagnostic:

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
composition surface grew against base 41f264ba26dba4581c74571870217c6924d335dc:
  production|setter|modules/settings/internal/application/settingsservice|TenantOperations.SetValueSetHook->TenantOperations.hook at modules/settings/internal/application/settingsservice/tenant_operations.go:47:2
  production|setter|modules/settings/internal/application/settingsservice|settingsService.SetClassRestrictionGuard->settingsService.classRestrictionGuard at modules/settings/internal/application/settingsservice/settings_service.go:122:2
  production|setter|modules/settings/internal/application/settingsservice|settingsService.SetGradeCapGuard->settingsService.gradeCapGuard at modules/settings/internal/application/settingsservice/settings_service.go:136:2
  production|setter|modules/settings/internal/application/settingsservice|settingsService.SetGradeRestrictionGuard->settingsService.gradeRestrictionGuard at modules/settings/internal/application/settingsservice/settings_service.go:129:2
  production|setter|modules/settings/internal/application/settingsservice|settingsService.SetLookupObserver->settingsService.observeLookup at modules/settings/internal/application/settingsservice/settings_service.go:55:2
  production|setter|modules/settings/internal/application/settingsservice|settingsService.SetRuntime->settingsService.runtime at modules/settings/internal/application/settingsservice/settings_service.go:58:57
  production|setter|modules/settings/internal/application/sideeffects|Registry.Register->Registry.handlers at modules/settings/internal/application/sideeffects/registry.go:43:2
  production|setter|modules/settings/internal/application/sideeffects|Registry.SetFailureObserver->Registry.observeFailure at modules/settings/internal/application/sideeffects/registry.go:32:2
remaining legacy violations: 62
exit status 1
```

**`services/config/defaults`: blocked.** Go internal visibility: the retained `services/config` tests import defaults from outside `modules/settings/`. The combined settings move also exposes the six public-contract violations listed below.

- `settings-platform` (same owner): `modules/settings/compose [Imports]`, `services/config [TestImports]`.

Round-3 attempt reverted cleanly. Exact remaining diagnostic:

```text
# github.com/moto-nrw/project-phoenix/services/config
package github.com/moto-nrw/project-phoenix/services/config (test)
	services/config/attendance_scope_test.go:12:2: use of internal package github.com/moto-nrw/project-phoenix/modules/settings/internal/application/defaults not allowed
FAIL	github.com/moto-nrw/project-phoenix/services/config [setup failed]
FAIL
```

**`services/config/settingstest`: blocked.** Foreign consumers cannot import the target Go `internal` package. Migrate the consumers listed below to the owner public contract first.

- `workforce` (foreign owner; public-contract prerequisite): `modules/workforce/internal/application/timetracking [XTestImports]`.


**`services/config/sideeffects`: blocked.** Mutable composition hooks: `Registry.Register -> handlers` and `Registry.SetFailureObserver -> observeFailure` become two new path-keyed setter targets. An immutable construction contract is required; no hook allowance is carried by ADR 0020.

- `settings-platform` (same owner): `modules/settings/compose [Imports]`.

Round-3 attempt reverted cleanly. Exact remaining diagnostic:

```text
ratchet budgets passed: 11 constants, 29 budget registers, 2 exception registers
composition surface grew against base 41f264ba26dba4581c74571870217c6924d335dc:
  production|setter|modules/settings/internal/application/sideeffects|Registry.Register->Registry.handlers at modules/settings/internal/application/sideeffects/registry.go:43:2
  production|setter|modules/settings/internal/application/sideeffects|Registry.SetFailureObserver->Registry.observeFailure at modules/settings/internal/application/sideeffects/registry.go:32:2
remaining legacy violations: 56
exit status 1
```

### staff-clock

**`services/iot/staffclock`: blocked.** Public error contract/role split required: the retained `StampError{Kind, Cause}` must preserve `Error() == Cause.Error()` and its existing literals/tests. After sharing request admission, this required Error delegation remains in an application owner with no budget. Removing it needs extraction to a public contract/adapter role and corresponding import rules, not a new budget or scan exception.

- `staff-clock` (same owner): `workflows/staffclock/compose [Imports]`.

Round-3 contract probe reverted cleanly; assertions, roles, rules and budgets remain unchanged.

```text
--- FAIL: TestModulePassthroughRatchet (0.20s)
    module_passthrough_ratchet_test.go:424: Module-passthrough ratchet check failed (2 issue(s)):

        [module passthrough] modules/auditlog: 3 single-call delegation(s), budget 1.
          Delegations were added. A service method encapsulates (validation, orchestration, audit,
          tenant transaction, error mapping) or it does not exist (Rule 8). Never raise a budget.

        [module passthrough] workflows/staffclock: 1 single-call delegation(s), but the module has no budget.
          New application package or composition — a new application package gets an entry with today's count
          and shrinks from there (Rule 8); code moved out of a counted tree keeps its delegations counted.

        These counts measure a shape (four AST conditions), not the worth of any single method —
        a counted method may well be the right place for its logic. See "Known limitations" in
        the header of test/module_passthrough_ratchet_test.go before acting on an individual hit.
FAIL
FAIL	github.com/moto-nrw/project-phoenix/test	1.288s
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

**`services/database`: blocked.** Public contract/role split required: the Transaction Runtime facade exposes `DatabaseStats`, `DatabaseStatsDependencies`, `DatabaseStatsCapabilities`, `DatabaseStatsResponse`, and `NewDatabaseStats` from the moved implementation.

- `transaction-runtime` (same owner): `modules/transactionruntime [Imports]`.

Round-3 attempt reverted cleanly. Exact remaining diagnostic:

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
remaining legacy violations: 61
exit status 1
```

### workforce

**`database/repositories/workforce`: moved.** `modules/workforce/internal/adapters/workforcepostgres`

- `workforce` (same owner): `modules/workforce/compose [Imports]`.


**`modules/workforce/legacy`: moved.** `modules/workforce/internal/adapters/substitutions`

- `workforce` (same owner): `modules/workforce/compose [Imports]`.

`List` applies each filter through `applyGroupSubstitutionFilter`; the old cognitive-complexity entry and the dissolved legacy-tree budget are deleted.

## Legacy-prefixed files

File moves below preserve bytes, declarations and assertions. Existing implementation files already inside `internal/` lose only the filename prefix. Public port DTOs remain in their standing public port package. Root host binding/test-helper files remain in the root composition. Implementation in public compositions is blocked: moving only a file would split receiver types or change a public construction contract; simply renaming such a file would not put that implementation under `internal/`.

| Source at snapshot | Owner | Decision | Destination or prerequisite |
|---|---|---|---|
| `api/legacy_builders_test_helpers.go` | `root-composition` | Renamed | `api/owner_builders_test_helpers.go` |
| `api/legacy_token_adapter.go` | `root-composition` | Renamed | `api/session_token_bindings.go` |
| `database/repositories/users/legacy_composition_support.go` | `people-directory` | Moved and renamed | Moved inside `peoplepostgres` without the legacy prefix; the old per-file budget is removed. |
| `database/repositories/users/legacy_types.go` | `people-directory` | Moved and renamed | Moved inside `peoplepostgres` without the legacy prefix; the old per-file budget is removed. |
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

- `database/repositories/audit/integration` (`audit-platform`, `adapter`, both test roles `adapter-test`) moved with its repository to `modules/auditlog/internal/adapters/auditpostgres/integration`; it has no production Go files.
- `services/import/integration` (`data-import`, `test-support`) must accompany the import service. It has no production Go files.
- `models` (`legacy-composition`, `legacy-model`) and `database/repositories/studentintegration` (`legacy-composition`, `compose`) currently contain tests only. They are not counted as production relocation candidates. Their imports are still counted as consumers above.
- Deferred active work: #4082 owns Care Plan, School Structure and `workflows/careexit`; #4079 owns People Directory, Student Presence and Timetable HTTP. No package relocation in this PR takes those active implementation packages. Test imports inside these packages may only receive mechanical import-path updates when an eligible dependency moves.

## Verification and limits

Round-1 dead-code repair: `database/repositories/base.AcquireXactLock` lost its last production caller when the Workforce lock binding moved to its own adapter. The unused helper is removed. Its existing contended-lock test moves to the Workforce composition and calls that live adapter, keeping every assertion and the same SQL lock/evidence contract. No dead-code exception or legacy key is added.

Round-1 lint repair: the Device Scan wrappers use Go-promoted embedded methods instead of redundant explicit embedded-field selectors (Staticcheck QF1008). This leaves calls and behavior unchanged. The syntax-based pass-through count falls from 55 to 48 as a result; its existing budget and total are lowered by seven. This is a scanner-shape change, not seven removed forwarding methods.

Each attempted owner cluster ran `scripts/backend-architecture.sh check`. Every retained package move also passed the unchanged module file-size, function-length, cognitive-complexity, facade-width, pass-through, legacy-budget, composition-alias and HTTP-ORM ratchets. Rejected attempts are absent from the final policy and source tree. Existing alias and legacy-file/tree budgets are only lowered or removed when their measured contents disappear.

Round-3 metadata repair: local `runtime_evidence.sources` paths follow moved files, including the renamed People Directory composition support. Historical ratchet keys and measurement claims are unchanged. The repository-policy composition-reach fixture names the new Workforce package path with the same forbidden-edge assertion.

Before publication: merge `origin/development`, rerun architecture checks, and run `/tmp/orch2580/heavy.sh scripts/run-go-toolchain.sh scripts/test-changed.sh origin/development` without `--fast`, using `GOCACHE=/tmp/gocache-4086`. PR review/CI results are recorded in the PR, not inferred from this inventory.
