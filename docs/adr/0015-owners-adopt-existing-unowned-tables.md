---
status: accepted
---

# Owners adopt existing unowned tables

[#3235](https://github.com/moto-nrw/project-phoenix/issues/3235) asks how a
target owner takes over a live table that predates the policy and that the
baseline tracks as `tables.unclassified` debt. The migration programme in
[#2580](https://github.com/moto-nrw/project-phoenix/issues/2580) answers: by
declaring the owner in a reviewed policy epoch, without moving the data.

Eleven tables are in this position: the four File Storage tables under
[#2707](https://github.com/moto-nrw/project-phoenix/issues/2707), six People
Directory and Care Plan tables under
[#2727](https://github.com/moto-nrw/project-phoenix/issues/2727), and
`auth.accounts_parents` under
[#2720](https://github.com/moto-nrw/project-phoenix/issues/2720). Until now the
evaluator accepted a new `data_objects` entry only for a table that a new
migration in the same candidate creates. For a live table that means a second
table and a data migration whose only purpose is to satisfy the checker. The
File Storage attempt recorded on #2707 shows the contradiction: the ticket's
exit criterion requires unchanged ownership and its rollback plan relies on
the unchanged table shape, while the evaluator requires a new table.
[ADR 0010](0010-file-storage-is-the-eighteenth-domain.md) already assigns
those tables to File Storage and names the missing mappings migration work.

The target architecture wants owner boundaries around the data schools
already have, not new data. A table that has one runtime write owner in the
decision and one accessing package in the code is owned in every sense but
the policy entry. Requiring a migration for that entry would move production
files, students and guardians between tables for no domain reason.

## Policy registration

A reviewed policy epoch may add a `data_objects` entry for a table when all of
the following hold against the base SHA:

1. The table has no entry in the base `data_objects`.
2. The base `legacy.jsonl` records at least one production
   `tables.unclassified` finding for the table.
3. Every package those findings name is, in the candidate policy, classified
   under the adopting owner, no longer classified at all, or no longer reads
   or writes the table in the candidate.
4. The candidate policy epoch is greater than the base epoch.

The candidate removes the consumed `tables.unclassified` entries from the
baseline in the same change, as it would any resolved finding. Everything
else stays a loosening: transferring an owned table to another owner,
adopting a table the baseline does not track, and adopting while a package of
another owner still accesses it. After the adoption the ordinary
`tables.foreign-write` and `tables.foreign-read` rules guard every other
accessor, and because semantic findings are excluded from the import-debt
conversion, a foreign accessor surfaces as a new violation and blocks the
pull request. The owning package itself is not moved by this path; a
relocation follows the ordinary candidate-created-package rules.

The third alternative of rule 3 was added by
[#3221](https://github.com/moto-nrw/project-phoenix/issues/3221). The staff
messaging tables are recorded against `database/repositories/users`, a People
Directory package that keeps serving People Directory tables after the
messaging SQL moves to Communication. Without it the move needs the adoption
(the moved access would otherwise be a new unclassified finding) and the
adoption needs the move to delete a package that still has other work. The
evaluator decides "no longer reads or writes" from the candidate's own
ownership findings: under the candidate policy any remaining access by
another owner's package is a `tables.foreign-read` or `tables.foreign-write`
finding, so the same evidence that would block the pull request also blocks
the adoption. A package that still has a `tables.unresolved` finding proves
nothing — its table expression names no table — and blocks the adoption too.

This is the third reviewed-epoch path after
[ADR 0013](0013-staff-offboarding-is-an-application-workflow.md) and
[ADR 0014](0014-test-roles-may-import-test-infrastructure.md). Like them it
is a narrow evaluator permission, not a rebuild of legacy findings.
