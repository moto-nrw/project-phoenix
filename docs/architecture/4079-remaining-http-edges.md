# #4079 owner HTTP cutover

> **Historical record.** State at `c32e6f616c` (2026-10-18). #2580 and #4069 are closed;
> counts, budgets and open issues below describe that state, not today.
> Final state: [`backend/architecture/WORKPLAN.md`](../../backend/architecture/WORKPLAN.md#endstand).

The implementation decision is **Option 1 for all 14 remaining exact keys**.
It supersedes the alternatives and stops recorded by the earlier bounded
slice. ADR 0056 makes Root the HTTP host; ADR 0053 point 2 covers the owner
HTTP roles it mounts. The root uses existing wiring values and adds no
composition fields.

| Original source | Target owner HTTP |
|---|---|
| People Directory students: schedules, companions, lifecycle, documents, care/excused/master-data decisions, parent-request conflicts | Care Plan |
| People Directory students: enrollment extra fields | Enrollment |
| People Directory students: OGS live and navigation | Group Live |
| People Directory students: review queues | Request Review |
| People Directory students: history, day-log, enriched list/detail/export reads | Presence projection |
| People Directory students: presence/status/privacy writes | Student Presence |
| People Directory students: deletion, impact and purge | Student Deletion workflow |
| Student Presence presence: dashboard and courtyard status | Supervision Dashboard |
| Student Presence presence: open-room move | Open Room Move workflow |
| Timetable: slot lists | Class Day |

All 14 exact #4079 baseline entries are removed. People Directory HTTP keeps
its own routes. Its existing `internal/collation` debt remains at the same
path under #4076; Root binds comparison operations into the read projection.
No private target or new legacy key replaces the retired imports.

The actual shared student state, class-list row, activity group and status-day
response definitions move once to native owner public packages. No aliases
or cloned DTOs bridge the transports. Root binds response rendering and
status-write operations between owner handlers through native values and
scalar operation signatures, not another Root service container.

Tests follow their handlers. Mixed read/write tests compose the real owner
handlers through the typed root. The original 671 tests retain all 3,437
assertions after mechanical qualifier and fixture-name normalization. One
mixed notification test keeps its two subtests in their respective owners.
Birthdays tests remain at their unchanged People Directory location.

See [cutover evidence](4079-owner-http-evidence.md) for checks and the exact
rule delta. Paths, methods, middleware order, authorization, tenant
transactions and responses retain their previous contracts.
