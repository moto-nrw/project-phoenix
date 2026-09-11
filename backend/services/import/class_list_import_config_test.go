package importpkg

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	importModels "github.com/moto-nrw/project-phoenix/models/import"
	"github.com/moto-nrw/project-phoenix/services/import/ports"
)

func classListRow(firstName, lastName, schoolClass string) importModels.ClassListEntryImportRow {
	return importModels.ClassListEntryImportRow{
		FirstName:   firstName,
		LastName:    lastName,
		SchoolClass: schoolClass,
	}
}

func TestClassListImportConfig_Validate(t *testing.T) {
	t.Parallel()

	config := NewClassListImportConfig(ClassListImportDeps{})

	require.NoError(t, config.PreloadReferenceData(context.Background()))
	assert.Equal(t, "Klassenlisteneintrag", config.EntityName())

	row := classListRow("  Zoe  ", " Aalders ", " 1a ")
	errs := config.Validate(context.Background(), &row)
	assert.Empty(t, errs)
	assert.Equal(t, "Zoe", row.FirstName, "Validate must trim the fields")
	assert.Equal(t, "Aalders", row.LastName)
	assert.Equal(t, "1a", row.SchoolClass)

	empty := classListRow("  ", "", " ")
	errs = config.Validate(context.Background(), &empty)
	require.Len(t, errs, 3)
	fields := []string{errs[0].Field, errs[1].Field, errs[2].Field}
	assert.ElementsMatch(t, []string{"first_name", "last_name", "school_class"}, fields)
}

func TestClassListImportConfig_ValidateBatchFlagsInFileDuplicates(t *testing.T) {
	t.Parallel()

	config := NewClassListImportConfig(ClassListImportDeps{})

	rows := []importModels.ClassListEntryImportRow{
		classListRow("Zoe", "Aalders", "1a"),
		classListRow("Ben", "Zorn", "3c"),
		classListRow(" zoe ", "AALDERS", "1A"), // case/space variant of row 1
		classListRow("", "", ""),               // empty rows are ignored
	}

	result := config.ValidateBatch(context.Background(), rows)
	require.Len(t, result, 1)
	dup, ok := result[2]
	require.True(t, ok, "the duplicate is keyed by its zero-based slice index — that is what the engine reads")
	require.Len(t, dup, 1)
	assert.Equal(t, "duplicate_in_file", dup[0].Code)
	assert.Contains(t, dup[0].Message, "Zeile 2", "the message shows the 1-based file row including the header")
}

func TestClassListImportConfig_FindExisting(t *testing.T) {
	t.Parallel()

	entries := &fakeClassList{entries: []ports.ClassListEntry{{ID: 77, FirstName: "Zoe", LastName: "Aalders", SchoolClass: "1a"}}}
	config := NewClassListImportConfig(ClassListImportDeps{
		Membership: entries, Persons: newFakePersons(), Students: &fakeStudents{},
	})

	id, err := config.FindExisting(context.Background(), classListRow("Zoe", "Aalders", "1a"))
	require.NoError(t, err)
	require.NotNil(t, id)
	assert.Equal(t, int64(77), *id)

	id, err = config.FindExisting(context.Background(), classListRow("Ben", "Zorn", "3c"))
	require.NoError(t, err)
	assert.Nil(t, id)

	failing := NewClassListImportConfig(ClassListImportDeps{Membership: &fakeClassList{listErr: errFakeOwner}})
	_, err = failing.FindExisting(context.Background(), classListRow("Zoe", "Aalders", "1a"))
	require.Error(t, err)
}

// A regular student with the entry's name and class IS the child's row on the
// class list: the preview must report the row as already existing, not as
// creatable (the actual create would fail on ErrClassListEntryStudentExists).
// The People Directory lookup matches like the legacy repository did:
// case-insensitive, trimmed, non-alumni only.
func TestClassListImportConfig_FindExistingReportsMatchingStudent(t *testing.T) {
	t.Parallel()

	persons := newFakePersons(
		ports.Person{ID: 1, FirstName: "ZOE", LastName: "aalders"},
		ports.Person{ID: 2, FirstName: "Zoe", LastName: "Aalders"},
		ports.Person{ID: 3, FirstName: "Zoey", LastName: "Aalders"},
	)
	students := &fakeStudents{students: []ports.Student{
		{ID: 41, PersonID: 1, SchoolClass: "1A ", Status: "active"},
		{ID: 42, PersonID: 2, SchoolClass: "1a", Status: ports.StudentStatusAlumnus},
		{ID: 43, PersonID: 3, SchoolClass: "1a", Status: "active"},
	}}
	config := NewClassListImportConfig(ClassListImportDeps{Membership: &fakeClassList{}, Persons: persons, Students: students})

	id, err := config.FindExisting(context.Background(), classListRow("Zoe", "Aalders", "1a"))
	require.NoError(t, err)
	require.NotNil(t, id)
	assert.Equal(t, int64(41), *id, "the case-folded live student matches; the alumnus and the prefix namesake do not")

	id, err = config.FindExisting(context.Background(), classListRow("Ben", "Zorn", "3c"))
	require.NoError(t, err)
	assert.Nil(t, id)

	failing := NewClassListImportConfig(ClassListImportDeps{Membership: &fakeClassList{}, Persons: persons, Students: &fakeStudents{err: errFakeOwner}})
	_, err = failing.FindExisting(context.Background(), classListRow("Zoe", "Aalders", "1a"))
	require.Error(t, err)
}

func TestClassListImportConfig_CreateThroughMembershipWithAuditTrail(t *testing.T) {
	t.Parallel()

	entries := &fakeClassList{}
	audit := &fakeAudit{}
	config := NewClassListImportConfig(ClassListImportDeps{Membership: entries, Persons: newFakePersons(), Students: &fakeStudents{}, Audit: audit})
	importerID := fakeImporterID()
	ctx := ContextWithImporterID(context.Background(), importerID)

	id, err := config.Create(ctx, classListRow("Zoe", "Aalders", "1a"))
	require.NoError(t, err)
	require.Len(t, entries.entries, 1)
	assert.Equal(t, entries.entries[0].ID, id, "the owner assigns the entry id")
	require.Len(t, entries.created, 1)
	require.NotNil(t, entries.created[0].CreatedBy)
	assert.Equal(t, importerID, *entries.created[0].CreatedBy, "the importer is the creating account")
	require.Len(t, audit.events, 1)
	change, ok := audit.events[0].(*auditModels.ClassListEntryChange)
	require.True(t, ok)
	assert.Equal(t, id, change.EntryID)
	assert.Equal(t, auditModels.ClassListEntryActionCreated, change.Action)
	assert.Equal(t, "Zoe Aalders (1a)", change.NewValue)
	assert.Equal(t, importerID, change.ChangedBy)

	// A second create of the same row is refused by the owner's unique index
	// and reported as the duplicate it is; the audit trail does not grow.
	_, err = config.Create(ctx, classListRow("zoe", "aalders", "1A"))
	assert.ErrorIs(t, err, ErrClassListEntryDuplicate)
	assert.Len(t, entries.entries, 1)
	assert.Len(t, audit.events, 1)
}

func TestClassListImportConfig_CreateMapsOwnerErrors(t *testing.T) {
	t.Parallel()

	// A regular student with the name already exists: the row needs no entry.
	persons := newFakePersons(ports.Person{ID: 1, FirstName: "Zoe", LastName: "Aalders"})
	students := &fakeStudents{students: []ports.Student{{ID: 41, PersonID: 1, SchoolClass: "1a", Status: "active"}}}
	config := NewClassListImportConfig(ClassListImportDeps{Membership: &fakeClassList{}, Persons: persons, Students: students})
	_, err := config.Create(context.Background(), classListRow("Zoe", "Aalders", "1a"))
	assert.ErrorIs(t, err, ErrClassListEntryStudentExists)

	// The owner's unique index is the race-safe backstop; its sentinel is
	// reported as the duplicate it is, not as a server error.
	raced := NewClassListImportConfig(ClassListImportDeps{
		Membership: &fakeClassList{create: func(ports.CreateClassListEntry) (ports.ClassListEntry, error) {
			return ports.ClassListEntry{}, ports.ErrMembershipClassListEntryDuplicate
		}},
		Persons: newFakePersons(), Students: &fakeStudents{},
	})
	_, err = raced.Create(context.Background(), classListRow("Zoe", "Aalders", "1a"))
	assert.ErrorIs(t, err, ErrClassListEntryDuplicate)

	broken := NewClassListImportConfig(ClassListImportDeps{
		Membership: &fakeClassList{create: func(ports.CreateClassListEntry) (ports.ClassListEntry, error) {
			return ports.ClassListEntry{}, errFakeOwner
		}},
		Persons: newFakePersons(), Students: &fakeStudents{},
	})
	_, err = broken.Create(context.Background(), classListRow("Zoe", "Aalders", "1a"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "create class list entry")

	// A failed audit append fails the row: the trail is part of the write.
	unaudited := NewClassListImportConfig(ClassListImportDeps{Membership: &fakeClassList{}, Persons: newFakePersons(), Students: &fakeStudents{}, Audit: &fakeAudit{err: errFakeOwner}})
	_, err = unaudited.Create(context.Background(), classListRow("Zoe", "Aalders", "1a"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "record class list entry change")
}

func TestClassListImportConfig_UpdateIsRefused(t *testing.T) {
	t.Parallel()

	config := NewClassListImportConfig(ClassListImportDeps{})
	err := config.Update(context.Background(), 1, classListRow("Zoe", "Aalders", "1a"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nicht aktualisiert")
}
