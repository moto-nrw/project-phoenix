---
status: accepted
---

# Domain applications no longer call workflows

Decision of 2026-10-24 for
[#4825](https://github.com/moto-nrw/project-phoenix/issues/4825). It removes
the domain-to-workflow runtime bindings that
[ADR 0071's addendum for #4740](0071-principle-owner-dependencies-point-one-way.md#addendum-2026-10-23-explicit-care-resumption-entry-4740)
records as ungranted debt. PR #4833 removed Care Plan's share visibility and
People Directory's birthday opt-out; this decision covers the rest of the
inventory. It adds no owner, no policy rule and no runtime exception. The
Care Exit Roster/Bookings E2 callbacks of ADR 0071 stay the only granted
exception.

## Decision

Each binding takes one of the two paths #4825 names: (a) the capability
belongs to a domain owner, or (b) the flow becomes a workflow entry reached
from HTTP or the root.

| Binding | Path | Result |
|---|---|---|
| Enrollment → Data Import `MapRelationshipType` | (a) | The relationship vocabulary belongs to People Directory (`contact.MapRelationshipType`). Enrollment's root rule and Data Import's ports call it. |
| Care Plan → Parent Portal `SameDayCutoff` | (a) | `carerequests.SameDayCutoff` is a Care Plan value. The request inputs carry it by value. The workflow builds it with Care Plan's constructor and its injected clock, not with a workflow method. |
| Workforce → Staff Offboarding `NewAbsenceDeletion` | (a) | Workforce builds the deletion record of the absences it deletes. The root binds only that value and the Audit Platform append. |
| School Membership runs a Staff Offboarding closure | (a) | School Membership offers `LockLiveStaff`. The workflow locks first and then writes in the same caller transaction. |
| School Structure → Shift Plan Sync `GroupHandovers` | (a) | The handover store binds Workforce's group-substitution commands like the store's other methods. The root translates only the duplicate error. |
| File Storage → Parent Portal `GuardianAudience` | (b) | File Storage offers tenant-scoped attachment reads as `TenantAttachmentReads`, outside its `Capability`. The root hands them only to the portal's `AnnouncementAttachments` entry. The parent download routes call that entry through their own `GuardianAttachments` port. The entry checks the audience, reads the rows, checks again and only then opens the bytes. |
| School Structure → Shift Plan Sync schedule adapter | (b) | The `ScheduleSubstitutions` entry authorizes, runs the Timetable deviation writes in the request transaction and announces them after commit. The root routes the schedule target to it and the group targets to School Structure's module. |
| Workforce → Shift Plan Sync `SickCascade` | (b) | The absence writes that can cascade a sick report return a `SickCascadePlan`. The `SickReports` entry runs the write and applies the plan in one tenant transaction. The Workforce routes reach it through `OwnAbsenceWrites` and `AdminAbsenceWrites`. |

The register also listed two families as #4825 direction-accounting debt.
Both were transport entries typed with a domain interface. Timetable's
pickup-extension routes now consume their own `PickupExtensionDecisions`
port, and Care Exit returns its own decision type. Data Import's observed
opening commands implement Data Import's own ports instead of Workforce's
interfaces.

### Retained behaviour

- Lock order, transaction scope, error identities, error texts and HTTP codes
  stay the same. A failed sick cascade still aborts the absence write.
- Sick-report deletion and merging now release or re-point the provenance
  stamps after Workforce's row writes instead of before them. Both steps run
  in the same tenant transaction. The stamps have no foreign key, and the
  cascade reads no absence row. Inside that transaction the absence row is now
  written before the cascade takes the staff-shift and substitute-day locks.
  The per-staff absence lock still comes first and serializes every absence
  write and cascade of the same staff member, so no other writer can hold a
  shift or day lock while it waits for that absence row. An unplanned sick
  cascade is refused at write time instead of at service construction.
- A parent attachment download now reads the attachment row twice: once
  before the second audience check and once when the bytes are opened. A row
  deleted between the two reads answers 404.
- The worker no longer builds the parents portal or the meal plan. It needed
  them only for the file store's former audience binding.

## Consequences

- No `debt` entry in `backend/architecture/command-effects.json` cites #4825
  any more. HTTP ports into workflows are transport entries. This follows the
  existing distinction in `validateEffectDirection`.
- The re-keyed entries cite this ADR as `approval_adr`. They are the
  HTTP-to-workflow commands classified `workflow`, plus the five unresolved
  `carerequests.SameDayCutoff.now` boundaries. They replace the `api.now`
  boundary of the former `DayCutoff` port; the Parent Portal workflow fills
  the field with its injected `Config.Now`. The boundary is not approved as a
  query or leaf. The permission checks of the Workforce substitutions HTTP
  port resolve to `permissions.Principal.HasPermission` and are queries.
- Platform triggers remain the open question of ADR 0071. This decision does
  not settle it.
