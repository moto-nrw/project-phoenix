package importpkg

import (
	"context"
	"errors"
	"fmt"
	"strings"

	importModels "github.com/moto-nrw/project-phoenix/models/import"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	usersService "github.com/moto-nrw/project-phoenix/services/users"
)

// ClassListImportDeps are the dependencies of the class-list entry import
// (#2382). The config creates through the service so the duplicate guards and
// the audit trail apply to imported rows exactly like to manually created
// ones.
type ClassListImportDeps struct {
	EntryService usersService.ClassListEntryService
	EntryRepo    userModels.ClassListEntryRepository
}

// ClassListImportConfig implements ImportConfig for class-list entries.
type ClassListImportConfig struct {
	deps ClassListImportDeps
}

// NewClassListImportConfig creates the import config.
func NewClassListImportConfig(deps ClassListImportDeps) *ClassListImportConfig {
	return &ClassListImportConfig{deps: deps}
}

// PreloadReferenceData is a no-op: entries reference nothing.
func (c *ClassListImportConfig) PreloadReferenceData(_ context.Context) error {
	return nil
}

// Validate checks one row: all three fields are required.
func (c *ClassListImportConfig) Validate(_ context.Context, row *importModels.ClassListEntryImportRow) []importModels.ValidationError {
	var errs []importModels.ValidationError
	row.FirstName = strings.TrimSpace(row.FirstName)
	row.LastName = strings.TrimSpace(row.LastName)
	row.SchoolClass = strings.TrimSpace(row.SchoolClass)

	if row.FirstName == "" {
		errs = append(errs, requiredFieldError("first_name", "Vorname ist erforderlich"))
	}
	if row.LastName == "" {
		errs = append(errs, requiredFieldError("last_name", "Nachname ist erforderlich"))
	}
	if row.SchoolClass == "" {
		errs = append(errs, requiredFieldError("school_class", "Klasse ist erforderlich"))
	}
	return errs
}

// ValidateBatch flags in-file duplicates: two rows with the same name and
// class would hit the unique index on insert — report it as a row error
// upfront instead.
func (c *ClassListImportConfig) ValidateBatch(_ context.Context, rows []importModels.ClassListEntryImportRow) map[int][]importModels.ValidationError {
	result := make(map[int][]importModels.ValidationError)
	seen := make(map[string]int, len(rows))
	for i, row := range rows {
		key := strings.ToLower(strings.TrimSpace(row.FirstName)) + "\x00" +
			strings.ToLower(strings.TrimSpace(row.LastName)) + "\x00" +
			strings.ToLower(strings.TrimSpace(row.SchoolClass))
		if key == "\x00\x00" {
			continue
		}
		if firstRow, dup := seen[key]; dup {
			result[i+1] = append(result[i+1], importModels.ValidationError{
				Field:    "first_name",
				Message:  fmt.Sprintf("Doppelter Eintrag in der Datei (bereits in Zeile %d)", firstRow),
				Code:     "duplicate_in_file",
				Severity: importModels.ErrorSeverityError,
			})
			continue
		}
		seen[key] = i + 1
	}
	return result
}

// FindExisting reports an already-existing entry with the same name and
// class. The import mode is create-only, so a hit becomes the engine's
// "already exists" row error.
func (c *ClassListImportConfig) FindExisting(ctx context.Context, row importModels.ClassListEntryImportRow) (*int64, error) {
	existing, err := c.deps.EntryRepo.FindByNameAndClass(ctx, row.FirstName, row.LastName, row.SchoolClass)
	if err != nil {
		return nil, err
	}
	if len(existing) > 0 {
		id := existing[0].ID
		return &id, nil
	}
	return nil, nil
}

// Create creates one entry through the service: duplicate-against-students
// guard and audit row included.
func (c *ClassListImportConfig) Create(ctx context.Context, row importModels.ClassListEntryImportRow) (int64, error) {
	entry, err := c.deps.EntryService.Create(ctx, usersService.ClassListEntryInput{
		FirstName:   row.FirstName,
		LastName:    row.LastName,
		SchoolClass: row.SchoolClass,
	}, ImporterIDFromContext(ctx))
	if err != nil {
		if errors.Is(err, usersService.ErrClassListEntryStudentExists) || errors.Is(err, usersService.ErrClassListEntryDuplicate) {
			return 0, err
		}
		return 0, fmt.Errorf("create class list entry: %w", err)
	}
	return entry.ID, nil
}

// Update is not supported: an entry has nothing to update from a re-import —
// the import mode is create-only.
func (c *ClassListImportConfig) Update(_ context.Context, _ int64, _ importModels.ClassListEntryImportRow) error {
	return errors.New("Klassenlisteneinträge werden beim Import nicht aktualisiert") //nolint:staticcheck // ST1005: user-facing German message
}

// EntityName returns the entity type name for logging and error messages.
func (c *ClassListImportConfig) EntityName() string {
	return "Klassenlisteneintrag"
}
