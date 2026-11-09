package importpkg

import (
	"context"
	"strings"

	auditModels "github.com/moto-nrw/project-phoenix/modules/auditlog/classlist"
	importModels "github.com/moto-nrw/project-phoenix/modules/dataimport"
	"github.com/moto-nrw/project-phoenix/modules/dataimport/ports"
)

// The class-list import is the first user of the chunk executor (#5827, ADR
// 0067 addendum of 2026-11-08, step 4). Rows of different names or classes
// never touch each other's match: an entry or a student is found by its own
// name and class only, and a write creates only its own entry.

// ChunkWriterReady reports whether School Membership's batch slice is bound.
func (c *ClassListImportConfig) ChunkWriterReady() bool { return c.deps.Batches != nil }

// ChunkKey is the in-file duplicate key of ValidateBatch.
func (c *ClassListImportConfig) ChunkKey(row importModels.ClassListEntryImportRow) string {
	return strings.ToLower(strings.TrimSpace(row.FirstName)) + "\x00" +
		strings.ToLower(strings.TrimSpace(row.LastName)) + "\x00" +
		strings.ToLower(strings.TrimSpace(row.SchoolClass))
}

// ResolveChunk reads, inside its own savepoint, the entries carrying each
// row's name and class and the students carrying its name, and answers
// FindExisting from that read like the row path answers it.
func (c *ClassListImportConfig) ResolveChunk(ctx context.Context, rows []importModels.ClassListEntryImportRow) (ports.ImportConfig[importModels.ClassListEntryImportRow], error) {
	keys := make([]ports.ClassListEntryFields, len(rows))
	index := make(map[string]int, len(rows))
	for i, row := range rows {
		keys[i] = ports.ClassListEntryFields{FirstName: row.FirstName, LastName: row.LastName, SchoolClass: row.SchoolClass}
		index[c.ChunkKey(row)] = i
	}
	resolved := &resolvedClassList{ClassListImportConfig: c, index: index, persons: c.deps.Persons, students: c.deps.Students}
	err := c.deps.Transactions.Savepoint(ctx, func(ctx context.Context) error {
		var err error
		if resolved.entries, err = c.deps.Batches.MatchClassListEntries(ctx, keys); err != nil {
			return err
		}
		resolved.persons, resolved.students, err = c.studentNameView(ctx, rows)
		return err
	})
	if err != nil {
		return nil, err
	}
	return resolved, nil
}

// RefusesChunk repeats the row path's guard against a student created since
// the match: a row whose name and class a student now carries refuses the
// batch, and the replay reports it as the row path does.
func (c *ClassListImportConfig) RefusesChunk(ctx context.Context, rows []importModels.ClassListEntryImportRow) (bool, error) {
	persons, students, err := c.studentNameView(ctx, rows)
	if err != nil {
		return false, err
	}
	for _, row := range rows {
		found, err := findStudentsByNameAndClass(ctx, persons, students, row.FirstName, row.LastName, row.SchoolClass)
		if err != nil {
			return false, err
		}
		if len(found) > 0 {
			return true, nil
		}
	}
	return false, nil
}

// CreateChunk creates the entries of one segment and their audit rows in two
// statements.
func (c *ClassListImportConfig) CreateChunk(ctx context.Context, rows []importModels.ClassListEntryImportRow) error {
	changedBy := ImporterIDFromContext(ctx)
	inputs := make([]ports.CreateClassListEntry, len(rows))
	for i, row := range rows {
		inputs[i] = ports.CreateClassListEntry{ClassListEntryFields: ports.ClassListEntryFields{
			FirstName: row.FirstName, LastName: row.LastName, SchoolClass: row.SchoolClass,
		}}
		if changedBy > 0 {
			inputs[i].CreatedBy = &changedBy
		}
	}
	entries, err := c.deps.Batches.CreateClassListEntries(ctx, inputs)
	if err != nil {
		return err
	}
	if c.deps.Audit == nil {
		return nil
	}
	events := make([]any, len(entries))
	for i, entry := range entries {
		display := strings.TrimSpace(entry.FirstName) + " " + strings.TrimSpace(entry.LastName) + " (" + strings.TrimSpace(entry.SchoolClass) + ")"
		events[i] = &auditModels.ClassListEntryChange{
			EntryID: entry.ID, Action: auditModels.ClassListEntryActionCreated, OldValue: "", NewValue: display, ChangedBy: changedBy,
		}
	}
	if batch, ok := c.deps.Audit.(ports.AuditBatchCommand); ok {
		return batch.AppendAll(ctx, events)
	}
	for _, event := range events {
		if err := c.deps.Audit.Append(ctx, event); err != nil {
			return err
		}
	}
	return nil
}

// studentNameView reads the persons carrying the rows' names and their
// students in one go; for names it cannot match like the owner predicate it
// keeps the owner directories, which answer per row.
func (c *ClassListImportConfig) studentNameView(ctx context.Context, rows []importModels.ClassListEntryImportRow) (ports.PersonDirectory, ports.StudentDirectory, error) {
	pairs := make([]ports.PersonName, len(rows))
	for i, row := range rows {
		pairs[i] = ports.PersonName{FirstName: row.FirstName, LastName: row.LastName}
	}
	filter, names := previewNameFilter(pairs)
	if len(filter.Names) == 0 {
		return c.deps.Persons, c.deps.Students, nil
	}
	found, students, err := readNameMatches(ctx, c.deps.Persons, c.deps.Students, filter)
	if err != nil {
		return nil, nil, err
	}
	persons, studentView, ok := nameMatchDirectories(found, students, filter, names, c.deps.Persons, c.deps.Students)
	if !ok {
		return c.deps.Persons, c.deps.Students, nil
	}
	return persons, studentView, nil
}

// resolvedClassList answers FindExisting for the rows of one segment from
// ResolveChunk's read; any other row asks the owners as the row path does.
type resolvedClassList struct {
	*ClassListImportConfig
	index    map[string]int
	entries  [][]ports.ClassListEntry
	persons  ports.PersonDirectory
	students ports.StudentDirectory
}

func (r *resolvedClassList) FindExisting(ctx context.Context, row importModels.ClassListEntryImportRow) (*int64, error) {
	i, ok := r.index[r.ChunkKey(row)]
	if !ok || i >= len(r.entries) {
		return r.ClassListImportConfig.FindExisting(ctx, row)
	}
	if len(r.entries[i]) > 0 {
		id := r.entries[i][0].ID
		return &id, nil
	}
	students, err := findStudentsByNameAndClass(ctx, r.persons, r.students, row.FirstName, row.LastName, row.SchoolClass)
	if err != nil {
		return nil, err
	}
	if len(students) > 0 {
		id := students[0].ID
		return &id, nil
	}
	return nil, nil
}
