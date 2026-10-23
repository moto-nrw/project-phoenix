# Command-effect reconciliation (#4739, first slice)

Inspected `origin/development` at `c00bd12eb7` (includes #4736 / PR #4742),
then generated from this PR. Paths start at `backend/`.

The [machine inventory](../../backend/architecture/command-effects.json) has
**128 exact may-bind entries: 79 query, 41 mutation, 6 lock, 2 unresolved**.
Of the 47 resolved non-query entries, **36 are E1 and 11 are leaf**. No E1
coordinator is auto-approved as a leaf. The two unresolved entries remain visible
known limitations. These are port/binding/target-method identities, not 128
runtime owner-pair edges, nor a replacement for 40 manually grouped operations.

[ADR 0089](../adr/0089-enrollment-interface-bindings-have-reviewed-command-effects.md)
defines the scope: Enrollment interface bindings to foreign domains, workflows
and projections, supplied by every typed production/root package. Platform
targets and other source owners remain outside. The hand inventory was never
a whole-repository census; its 40 rows group protocols and transitive owners.

## All 40 hand-inventory rows

Numbers refer to the original table order. Seven rows map to direct bindings,
one records a transitive first-hop limit, and 32 have a different source owner.
“Outside” is not a positive result about the protocol or absence of writes.

| Row | Manual edge / operation | Machine comparison and reason |
|---:|---|---|
| 1 | Enrollment decision → Care Plan materialization/adjustment/pickup | Covered: `DecisionBookings` → Care Plan materialization, adjustment, pickup and locks. E1, not leaf. |
| 2 | Enrollment decision → Timetable sourced templates, via Care Plan | First hop only: Enrollment resync interfaces → Care Plan `BookingMaterializationCapability`. The later Care Plan → Timetable call is outside this source-owner cut. Do not invent a direct Timetable edge. |
| 3 | Enrollment decision → Identity guardian tenant access | Covered: `DecisionGuardianAccess.GrantGuardianTenantAccess` → Identity `Owner`. E1; account reads remain queries. |
| 4 | Enrollment decision → People student enrollment/profile | Covered: the alias to `peopledirectory/enrollment.Commands` resolves to People `Capability` create/renew/profile/class-gate/student-read methods. E1; locking reads are locks. |
| 5 | Enrollment approved-child sync → People/Care Plan | Partly covered: People person/guardian/account-link and Care Plan reconciliation bindings remain E1. Direct weekly pickup/arrival writes to Timetable are additionally visible. `DepartureCompanions.DeleteCompanionEdges` is the one exact unresolved function field, explicitly not classified as a query. After-commit orchestration and platform audit/notices are outside this cut. |
| 6 | Care end/cancel → School Membership | Outside this slice: source is not Enrollment. Preserve the hand inventory and its Leaf/W/E classification; no typed-effect coverage is claimed here. |
| 7 | Care resume → School Membership | Outside this slice and reserved for #4740: Care resumption is workflow-backed. This PR neither changes nor silently reclassifies that binding. |
| 8 | Identity school identity → People person/link/tag | Outside this slice: source is not Enrollment. Preserve the hand inventory and its Leaf/W/E classification; no typed-effect coverage is claimed here. |
| 9 | Identity school identity → School Membership staff/teacher | Outside this slice: source is not Enrollment. Preserve the hand inventory and its Leaf/W/E classification; no typed-effect coverage is claimed here. |
| 10 | Identity guardian profile create/link → People | Outside this slice: source is not Enrollment. Preserve the hand inventory and its Leaf/W/E classification; no typed-effect coverage is claimed here. |
| 11 | Identity guardian role upgrade → People relationship | Outside this slice: source is not Enrollment. Preserve the hand inventory and its Leaf/W/E classification; no typed-effect coverage is claimed here. |
| 12 | Identity guardian link insert → People (and its pickup/access halves) | Outside this slice: source is not Enrollment. Preserve the hand inventory and its Leaf/W/E classification; no typed-effect coverage is claimed here. |
| 13 | Identity demo access → Organisation demo orders/retirement/last-use | Outside this slice: source is not Enrollment. Preserve the hand inventory and its Leaf/W/E classification; no typed-effect coverage is claimed here. |
| 14 | People student create/profile/renew → School Membership | Outside this slice: source is not Enrollment. Preserve the hand inventory and its Leaf/W/E classification; no typed-effect coverage is claimed here. |
| 15 | People student create/profile/renew → Care Plan | Outside this slice: source is not Enrollment. Preserve the hand inventory and its Leaf/W/E classification; no typed-effect coverage is claimed here. |
| 16 | People relationship/pickup patch → Care Plan | Outside this slice: source is not Enrollment. Preserve the hand inventory and its Leaf/W/E classification; no typed-effect coverage is claimed here. |
| 17 | People relationship creation/permissions → Identity access | Outside this slice: source is not Enrollment. Preserve the hand inventory and its Leaf/W/E classification; no typed-effect coverage is claimed here. |
| 18 | People staff edits → School Membership staff | Outside this slice: source is not Enrollment. Preserve the hand inventory and its Leaf/W/E classification; no typed-effect coverage is claimed here. |
| 19 | People teacher edits → School Membership caregiver | Outside this slice: source is not Enrollment. Preserve the hand inventory and its Leaf/W/E classification; no typed-effect coverage is claimed here. |
| 20 | School Membership staff → Workforce employment | Outside this slice: source is not Enrollment. Preserve the hand inventory and its Leaf/W/E classification; no typed-effect coverage is claimed here. |
| 21 | Workforce staff edits → School Membership employment fields | Outside this slice: source is not Enrollment. Preserve the hand inventory and its Leaf/W/E classification; no typed-effect coverage is claimed here. |
| 22 | Workforce staff name/contact edits → People | Outside this slice: source is not Enrollment. Preserve the hand inventory and its Leaf/W/E classification; no typed-effect coverage is claimed here. |
| 23 | School Structure class/group assignments → School Membership | Outside this slice: source is not Enrollment. Preserve the hand inventory and its Leaf/W/E classification; no typed-effect coverage is claimed here. |
| 24 | School Structure group handover → Workforce | Outside this slice: source is not Enrollment. Preserve the hand inventory and its Leaf/W/E classification; no typed-effect coverage is claimed here. |
| 25 | Timetable check-in/out/move → Student Presence | Outside this slice: source is Timetable. Preserve the current #4736/PR #4742 authority-A audit and tests, not the older unresolved-bypass wording. |
| 26 | Timetable cancellation → Communication announcement | Outside this slice: source is not Enrollment. Preserve the hand inventory and its Leaf/W/E classification; no typed-effect coverage is claimed here. |
| 27 | Student Presence check-in status clear → Care Plan live flags | Outside this slice: source is not Enrollment. Preserve the hand inventory and its Leaf/W/E classification; no typed-effect coverage is claimed here. |
| 28 | Student Presence NFC start → Workforce check-in | Outside this slice: source is not Enrollment. Preserve the hand inventory and its Leaf/W/E classification; no typed-effect coverage is claimed here. |
| 29 | Care Plan offering roster/source change → Timetable | Outside this slice: source is Care Plan, including #4740-reserved surfaces. Use development as the authority and regenerate after integration; no Care Plan/workflow source changes here. |
| 30 | Alleged Care Plan booking materialization → People class/status writes | Outside this slice: source is Care Plan. Preserve the manual correction: this binding reads/locks People rather than writing its rows. A future slice must label its locking reads, not invent a People mutation. |
| 31 | Care Plan master-data approval → People student/person | Outside this slice: source is not Enrollment. Preserve the hand inventory and its Leaf/W/E classification; no typed-effect coverage is claimed here. |
| 32 | Care Plan HTTP child create → People person/student/guardians | Outside this slice: source is not Enrollment. Preserve the hand inventory and its Leaf/W/E classification; no typed-effect coverage is claimed here. |
| 33 | Enrollment intake → Care Plan booking intervals | Covered: `CareBookingCommands.RecordCareBookings` → Care Plan `RecordCareOfferingBookings`, leaf. The same supplied method serves rows 33 and 34; the machine does not pretend these are two method identities. |
| 34 | Enrollment rollover → Care Plan booking intervals | Covered by the same exact binding as row 33. The source rollover operation remains separate manual protocol evidence; its approval is not reclassified from E1. |
| 35 | Enrollment form-change apply → Care Plan replace bookings | Covered: `CareBookingChanges.ReplaceCareBookings` → Care Plan `ReplaceCareOfferingBookings`, leaf. The offering gate/reconciliation and approved-child-sync interfaces are separate E1 entries. |
| 36 | Organisation school creation → Device Fleet | Outside this slice: source is not Enrollment. Preserve the hand inventory and its Leaf/W/E classification; no typed-effect coverage is claimed here. |
| 37 | Organisation device transfer → Device Fleet | Outside this slice: source is not Enrollment. Preserve the hand inventory and its Leaf/W/E classification; no typed-effect coverage is claimed here. |
| 38 | Organisation person retirement → People | Outside this slice: source is not Enrollment. Preserve the hand inventory and its Leaf/W/E classification; no typed-effect coverage is claimed here. |
| 39 | Organisation provisioning/retirement → Identity | Outside this slice: source is not Enrollment. Preserve the hand inventory and its Leaf/W/E classification; no typed-effect coverage is claimed here. |
| 40 | Organisation school creation → Timetable categories | Outside this slice: source is not Enrollment. Preserve the hand inventory and its Leaf/W/E classification; no typed-effect coverage is claimed here. |

## Additional entries and limits

1. **Finer granularity:** queries and locks are separate method entries; one
   broad supplied interface may supply several targets. Canonical aliases,
   embedded interfaces, promoted methods and method values are not dead ports.
2. **Additional exposed surfaces:** Enrollment's native Care Plan catalog
   bindings include create/update/delete/clone and rollover cloning, omitted
   from the issue-example table. The target owns its catalog guards and gates.
   These are supplied interface methods, not proof all are called by Enrollment.
   Direct Timetable weekly pickup/arrival bindings and People guardian/profile/
   phone/audit halves are also now explicit rather than hidden by grouped rows.
3. **Conservative assignment union:** the same interface can be assigned in
   another root or testhook. For example `modules/studentpresence/http.TenantSettings`
   appears as a query supplier through a structurally identical anonymous root
   settings interface. This is an overapproximation, not a discovered Enrollment
   write into Presence. `dataimport.MapRelationshipType` is a pure function,
   not workflow command coordination. Foreign validation/value helpers are queries.
4. **Two unresolved callable boundaries:**
   `roots/owners/enrollment_decision_people.go:566` calls `c.delete`.
   `enrollment_decision_composition.go:83` receives the function and
   `root_enrollment_services.go:99` supplies `carePlan.DeleteCompanionEdges`.
   The exact register preserves the E1 context and this evidence under ADR 0089;
   the static scanner does not infer that field's target owner or call it safe.
   Separately, `api/school_directories.go:185` calls generic helper parameter
   `find`, supplied at `:151` by `d.schools.FindSchoolBySlug`. Its typed selector
   is inventoried as a query, but generic function-parameter flow remains an
   exact reviewed unresolved entry, not an inferred query.
5. **Transitive protocols remain manual:** calls stop at foreign contracts.
   Enrollment → Care Plan → Timetable and Enrollment → People → Membership/
   Care Plan are not flattened. Scope/transaction participation, authorization,
   failure propagation, savepoints, callbacks and lock-order correctness retain
   their existing behavior/concurrency tests and review requirements.

No new confirmed runtime defect was found; none of these coverage limits
licenses a coordinator migration or a new E1–E5 protocol.

## Proposed follow-up issues, not created

1. Extend typed interface binding effects to the remaining 17 domain source
   owners and reconcile E2–E5/workflow-backed ports, including the #4740 seam:
   **2–3 engineering days**, inventory review additional to code size.
2. Resolve function-valued and multi-result/variadic/generic flows and reduce
   type-level union to instance-sensitive root evidence; add direct foreign
   call-site coverage and relocation fixtures: **2–3 days**.
3. Add platform durable/scope/notice effects and reviewed transitive protocol
   evidence (transaction/savepoint boundaries, authorization, lock order,
   callbacks), with changed-effect/base fixtures: **2–4 days**.

These estimates are follow-up implementation/review preparation, not promises.
This PR uses `Refs #4739`, not `Closes #4739`.
