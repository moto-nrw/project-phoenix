# #4079 remaining exact HTTP edges

This is the approved bounded fallback, not a claim that the full cutover is
impossible. #4314's ownership decision is recorded in ADR 0058. The rejected
candidate is unpublished; no permissions or composition identities from it
are carried into this change.

All entries below retain `production|imports.forbidden`, the module prefix
`github.com/moto-nrw/project-phoenix/`, and carrier
`https://github.com/moto-nrw/project-phoenix/issues/4079`. Source and target
columns are exact suffixes, not package-family exceptions.

The full candidate's exact additional blockers included
`production|imports.forbidden|github.com/moto-nrw/project-phoenix/api|github.com/moto-nrw/project-phoenix/api/presence`
(Root compose → Root compose),
`internal_test|imports.forbidden|github.com/moto-nrw/project-phoenix/api/timetable|github.com/moto-nrw/project-phoenix/modules/timetable/ports`,
and 228 new composition fields. For example, the moved students resource added
`production|field|api/students|ResourceConfig.ActiveService` rather than
retaining the existing field identity. New Root fields are forbidden; no
budget or private-target rule is added to hide these blockers.

## Retained keys

| Source | Target | Exact dependency still blocking retirement | Option 1 | Option 2 |
|---|---|---|---|---|
| `modules/peopledirectory/inbound/students` | `modules/careplan` | Schedule, companion, lifecycle and document signatures and response facts name Care Plan; a People Directory → Care Plan target rule reverses the accepted layering. | Mount owner-owned Care Plan HTTP handlers from Root, reusing current Root capabilities. | Move native read aggregates into Presence projection and bind consumer-owned readers; keep writes in Care Plan HTTP. |
| `modules/peopledirectory/inbound/students` | `modules/careplan/absencerecords` | Attendance-history/day-log joins name native Care Plan absence rows. | Move the native joins and their existing result types into Presence projection; bind a reader through existing wiring. | Serve those reads from Presence projection HTTP and mount them from Root. |
| `modules/peopledirectory/inbound/students` | `modules/careplan/carerequests` | Care-request decisions and conflict/error mapping name the native contract. | Care Plan HTTP owns these decision handlers; Root mounts them. | A consumer operation port returns the existing native response through Request Review, with Care Plan retaining the decision and error mapping. |
| `modules/peopledirectory/inbound/students` | `modules/careplan/excusedrequests` | Excused decisions, queue filters and response facts still name the contract. | Mount Care Plan's excused-request HTTP surface from Root. | Deepen Request Review's reader/decision-result boundary and bind the existing owner operations without copying response types. |
| `modules/peopledirectory/inbound/students` | `modules/careplan/masterdatarequests` | Master-data decision handlers name native review items/errors. | Move decision transport into Care Plan HTTP, mounted by Root. | Bind a Request Review consumer operation returning its existing native response, with owner-side error classification. |
| `modules/peopledirectory/inbound/students` | `modules/careplan/parentrequests` | Bulk/conflict/lifecycle handlers name native parent-request operations and results. | Move those transports into Care Plan HTTP and retain route mounts in Root. | Deepen the existing Request Review boundary for aggregate results and bind consumer operations without adding Root fields. |
| `modules/peopledirectory/inbound/students` | `modules/enrollment` | Enrollment decision/form-schema fields and handlers name the higher owner's public facts. | Mount Enrollment HTTP operations from existing Root wiring. | Split read results into the existing projection and bind write operations at the mount without exposing Enrollment DTOs to People Directory HTTP. |
| `modules/peopledirectory/inbound/students` | `modules/grouplive` | OGS live handlers and resource field name the projection query/result. | Let Group Live HTTP serve its own aggregate, mounted by Root. | Bind an HTTP consumer reader after relocating the existing wire result to the projection-owned response boundary, not cloning it. |
| `modules/peopledirectory/inbound/students` | `modules/requestreview` | Queue/read handlers still name the projection's native query and wire results. | Mount Request Review HTTP for its native queues/responses from Root. | Bind a consumer reader to a projection-owned response handler at the existing resource construction seam, removing direct projection type references. |
| `modules/peopledirectory/inbound/students` | `modules/studentpresence` | Presence/history/privacy fields and native read/write results remain coupled to the higher owner. | Presence projection owns history/day-log/export joins and serves them through a bound consumer reader; Presence HTTP owns writes. | Mount Presence projection HTTP reads and Student Presence HTTP operations directly from Root. |
| `modules/peopledirectory/inbound/students` | `workflows/studentdeletion` | Deletion/impact handlers hold the native workflow and its refusals. | Student Deletion HTTP owns the workflow transport and Root mounts it. | Bind a route-sized deletion operation/response at the mount, retaining workflow error mapping outside People Directory. |
| `modules/studentpresence/inbound/presence` | `modules/supervisiondashboard` | `SupervisionDashboardService` and dashboard response alias name the projection. | Supervision Dashboard HTTP owns the endpoint, mounted from existing Root values. | Bind a consumer reader/response boundary to the existing projection result, removing the alias instead of cloning the result. |
| `modules/studentpresence/inbound/presence` | `workflows/openroommove` | `OpenRoomMove`, `Move`/`Actor` and workflow refusal mapping couple domain HTTP to workflow. | Open Room Move HTTP owns that handler and error mapping; Root mounts it. | Bind a consumer operation in the existing move field using native Student Presence move request/result vocabulary, with workflow-side translation. |
| `modules/timetable/http` | `modules/classday` | `SlotListsService`, list params/results and refusal mapping name Class Day projection. | Class Day HTTP owns the slot-list handlers, mounted from existing Root wiring. | Bind consumer list operations and relocate the native transport parsing/response boundary into Class Day, without cloning its result DTOs. |

Neither option is implemented or represented as verified here. Each requires
preserving assertions, route/middleware goldens and composition identities.
Students' existing `internal/collation` key is untouched at its current path;
retained-layout cleanup belongs to #4086, not to a new Root private import.
