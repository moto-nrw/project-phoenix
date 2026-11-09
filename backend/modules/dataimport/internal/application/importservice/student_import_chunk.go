package importpkg

import (
	"context"
	"fmt"

	importModels "github.com/moto-nrw/project-phoenix/modules/dataimport"
	"github.com/moto-nrw/project-phoenix/modules/dataimport/ports"
)

// The student create is the second user of the chunk executor (#5552, ADR
// 0067 addendum of 2026-11-08, step 5). A created child is found again by
// its card or, without a hit for the card, by its name. Rows that share
// neither never touch each other's match. Guardians are still written row by
// row in processing order, so a later row reuses an earlier row's guardian as
// on the row path; their e-mail joins the keys once a batch command writes
// them.

// ChunkWriterReady reports whether People Directory's person batch slice is
// bound.
func (c *StudentImportConfig) ChunkWriterReady() bool { return c.PersonBatches != nil }

// ChunkMinRows: a batch segment costs six to seven statements (bound set and
// reset, the protected name read with its savepoint, one person statement)
// where the row path spends two per row (name search, person insert), and the
// chunk savepoint adds two; a row-path chunk of more than four rows already
// reads its names at once (BatchWriteConfig). From six rows on the batch path
// issues fewer statements.
func (c *StudentImportConfig) ChunkMinRows() int { return 6 }

// ChunkKeys are the inputs a create row is matched by: the name (which a
// card without a hit falls back to) and the card.
func (c *StudentImportConfig) ChunkKeys(row importModels.StudentImportRow) []string {
	keys := []string{"name\x00" + matchKey(row.FirstName) + "\x00" + matchKey(row.LastName)}
	if tag := matchKey(row.TagID); tag != "" {
		keys = append(keys, "tag\x00"+tag)
	}
	return keys
}

// ChunkRowPath keeps a row with a card on the row path. A new card is a
// unique entry the row path claims in processing order, with the exclusive
// child quota between two children; claiming the cards of a segment in one
// statement before the first child would pair a deadlock with a parallel
// upload of the same school that the row path never forms (ADR 0067
// addendum, note on unique entries of new rows). The person batch thus
// writes persons without card and account only.
func (c *StudentImportConfig) ChunkRowPath(row importModels.StudentImportRow) bool {
	return importModels.OptionalCell(row.TagID) != nil
}

// ResolveChunk reads, inside its own savepoint, the persons carrying the
// segment's names and their students, and returns a config whose
// FindExisting answers the name match from that read; a card is still looked
// up per row, as the row path does.
func (c *StudentImportConfig) ResolveChunk(ctx context.Context, rows []importModels.StudentImportRow) (ports.ImportConfig[importModels.StudentImportRow], error) {
	pairs := make([]ports.PersonName, len(rows))
	for i, row := range rows {
		pairs[i] = ports.PersonName{FirstName: row.FirstName, LastName: row.LastName}
	}
	resolved := *c
	err := c.Transactions.Savepoint(ctx, func(ctx context.Context) error {
		var err error
		resolved.Persons, resolved.Students, err = nameMatchView(ctx, c.Persons, c.Students, pairs)
		return err
	})
	if err != nil {
		return nil, err
	}
	return &resolved, nil
}

// RefusesChunk is always false: the student create has no check between the
// match and the write that the row path could fail on.
func (c *StudentImportConfig) RefusesChunk(context.Context, []importModels.StudentImportRow) (bool, error) {
	return false, nil
}

// CreateChunk creates the persons of one segment in one bundled statement
// under the bound and then, row by row in processing order and without the
// bound, everything the row path creates after the person.
func (c *StudentImportConfig) CreateChunk(ctx context.Context, rows []importModels.StudentImportRow, bounded ports.BatchStatement) error {
	inputs := make([]ports.CreatePerson, len(rows))
	for i, row := range rows {
		inputs[i] = personInputFromRow(row)
	}
	var persons []ports.Person
	if err := bounded(ctx, func(ctx context.Context) error {
		var err error
		persons, err = c.PersonBatches.CreatePersons(ctx, inputs)
		return err
	}); err != nil {
		return fmt.Errorf("create persons: %w", err)
	}
	for i, row := range rows {
		if _, err := c.createStudentEntities(ctx, persons[i].ID, row); err != nil {
			return err
		}
	}
	return nil
}
