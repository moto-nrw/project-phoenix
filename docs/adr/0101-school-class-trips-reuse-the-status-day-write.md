---
status: accepted
---

# School class trips reuse the status-day write

#4799 lets a Lehrkraft report a class trip in moto schule. A trip without a
return time is the status "Klassenfahrt" for every chosen child and day, the
same status the OGS's own class-trip dialog writes. Class Day therefore takes
Student Presence's existing status-day write instead of a second write path.

## Decision

The composition root binds Class Day's `ClassTripStatusDays` port in
`buildClassDayViews`. Student Presence's `SchoolClassTrips` adapter records
the trip with `StatusDays.BulkCreateForDates`, under the same student and
exception-day locks as the OGS dialog. It re-checks class membership under
those locks, publishes the same SSE event and wakes the guardians' view. A
trip with a return time writes Care Plan arrival exceptions instead and does
not touch the status days.

No owner, permission, effect or lock order changes. Class Day gains no direct
dependency on Care Plan's lock. That lock stays inside the Student Presence
command, which Class Day already calls only through its port.

## Register delta and approval

`buildClassDayViews` now binds `presence.StatusDays`. That makes its body part
of the binding evidence of every Student Presence status-day binding, so the
command-effect register changes 9 binding fingerprints and nothing else.
No entry is added or removed, and effect, classification, ADR and target stay
the same.

- Student Presence takes Care Plan's `LockExceptionDay` through
  `Capability` and `Owner` from `BulkCreateForDates`, `CreateForDates` and
  `UpsertReported` (6 debt entries).
- The `StudentStatusDayService.now` clock field stays unresolved from
  `BulkCreateForDates`, `CreateForDates` and `DeleteStatusDay`
  (3 unresolved entries).

These 9 entries cite `approval_adr: "0101"`. The counterprobe that removes
this ADR rejects exactly those 9 entries. This approves only the changed
evidence; it does not review the Student Presence → Care Plan lock protocol.
That review remains follow-up work under ADR 0098. Once this ADR is in the
base, it approves nothing further.
