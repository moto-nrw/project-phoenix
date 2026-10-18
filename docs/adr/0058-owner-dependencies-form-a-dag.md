---
status: accepted
---

# Owner dependencies form a DAG

Decision of 2026-10-15 under
[#2580](https://github.com/moto-nrw/project-phoenix/issues/2580). It supersedes
[ADR 0032](0032-the-target-may-keep-two-cyclic-components-and-they-only-shrink.md).
[#3417](https://github.com/moto-nrw/project-phoenix/issues/3417) carries it
out.

## Context

ADR 0032 allowed the target graph two cyclic components, of 17 and 5 owners,
that would only shrink. Its subset check was never written and its follow-ups
are open. The first owner outside its lists joined on 2026-09-21
(`ea5880362d`), and #2731 merged both components. On `4072c83a48` the target
graph has one cyclic component of 32 of 58 owners with 139 edges; the
migration graph has one of 39.

ADR 0032 argued that removing domain cycles was modelling work beyond the
migration. Without it the modules cannot be changed, tested or built in
order on their own, which is what the migration is for. An audit measured
which rules hold the component together: removing the cross-owner reach of
owner compositions, the platform-to-domain rules and the cross-owner rules of
domain `http` roles leaves three pairs (`people-directory` with
`student-directory-view`, `care-plan` with `enrollment` and
`timetable-legacy-view`, `student-presence` with `workforce`).

## Decision

1. **The target graph is acyclic.** `scripts/backend-architecture.sh cycles
   --graph target` reports no cyclic component. CI fails on any component;
   there is no member list and no allowance.
2. **Directions.**
   - A platform depends on platforms and the shared kernel only. Where it
     needs domain facts it declares a consumer-owned port that the root binds.
   - A domain depends on platforms and on other domains' `public` or
     `contract` roles, in one direction per pair.
   - A projection reads domains. A domain that needs a projection's result
     declares its own reader port; the root binds it.
   - A workflow depends on domains, projections and platforms. Only roots
     depend on workflows.
   - Cross-owner glue lives in the roots (ADR 0053 point 1). No owner imports
     a root in production, so roots add no cycle.
3. **Two domains that need each other** get one target direction. The other
   direction becomes a consumer-owned port bound in the root, or the shared
   behaviour moves into a workflow when it writes to both. #3417 decides the
   direction for each of the three remaining pairs.
4. **The measurement counts every production package**, including
   test-support code compiled into production. Test scopes stay out.
5. **Transition.** By [ADR 0057](0057-only-the-target-is-a-rule.md) a rule
   that closes a cycle is not target. #4070 and #3417 convert those rules into
   exact debt, so the target graph is acyclic when #3417 merges, and the CI
   check is hard from then on. The migration graph, which includes the debt,
   may stay cyclic until the keys are gone.
6. The platform triangle edge `organization-tenancy -> delivery-platform`
   (ADR 0032) is cut as decided there, under
   [#4072](https://github.com/moto-nrw/project-phoenix/issues/4072).

## Consequences

- An acyclic owner graph has a build order. The roots can construct owners in
  that order without late binding
  ([ADR 0059](0059-the-roots-hold-no-service-graph.md)).
- The Definition of Done of #2580 contains this criterion in place of the
  ADR 0032 sentence.

## Addendum (#3417): the layering of the remaining domain cycle

Decision of 2026-10-16. On `ced14cc9ff` the target graph had one cyclic
component of 14 owners. Without the domain → projection and domain → workflow
rules that point 2 already excludes, six domains still formed one component:
`care-plan`, `enrollment`, `people-directory`, `student-presence`,
`timetable-activities` and `workforce`. The directions are:

- People Directory is the lowest of them: it holds the master data the
  others read and depends on none of them.
- Care Plan sits above People Directory, Enrollment above Care Plan.
- Workforce sits above Student Presence.

Not target, therefore, and cut: People Directory's HTTP reach into Care
Plan, Enrollment and Student Presence (it moves into the root,
[#4079](https://github.com/moto-nrw/project-phoenix/issues/4079)),
`care-plan → enrollment` and `student-presence → workforce` (consumer-owned
ports the root binds,
[#4076](https://github.com/moto-nrw/project-phoenix/issues/4076)). The
remaining edges among the six (Care Plan → People Directory, Enrollment →
Care Plan, People Directory and Timetable & Activities, Student Presence →
Care Plan, Timetable & Activities → Care Plan and Student Presence,
Workforce → Student Presence and Timetable & Activities) are acyclic.

The 14 rules that closed the cycle are deleted and their 18 imports are
exact keys, the conversion point 5 prescribes. From policy epoch 48 the
policy loader rejects a target graph with a cyclic component.

## Addendum (#4084): the reach of a domain owner's implementation

Decision of 2026-10-16. #4084 moves domain logic, row mapping and repository
code out of the owner compositions into `application` and `postgres`. That
code names the owner's own `public` contract, the tenant runtime and the
public contracts of domains the owner already depends on. Measured on
`537bf3572e`, Timetable's `application` role had no rule into any of them,
while Care Plan, Enrollment, Student Presence, Workforce and five other
domains each carried a per-owner rule into their own `public` role.

From policy epoch 50 a reviewed epoch may add exactly these rules
(`reviewedInternalReachRule`, `internal/architecture/internal_reach_rule.go`):

1. `domain.application-public` and `domain.postgres-public`: a domain's
   `application` and `postgres` roles into its own `public` role, production
   only. They replace the nine per-owner rules of that shape.
2. Per owner, a domain's `application` or `postgres` role into the `public`
   role of `tenant-runtime` or `transaction-runtime`, production only. A
   domain depends on platforms (point 2); no other role of a platform is
   admitted.
3. Per owner, a domain's `application` or `postgres` role into another
   domain's `public` role, production only, when the base already grants the
   owner a production rule into that target. Such a rule adds no edge to the
   owner graph: it moves an edge the owner's composition or HTTP role holds
   into its implementation, so the direction recorded above stays, and the
   reverse direction stays forbidden.

4. The internal tests of that code (role `module-internal-test`, scope
   `internal_test` only) may reach what points 2 and 3 admit in production;
   they test unexported functions and move with the code.

5. From policy epoch 57 a domain's `port` role is a source of the same reach.
   The roots name the dependency types they build an owner's services with;
   those types live in a non-internal `ports` package instead of being
   re-exported from the composition as aliases. Admitted are
   `domain.port-public` (own `public`), per owner the `public` role of
   `tenant-runtime` or `transaction-runtime`, and per owner another domain's
   `public` role when the base grants the owner that edge. Two shapes are
   added for the move:
   - Per owner, a domain's `port` role into its own `legacy-model` role,
     production only. It is temporary: Timetable's rule
     `timetable-activities.port.legacy-model` falls away when #4086
     dissolves `models/schedule` and `models/activities`.
   - A narrowing: a source that the base already lets reach another owner's
     `compose` role may reach that owner's `port` role in the same or fewer
     scopes. The port is a subset of what the composition exposes.
   A projection or workflow target, another owner's `legacy-model`, a
   platform's non-public role and the reverse DAG direction stay rejected.

6. From policy epoch 65 a domain's `postgres` role may reach the same
   owner's `port` role, per owner and production only: the owner's
   repositories implement the repository contracts its ports declare
   (`reviewedPostgresPortRule`). The rule stays inside one owner and adds no
   edge to the owner graph. People Directory is the first user
   (`people-directory.postgres.port`, #4084).

A per-owner rule is added in the change that first uses it. The composition's
rule into the same target goes once the composition no longer needs it.
Projection, workflow and platform sources, other targets, external classes
and test scopes stay under the ordinary loosening guards.

## Addendum (#4314): owners of the cross-owner HTTP aggregates

Decision of 2026-10-18 for [#4079](https://github.com/moto-nrw/project-phoenix/issues/4079).
The existing projections own the cross-domain reads; no new owner is created.

- `presenceprojection` (`presence-legacy-view`) owns the student
  attendance-history, day-log and export joins, including
  `attendanceHistorySources` and `dayLogData`.
- `timetableprojection` (`timetable-legacy-view`) owns the enriched weekly
  instance list, including `enrichedInstance` and `weeklyInstancesResponse`.
- Care Plan's `http` role owns `CompanionConflictResponse`; its native
  `common.ErrResponse` envelope belongs to Tenant Runtime's `http` role.

The root only mounts and binds. Domain HTTP adapters use consumer-owned
reader ports bound by the root, or the projection serves the endpoint through
its own `http` role. Projection ownership permits cross-domain reads, not
writes. Layering B remains unchanged: People Directory lowest, Care Plan
above it, Enrollment above Care Plan, Workforce above Student Presence.

The widened implementation approval permits every actually used edge from a
projection to a domain or platform `public` or `contract` role, including
`presence-legacy-view -> document-rendering/contract`. Platforms sit below
all other owner kinds, so this is a downward edge. It permits neither reverse
edges nor dependencies into `application`, `postgres`, `compose` or `http`.
Use an existing reviewed acceptor where possible; otherwise add a narrow
acceptor with table tests for accepted cases and rejected reverse/private-role
cases. Record every added and deleted rule here and in the single cutover PR.
No new Root composition fields, aliases, cloned DTOs or larger budgets are
approved.

### Unpublished candidate: boundaries outside that approval

A local candidate moved the native attendance/day-log/export joins into
`modules/presenceprojection/history` and the weekly joins into
`modules/timetableprojection/weekly`, with Root operation bindings. Compilation,
focused behavior tests and the production router golden passed. Normalizing
only the middleware package-path renames reproduced every original golden
byte. The architecture check did not pass; the candidate was archived locally
and reversed, not committed or published.

The check exposed two dependencies outside the widened class:

- The native shared envelope `common.ErrResponse` is declared in
  `api/common/errors.go`. Policy classifies `api/common` as
  `tenant-runtime/http`, not `public`. Moving the companion response unchanged
  to Care Plan HTTP would need `care-plan/http -> tenant-runtime/http`.
  The candidate's child-document HTTP lifecycle reached the same package and
  also required a new Care Plan HTTP permission into `tenant-runtime/public`.
  No such rule is retained. A consumer-bound HTTP surface or a native public
  envelope cutover needs to be designed before this candidate can proceed.
- The student listing/export sorts still call `internal/collation`, classified
  as `legacy-shared/domain`. Their existing People Directory import is exact
  #4076 debt. Moving that transport into Root creates a new
  `api/students -> internal/collation` key; the package-only relocation path
  cannot carry it across an owner/role change. No private-role rule or new
  legacy key is retained. The sorting boundary also needs a valid binding.

The candidate also had unresolved test-fixture/operation-port composition
fields and stale rules. Its passing focused tests do not establish completion.
At that stop, the retained implementation had **no added or deleted rules**;
all 15 #4079 keys and the previous composition surface remained. The widened
projection-to-domain/platform approval is not revoked by this stop.


### Round-16 resolutions and bounded delivery

The implementation session explicitly approves Care Plan HTTP's same-shape,
per-owner `tenant-runtime/http` rule, restricted to `api/common`, as held by
People Directory and the other HTTP owners. Add it only when that cutover
actually uses it. No such rule is needed by this delivery.

Students HTTP stays in `modules/peopledirectory/inbound/students`; it must
consume projection readers through consumer-owned ports bound by Root.
Its existing collation debt stays at that package. A future path-only move
uses ADR 0020, retaining ownership, roles and exact debt. The retained-layout
work remains with #4086; this change does not reassign baseline carriers.
Projection readers must use existing Root values or owner constructors,
never new Root fields. These resolutions supersede the stops above, not the
no-growth and no-private-reach constraints.

The full archived candidate still required `api -> api/presence` between
Root `compose` packages and new moved-suite imports, including
`api/timetable` internal tests into `timetable-activities/port`. Neither is
covered by projection-to-public/contract approval. Its 228 new composition
keys are also not approved. The scope fallback therefore ships only the
clean `students -> requestreview/compose` retirement. #4079 remains open.
The ownership decision requested by #4314 is settled by this addendum;
settling it does not claim the other 14 imports have been removed.

Request Review now owns its native care/master-data/excused response mapping
and offering-day labels in its public capability, rather than its composition.
The original implementations move, without aliases, forwarding shims or DTO
copies. The care-impact response uses Security Runtime's existing lowercase
SHA-256 primitive; the approval composition's existing fingerprint is unchanged.
Students HTTP and queue composition call these same native mappers.

Policy epoch 72 → 73; keys 101 → 100 (#4079: 15 → 14); rules 827 → 830.
Added rules, each production-only, per-owner and from `request-review-view/public`:

- `request-review-view.public.care-plan.public`
- `request-review-view.public.care-plan.contract`
- `request-review-view.public.security-runtime.public`

Deleted rules: none. The narrow tested acceptor admits only a projection's
public source into domain/platform public or contract targets; epoch review,
acyclicity and private-role guards remain enforced. Root composition stays
1,652. Request Review compose LOC falls 1,255 → 1,069; the total compose cap
falls 34,483 → 34,297. Routes, responses, IoT and goldens remain unchanged.

[Remaining exact keys and alternatives](../architecture/4079-remaining-http-edges.md)
record the delivery boundary without another probe round.


### Final owner HTTP cutover (#4079 and #4314)

The implementation decision selects Option 1 for every remaining key. This
supersedes the bounded delivery's restriction that all student routes stay
in People Directory HTTP. Each transport belongs to the owner of the data
and operations it serves, including projection and workflow HTTP roles.
Root mounts these handlers from existing values, without new Root fields.
The exact destinations are recorded in the remaining-edge map.

An owner's HTTP transport uses its own public/contracts and its owner's
existing target directions. Projection/workflow HTTP may consume domain and
platform public/contracts. The moved handlers retain same-or-narrower native
successors of People Directory's existing HTTP rules: Care Plan reaches
People Directory public/departure and Identity's request-review contract;
Student Presence reaches People Directory public; both retain their existing
Delivery, Security and Settings platform targets. The shared tenant HTTP
runtime and session-token runtime remain exact package targets. Tests retain
native public/contracts and exact HTTP runtime targets in their original
scopes, with source-package selectors.

`reviewedOwnerHTTPRule` admits only these reviewed shapes in an increased
epoch. Every source is pinned to its classified HTTP package and owner. The
finite native-successor set preserves an existing target rule; arbitrary new
domain directions, reverse/projection/workflow targets, private roles,
external classes and widened scopes remain rejected. Table tests pin both
accepted and refused shapes. Acyclic target-graph checks remain unchanged.

The original data/response definitions move once. Root binds native response
operations between Care Plan, Presence projection and Request Review; it
binds comparison functions without moving the retained collation import to
Root. There are no aliases, cloned DTOs, private-target grants or budget
increases. All 14 #4079 keys retire together in one PR; #4314's ownership
decision is implemented, not only recorded.

Policy epoch 73 → 74; baseline keys 76 → 62; rules 833 → 902.
Root composition remains 1,644 after integrating development. The integrated compose cap remains 33,216 (downstream development lowered the original 34,060).

[Verification and every added/deleted rule](../architecture/4079-owner-http-evidence.md).
