---
status: accepted
---

# Owners adopt tables a retired generic store reached

Accepted for [#2706](https://github.com/moto-nrw/project-phoenix/issues/2706)
in policy epoch 32, as an extension of
[ADR 0015](0015-owners-adopt-existing-unowned-tables.md). The pull request
review is where it can still be rejected; the path is usable only in a
reviewed epoch.

## Context

ADR 0015 lets a target owner adopt a live table that has no policy owner,
provided the base baseline records a production `tables.unclassified`
finding for it. That finding is the evidence that the table is in use and
which packages touch it.

`documents.file_cleanup` has no such finding. [ADR 0010](0010-file-storage-is-the-eighteenth-domain.md)
assigns it to File Storage, but the only code that reached it was the generic
document repository `database/repositories/documents`, which takes its table
names as runtime parameters. The analysis cannot resolve those expressions,
so the base baseline records two `tables.unresolved` findings against the
repository and nothing against the table. When File Storage replaces the
generic repository with its own adapter that names the table statically, the
table needs an owner, and ADR 0015 cannot give it one. The remaining routes
were a second table filled by a data migration whose only purpose is the
checker, which ADR 0015 already rejected, or leaving the generic repository
and its five baseline entries in place for good.

## Decision

A reviewed policy epoch may add a `data_objects` entry for a table when all
of the following hold:

1. The table has no entry in the base `data_objects`.
2. The base baseline records no production `tables.unclassified` finding for
   the table. Tables with such a finding use the ADR 0015 path.
3. The base baseline records at least one production `tables.unresolved`
   finding for a package that the candidate policy no longer classifies, that
   is, a generic store the candidate retires.
4. Analysed with the table assigned to an owner that does not exist, the
   candidate names the table statically from at least one package, and every
   such package belongs to the adopting owner.
5. The candidate policy epoch is greater than the base epoch.

Condition 4 ties the adoption to the access that replaced the retired
expression. The evaluator runs the semantic analysis a second time for it,
and only when a candidate data object passes conditions 1 to 3. Because the
probe owner matches no package, every reader and writer of the table shows up
as a foreign read or write, so a package of another owner that names the table
blocks the adoption the same way it would surface as a new violation.

The analysis cannot prove that the retired store reached this particular
table, because its expression named none. Review supplies that link: the
pull request names the table, the retired store and the configuration that
passed the table name in. Everything ADR 0015 keeps as a loosening stays one:
transferring an owned table, adopting a table nobody names statically, and
adopting while another owner's package names it.

## Consequences

#2706 deletes `database/repositories/documents`, `models/documents` and the
binder in `modules/documentrendering/compose`. File Storage serves the cleanup
intents through `modules/filestorage/internal/adapters/postgres` with a static
table name, and `documents.file_cleanup` joins the other File Storage tables
under `file-storage`. The two `tables.unresolved` entries and the three import
entries of the generic packages leave the baseline; no data moves.

The path is narrow by construction: it needs a generic store with recorded
unresolved debt, and the store must go in the same change. Further uses will
be rare. The fixture tests in
`backend/internal/architecture/table_adoption_test.go` pin the accepted case
and each guard.
