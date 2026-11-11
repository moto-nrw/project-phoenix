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
// neither never touch each other's match. Guardians with an e-mail are still
// written row by row in processing order, so a later row reuses an earlier
// row's guardian as on the row path; their e-mail joins the keys once a
// batch command writes them. Guardians without one are batched (step 5c).

// ChunkWriterReady reports whether the batch slices of People Directory
// (persons, new guardians), Student Presence and the Audit consent recorder
// are bound.
func (c *StudentImportConfig) ChunkWriterReady() bool {
	return c.PersonBatches != nil && c.PrivacyConsentBatches != nil && c.ConsentHistoryBatches != nil && c.GuardianBatches != nil
}

// ChunkMinRows: a batch segment costs six to seven statements (bound set and
// reset, the protected name read with its savepoint, one person statement)
// where the row path spends two per row (name search, person insert), and the
// chunk savepoint adds two; a row-path chunk of more than four rows already
// reads its names at once (BatchWriteConfig). From six rows on the batch path
// issues fewer statements.
func (c *StudentImportConfig) ChunkMinRows() int { return 6 }

// ChunkKeys are the inputs a create row is matched by: the name (which a
// card without a hit falls back to) and the card. A guardian e-mail is no key
// (#5552 step 5c): e-mail guardians are still written row by row in
// processing order, so a later row reuses an earlier row's guardian as on the
// row path, and a batched guardian has no e-mail and is never reused. Keying
// on it would only split sibling files below ChunkMinRows.
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
// under the bound, then, row by row in processing order and without the
// bound, everything the row path creates after the person except the
// retention consent and the consent history. Those it collects with each
// row's own instant and writes after the rows, each kind in one bundled
// statement under a second bound (#5552, step 5b).
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
	leafs := &studentLeafWrites{}
	for i, row := range rows {
		if _, err := c.createStudentEntities(ctx, persons[i].ID, row, leafs); err != nil {
			return err
		}
	}
	return c.writeLeafs(ctx, leafs, bounded)
}

// writeLeafs writes the collected new guardians (step 5c), consents and
// consent history, each kind in one bundled statement per table, under one
// bound. All touch only rows of this transaction and the school row the
// persons statement already holds, so no wait moves (PR #6408 and #6514,
// tables C and D). An empty kind issues no statement, and with none the
// bound is not set.
func (c *StudentImportConfig) writeLeafs(ctx context.Context, leafs *studentLeafWrites, bounded ports.BatchStatement) error {
	if len(leafs.guardians) == 0 && len(leafs.consents) == 0 && !hasConsentEvents(leafs.transitions) {
		return nil
	}
	return bounded(ctx, func(ctx context.Context) error {
		if len(leafs.guardians) > 0 {
			if err := c.GuardianBatches.CreateNewStudentGuardians(ctx, leafs.guardians); err != nil {
				return fmt.Errorf("create guardians: %w", err)
			}
		}
		if len(leafs.consents) > 0 {
			if err := ports.ObserveCommand(ctx, "student-presence", "record_privacy_consents", func() error {
				return c.PrivacyConsentBatches.RecordPrivacyConsents(ctx, leafs.consents)
			}); err != nil {
				return fmt.Errorf("create privacy consents: %w", err)
			}
		}
		if err := ports.ObserveCommand(ctx, "audit-platform", "record_consent_transitions_all", func() error {
			return c.ConsentHistoryBatches.RecordTransitionsAll(ctx, leafs.transitions)
		}); err != nil {
			return fmt.Errorf("create student consent history: %w", err)
		}
		return nil
	})
}

// hasConsentEvents reports whether a transition of a new child records an
// event: one per consent date the row carries (a new child has no "before").
func hasConsentEvents(transitions []ports.ConsentTransition) bool {
	for _, transition := range transitions {
		after := transition.After
		if after != nil && (after.AGBAcceptedAt != nil || after.DataProcessingAcceptedAt != nil ||
			after.EmailContactAcceptedAt != nil || after.PhotoConsentGivenAt != nil) {
			return true
		}
	}
	return false
}
