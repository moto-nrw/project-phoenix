package importpkg

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	importModels "github.com/moto-nrw/project-phoenix/models/import"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	usersService "github.com/moto-nrw/project-phoenix/services/users"
)

// stubClassListEntryService only serves Create; every other method panics via
// the embedded nil interface.
type stubClassListEntryService struct {
	usersService.ClassListEntryService
	create func(ctx context.Context, input usersService.ClassListEntryInput, changedBy int64) (*userModels.ClassListEntry, error)
}

func (s stubClassListEntryService) Create(ctx context.Context, input usersService.ClassListEntryInput, changedBy int64) (*userModels.ClassListEntry, error) {
	return s.create(ctx, input, changedBy)
}

// stubClassListEntryRepo only serves FindByNameAndClass.
type stubClassListEntryRepo struct {
	userModels.ClassListEntryRepository
	findByNameAndClass func(ctx context.Context, firstName, lastName, schoolClass string) ([]*userModels.ClassListEntry, error)
}

func (r stubClassListEntryRepo) FindByNameAndClass(ctx context.Context, firstName, lastName, schoolClass string) ([]*userModels.ClassListEntry, error) {
	return r.findByNameAndClass(ctx, firstName, lastName, schoolClass)
}

func classListRow(firstName, lastName, schoolClass string) importModels.ClassListEntryImportRow {
	return importModels.ClassListEntryImportRow{
		FirstName:   firstName,
		LastName:    lastName,
		SchoolClass: schoolClass,
	}
}

func TestClassListImportConfig_Validate(t *testing.T) {
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
	config := NewClassListImportConfig(ClassListImportDeps{})

	rows := []importModels.ClassListEntryImportRow{
		classListRow("Zoe", "Aalders", "1a"),
		classListRow("Ben", "Zorn", "3c"),
		classListRow(" zoe ", "AALDERS", "1A"), // case/space variant of row 1
		classListRow("", "", ""),               // empty rows are ignored
	}

	result := config.ValidateBatch(context.Background(), rows)
	require.Len(t, result, 1)
	dup, ok := result[3]
	require.True(t, ok, "the duplicate is reported on its own (1-based) row")
	require.Len(t, dup, 1)
	assert.Equal(t, "duplicate_in_file", dup[0].Code)
	assert.Contains(t, dup[0].Message, "Zeile 1")
}

func TestClassListImportConfig_FindExisting(t *testing.T) {
	existing := &userModels.ClassListEntry{}
	existing.ID = 77

	config := NewClassListImportConfig(ClassListImportDeps{
		EntryRepo: stubClassListEntryRepo{findByNameAndClass: func(_ context.Context, firstName, lastName, schoolClass string) ([]*userModels.ClassListEntry, error) {
			if firstName == "Zoe" && lastName == "Aalders" && schoolClass == "1a" {
				return []*userModels.ClassListEntry{existing}, nil
			}
			return nil, nil
		}},
	})

	id, err := config.FindExisting(context.Background(), classListRow("Zoe", "Aalders", "1a"))
	require.NoError(t, err)
	require.NotNil(t, id)
	assert.Equal(t, existing.ID, *id)

	id, err = config.FindExisting(context.Background(), classListRow("Ben", "Zorn", "3c"))
	require.NoError(t, err)
	assert.Nil(t, id)

	failing := NewClassListImportConfig(ClassListImportDeps{
		EntryRepo: stubClassListEntryRepo{findByNameAndClass: func(context.Context, string, string, string) ([]*userModels.ClassListEntry, error) {
			return nil, errors.New("kaputt")
		}},
	})
	_, err = failing.FindExisting(context.Background(), classListRow("Zoe", "Aalders", "1a"))
	require.Error(t, err)
}

func TestClassListImportConfig_CreateMapsServiceErrors(t *testing.T) {
	created := &userModels.ClassListEntry{}
	created.ID = 9

	config := NewClassListImportConfig(ClassListImportDeps{
		EntryService: stubClassListEntryService{create: func(_ context.Context, input usersService.ClassListEntryInput, _ int64) (*userModels.ClassListEntry, error) {
			assert.Equal(t, "Zoe", input.FirstName)
			return created, nil
		}},
	})

	id, err := config.Create(context.Background(), classListRow("Zoe", "Aalders", "1a"))
	require.NoError(t, err)
	assert.Equal(t, created.ID, id)

	// User-facing duplicate guards pass through verbatim so the row error in
	// the import report carries the German message.
	duplicate := NewClassListImportConfig(ClassListImportDeps{
		EntryService: stubClassListEntryService{create: func(context.Context, usersService.ClassListEntryInput, int64) (*userModels.ClassListEntry, error) {
			return nil, usersService.ErrClassListEntryStudentExists
		}},
	})
	_, err = duplicate.Create(context.Background(), classListRow("Zoe", "Aalders", "1a"))
	assert.ErrorIs(t, err, usersService.ErrClassListEntryStudentExists)

	broken := NewClassListImportConfig(ClassListImportDeps{
		EntryService: stubClassListEntryService{create: func(context.Context, usersService.ClassListEntryInput, int64) (*userModels.ClassListEntry, error) {
			return nil, errors.New("kaputt")
		}},
	})
	_, err = broken.Create(context.Background(), classListRow("Zoe", "Aalders", "1a"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "create class list entry")
}

func TestClassListImportConfig_UpdateIsRefused(t *testing.T) {
	config := NewClassListImportConfig(ClassListImportDeps{})
	err := config.Update(context.Background(), 1, classListRow("Zoe", "Aalders", "1a"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nicht aktualisiert")
}
