---
status: accepted
---

# Retire student contact copies without changing school workflows

For [#3387](https://github.com/moto-nrw/project-phoenix/issues/3387), the agreed
direction is to retire `guardian_name`, `guardian_contact`, `guardian_email`
and `guardian_phone` from the student contract and application storage access.
Linked contact people remain authoritative. Keeping a singular projection would
require a new rule for choosing among several contacts; the existing relationship
ordering and primary flags do not establish that rule.

This preserves school workflows while deliberately removing the legacy API fields.
Both binary and detailed attendance remain supported, independently of whether
bookings determine care days. Contact display and editing, direct child creation,
enrollment approval, imports and existing search behavior need verification.
Contact people without parent accounts remain supported. Enrollment-request and
messaging fields with the same names are separate contracts and stay intact.

Application work is separate from production reconciliation. Production rollout
requires a fresh reconciliation check; destructive storage cleanup belongs to
[#2760](https://github.com/moto-nrw/project-phoenix/issues/2760).

## Evidence and limits

Read-only production inspection on 2026-09-19 found release `7fe3af5` running.
Altenberge had binary attendance, the default non-booking care-day authority,
455 children and 447 children with linked contact people. All four legacy fields
were empty there. OGS am Berg had booking-authoritative care days, 208 children,
208 legacy e-mail values, 207 legacy phone values, and linked contacts for all
208 children. Across the queried student rows, all legacy names and free-text
contacts were empty; every nonempty legacy e-mail and phone matched a linked
contact using the backfill's case/whitespace and digit normalization.

This supersedes the earlier survey's unmatched contact counts as an observation,
not as a permanent rollout guarantee. It does not prove behavioral parity.
No production data was changed, and no personal contact values were recorded.

[ADR 0023](0023-student-enrollment-owner-and-directory-projection.md)'s claim
that guardian fields are adopted into `student_profiles` does not match the
current migration code: `student_owner_compatibility.go` reads and writes the
rollback archive instead. That paragraph must not justify preserving a second
contact owner.

## Search and write compatibility

Remove the legacy guardian-name search branch without replacing it with search
over linked contacts. The user explicitly accepted this: current production
matches remain unchanged because all legacy names are empty. Searching linked
contact names would be a separate feature.

There is no transitional compatibility layer for the four retired fields: no
fallback, dual write, or replacement singular projection. Migrate affected
callers together with the student API. An independently deployed consumer, if
discovered, requires coordinated cutover rather than a compatibility shim.
Retired contact input must not be silently discarded as a successful update.

## Implementation boundary

The user confirmed the completed design on 2026-09-19. This document records
accepted decisions, not completed application changes or verified behavioral
parity. Implementation follows in #3387 under the constraints above.
