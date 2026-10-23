---
status: accepted
---

# Principle: only the target is a rule, and debt leaves with evidence

Principle ADR of 2026-10-19 under
[#2580](https://github.com/moto-nrw/project-phoenix/issues/2580), written for
[#4091](https://github.com/moto-nrw/project-phoenix/issues/4091). It states the
rules for the ratchet, recorded debt, reviewed policy changes and migration
evidence that still hold. The ADRs it cites keep their context, measurements
and history; this ADR changes none of them.

## Decision

1. **A rule in `policy.json` states the target and nothing else.** It permits
   only an import the principles of ADR 0066 to ADR 0071 keep. Never a rule,
   for any pair of owners: a reach into another owner's `application`,
   `domain`, `adapter`, `postgres`, `cli` or `legacy-model` role, or a reach
   the owner composition, direction and cycle rules of
   [ADR 0068](0068-principle-owners-compose-themselves-and-the-roots-only-wire.md)
   and [ADR 0071](0071-principle-owner-dependencies-point-one-way.md) forbid.
   A rule carries no `issue` field and no transition marker. The policy loader
   rejects each of these shapes
   ([ADR 0057](0057-only-the-target-is-a-rule.md) points 1 and 2).
2. **Everything else that exists is exact debt.** It is one `legacy.jsonl`
   key, `scope|rule|source|target`, with one open carrier issue: no wildcards,
   no rebaseline mode, no reassigned issue. Debt only shrinks; a newly
   recorded key is accepted only for an import the base policy allowed and the
   base tree had (ADR 0057 point 2). A cutover never hands its successor the
   old reach as a rule; it converts that reach into keys or removes it.
3. **Retained `models/*` packages have the role `legacy-model`**, which no
   foreign owner may import, until they are dissolved into their owners
   (ADR 0057 point 4).
4. **Ratchets outside the policy only fall.** PR mode reads every budget,
   register and exception count of the ratchet tests at the base commit and
   rejects a raise. A raise passes only under an ADR the same change adds and
   the raised entry cites; renaming or copying an existing ADR authorizes
   nothing (ADR 0057 point 5). The recorded raises are
   [ADR 0063](0063-operator-status-lifecycle-raises-the-identity-compose-budget.md),
   [ADR 0064](0064-moved-code-stays-in-the-pass-through-scan.md) and
   [ADR 0072](0072-gocognit-upgrade-rebaselines-the-module-complexity-register.md).
5. **A measurement follows the code, not its location.** Pass-throughs are
   counted per owner across its application layer and its compositions
   ([ADR 0061](0061-pass-throughs-are-counted-per-owner.md)); code moved into
   another package of the same owner stays in the scan, and a later move into
   another such package adds it to the scanned list in the same change
   (ADR 0064).
6. **A relocated package keeps its debt.** A reviewed `relocations` entry
   renames the base keys onto the new path and grants nothing: owner and roles
   stay, an added import is still a new violation, a vanished one is still
   stale
   ([ADR 0020](0020-a-relocated-package-keeps-its-debt.md)).
7. **The policy epoch rises only for a reviewed architecture decision.** A
   reviewed epoch may add only what a named acceptor admits: a finite selector
   set with positive and negative fixtures
   ([ADR 0053](0053-owners-compose-themselves-and-the-roots-compose-the-owners.md)
   point 9). Such procedures include a data-less workflow owner whose packages
   are all new
   ([ADR 0013](0013-staff-offboarding-is-an-application-workflow.md)) and the
   adoption of an existing table without moving data, evidenced by a recorded
   `tables.unclassified` finding
   ([ADR 0015](0015-owners-adopt-existing-unowned-tables.md)) or by a retired
   generic store's `tables.unresolved` finding
   ([ADR 0045](0045-owners-adopt-tables-a-retired-generic-store-reached.md)).
   An acceptor whose base condition can no longer hold is deleted with its
   tests (ADR 0057 point 7).
8. **Exact package selectors only narrow.** `source_package` and
   `target_package` name one classified package and intersect the other
   selectors. Adding one is narrowing; removing or changing one is compared as
   a possible widening (ADR 0057, addendum of 2026-10-16).
9. **A retirement carries evidence with its provenance.** While the baseline
   held debt, every retired key needed a migration record with measured or
   justified runtime evidence
   ([ADR 0026](0026-migration-evidence-gate-preserves-provenance.md)). The
   baseline is empty and can only shrink, so the records and their gate are
   removed ([amendment of 2026-10-23](0026-migration-evidence-gate-preserves-provenance.md#amendment-of-2026-10-23));
   the completed records stay readable at the commit the amendment names.

## Consequences

- Ratchet 0 means what #2580 promised: no permitted violation is hidden in the
  policy. One key remains, `modules/dataimport/compose -> auth/authorize`
  under [#4004](https://github.com/moto-nrw/project-phoenix/issues/4004). The
  zero-ratchet cutover follows the runtime checkpoint
  [#3021](https://github.com/moto-nrw/project-phoenix/issues/3021).
- The cutover ADRs that let successors keep retired reach (ADR 0030, ADR 0035
  to ADR 0038, ADR 0052) are historical records. Their exceptions no longer
  exist, and what they let stand became exact debt and has since been removed.

## References

ADR 0020, ADR 0026 and ADR 0057 carry the decisions; ADR 0013, ADR 0015 and
ADR 0045 the reviewed procedures; ADR 0053, ADR 0061, ADR 0063 and ADR 0064 the
points named above. The full rule map is the
[traceability appendix](../../backend/architecture/README.md#appendix-principle-traceability).
