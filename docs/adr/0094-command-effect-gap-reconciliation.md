---
status: accepted
---

# Reconcile callable evidence and workflow debt without a reusable gate exemption

Decision of 2026-10-23 for [#4836](https://github.com/moto-nrw/project-phoenix/issues/4836).
The [ADR 0083 gap-closure addendum](0083-ratchets-protect-invariants-and-expose-command-effects.md#gate-gap-closure-2026-10-23-4836)
records the scanner changes, exact register delta, evidence and limitations.

Existing callable diagnostics gain assignment/closure evidence. Existing
`workflow` labels outside the seven Care Exit Roster/Bookings E2 command/lock
identities become `debt`, citing #4825. Neither change introduces a runtime
command, a new exception or a permission to call workflows from domains.
The retained seven labels record the bounded E2 protocol described by ADR 0067
and ADR 0071. PR #4753 was unmerged at the initial reconciliation and has now
entered the base. This decision does not independently amend ADR 0071.

The changed entries cite this newly numbered ADR as `approval_adr`. This uses
the existing immutable-base approval contract rather than teaching the checker
another one-time census heading or a fingerprint-migration bypass. After this
ADR enters the base it cannot approve another exception, changed callback or
new workflow label. The counterprobe removes this decision and must fail.

A semantic edit to a still-unresolved closure requires fresh ADR evidence even
if its author believes no foreign effect changed. We retain that review cost:
without resolving runtime dispatch the checker cannot prove that claim.
Comments, formatting and unrelated closures do not change its fingerprint.

## Development merge reconciliation

Development commit `3e625a1f5d` already uses ADR 0091 for external caregiver
records. The first reconciliation used 0092. Development `d780efb392` now
uses 0092 for the tenant data-lifecycle catalogue, and open PR #4835 uses
0093. This decision therefore moves to 0094; unchanged base approvals continue
to cite 0091. Development and all open PR file lists were checked. The number
must still be new at merge time: an ADR already in the immutable base cannot
approve these changes. Regeneration removes all 15 obsolete branch
identities, including the old GuestRepository callbacks, rather than preserving
them as debt. Replacement identities retain the base's reviewed classifications.

Against that development commit the merged register contains 5,350 entries:
3 added, 0 removed and 48 changed fingerprints. At that historical stage there were 7 workflow, 92 debt
and 642 unresolved entries. The 70 original gap-repair approvals cite 0094, including
the renamed callable whose assignment fingerprint changes. The original
pre-merge measurements in ADR 0083 remain historical evidence.

The subsequent integrations of `c69f73f218` (#4755) and `b8c42a3cda` (#4832) leave these register counts
unchanged. The tuple-argument evidence correction adds no production identity
or digest change; its normal/variadic fixtures are red before and green after.

Final tuple-corrected warm `check` against full merge-base
`b8c42a3cdac13d1270017eee39fcd79a207b8fba`: 33.80 s wall, 93.69 s user,
20.87 s system, after an untimed warmup and without concurrent own tests/lint.
The fresh-base no-new-ADR counterprobe remains red (67 existing changes).

## Runtime-direction validation (round 2)

Domain → Workflow direction is checked from policy owner kinds, independently
of the effect. `leaf` and ordinary `query` cannot grant that direction.
`workflow` is limited to Care Plan's seven exact Care Exit Roster/Bookings E2
command/lock targets; named E1–E5 and explicit `debt` still require their ADR
and fresh immutable-base approval. Query effects may carry debt or a named
exception, but cannot become ordinary leaves.

The 18 existing query-labelled workflow targets need explicit classification.
Four existing Care Exit Roster/Bookings reads within the now-merged ADR 0071
grant become `query/E2`, citing ADR 0067. The other 14 become `query/debt`,
citing ADR 0083 and #4825. This includes ShareRecipients, SameDayCutoff,
MapRelationshipType, GuardianAudience/Portal, ScheduleAdapter.Overview,
workflow-owned construction/import helpers. Care Exit's four granted reads
remain explicit E2 exceptions, not ordinary domain dependencies. The register
is conservative may-bind accounting, not evidence that every supplied method
runs. It grants no new read callback exception and does not change ADR 0071; it
records the four reads already granted by merged PR #4753.

Against development `9598e00b31741b040445aae6a14a29848b0ef6ac`, regeneration
retains 5,349 entries: 7 workflow, 105 debt, 22 E2 and 642 unresolved; 87 entries
cite approval ADR 0094. Effects remain 2,783 query, 1,613 mutation, 298 lock,
7 scope, 1 durable, 5 notice and 642 unresolved. The 640 unresolved count in
ADR 0083 describes its original pre-merge census, not this register.

## Explicit remaining boundary: domain-owned callable fields (M3)

**This PR does not close #4836.** The scanner stops at a concrete receiver of
the source owner. Therefore the root closure passed to
`communication.NewStaffColleagueReads` and stored as
`staffColleagueReads.list` is not inventoried: adding an Identity & Access
`GrantGuardianTenantAccess` call inside that closure can still pass `check`.
The assignment fingerprints repaired here protect diagnostics that the scanner
already reaches, not every closure supplied to a domain constructor.

Before the #4753 base integration, a temporary constructor/field fixture
reproduced zero entries before a
prototype fix. The prototype fingerprints such root-supplied fields as
`unresolved` and rejects a changed closure, while ignoring a comment-only edit.
Even the interface-provider-scoped version exposes 418 additional register
identities and over 4,500 register lines, beyond this round's roughly 800-line
scope limit. It was not retained or hidden behind an owner/type allowlist.

Follow-up proposal: give domain-owned callback inputs their own typed census,
then trace constructor parameters, aliases and field assignments to their root
closure or root helper. Cover direct fields, same-owner interface providers,
forwarding helpers and comment-only edits with immutable-base CLI fixtures;
review and approve the newly exposed unresolved entries in a separate fresh
ADR. Do not traverse all foreign domain implementations as if they were root
adapters. Generic/anonymous-only dispatch and platform/protocol effects remain
separate limits too.

Local identifier renames inside an already reached unresolved closure also
change its normalized syntax fingerprint and conservatively require a fresh
ADR. Comments, positions and formatting remain excluded.

## Round-2 repository counterprobes

The first round-2 default and explicit merge-base checks passed against `d780efb392` with
0 legacy violations, composition 0 → 0 and 3,522 exact pass-through sites.
Removing only ADR 0094 rejects 85 existing-entry changes. A static-only
insertion of `GuardianAccessCommand.GrantGuardianTenantAccess` into the real
`StaffColleagueReads` root closure still passes, confirming M3 rather than
claiming it closed. The probe is not executed as application code. Both probes
restore the exact original file bytes.

## Final base integration (#4753)

While the first round-2 quorum run was finishing, #4753 and #4840 entered
development (`9598e00b31`). The register conflict is resolved by removing the
obsolete `CareResumption.Resume` domain binding, never keeping it as debt.
The generator removes no other reviewed binding. Four query classifications
now explicitly record the existing E2 read grant from the merged ADR 0071;
only its other 14 workflow reads become direction debt. The seven workflow
command/lock labels stay unchanged. PR #4833 is still open at this integration.

The preceding 5,350/110-debt/88-approval figures and first quorum result refer
to the earlier head; the current counts above supersede them. This is a base
reconciliation, not a new callback grant or an edit to ADR 0071 itself.
