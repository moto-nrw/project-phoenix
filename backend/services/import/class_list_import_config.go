package importpkg

import (
	"context"
	"errors"
	"fmt"
	"strings"

	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	importModels "github.com/moto-nrw/project-phoenix/models/import"
	"github.com/moto-nrw/project-phoenix/services/import/ports"
)

var (
	// ErrClassListEntryDuplicate reports a class-list entry that already
	// carries the row's name and class.
	ErrClassListEntryDuplicate = errors.New("class list entry already exists in this class")
	// ErrClassListEntryStudentExists reports a regular student that already
	// carries the row's name and class; the child needs no list entry.
	ErrClassListEntryStudentExists = errors.New("a student with this name already exists in this class")
)

// ClassListImportDeps are the owner ports of the class-list entry import
// (#2382, #2708): School Membership writes the entry, People Directory
// answers whether a student already carries the name, and the Audit
// platform records the change so imported rows leave the same trail as
// manually created ones.
type ClassListImportDeps struct {
	Membership ports.ClassListMembership
	Persons    ports.PersonDirectory
	Students   ports.StudentDirectory
	Audit      ports.AuditCommand
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
// upfront instead. The map is keyed by the zero-based slice index (that is
// what the engine reads); the message shows the 1-based file row including
// the header line, matching the engine's own row numbering.
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
			result[i] = append(result[i], importModels.ValidationError{
				Field:    "first_name",
				Message:  fmt.Sprintf("Doppelter Eintrag in der Datei (bereits in Zeile %d)", firstRow+2),
				Code:     "duplicate_in_file",
				Severity: importModels.ErrorSeverityError,
			})
			continue
		}
		seen[key] = i
	}
	return result
}

// FindExisting reports a child already on the class list: an existing entry
// with the same name and class, or a regular student — a student IS the
// child's row on the list, so the dry-run preview must report that case as
// "already exists" too instead of counting it as creatable and failing only
// on the actual create. The import mode is create-only, so the returned ID
// is never used for an update; a hit becomes the engine's "already exists"
// row error.
func (c *ClassListImportConfig) FindExisting(ctx context.Context, row importModels.ClassListEntryImportRow) (*int64, error) {
	existing, err := c.deps.Membership.ListClassListEntries(ctx, ports.ClassListEntryFilter{
		FirstName: row.FirstName, LastName: row.LastName, SchoolClass: row.SchoolClass,
	})
	if err != nil {
		return nil, err
	}
	if len(existing) > 0 {
		id := existing[0].ID
		return &id, nil
	}
	students, err := findStudentsByNameAndClass(ctx, c.deps.Persons, c.deps.Students, row.FirstName, row.LastName, row.SchoolClass)
	if err != nil {
		return nil, err
	}
	if len(students) > 0 {
		id := students[0].ID
		return &id, nil
	}
	return nil, nil
}

// Create creates one entry through the owner: the duplicate-against-students
// guard and the audit row are applied exactly like for manual creation.
func (c *ClassListImportConfig) Create(ctx context.Context, row importModels.ClassListEntryImportRow) (int64, error) {
	existingID, err := c.FindExisting(ctx, row)
	if err != nil {
		return 0, fmt.Errorf("class list entry duplicate check: %w", err)
	}
	if existingID != nil {
		students, err := findStudentsByNameAndClass(ctx, c.deps.Persons, c.deps.Students, row.FirstName, row.LastName, row.SchoolClass)
		if err != nil {
			return 0, fmt.Errorf("class list entry student check: %w", err)
		}
		if len(students) > 0 {
			return 0, ErrClassListEntryStudentExists
		}
		return 0, ErrClassListEntryDuplicate
	}

	changedBy := ImporterIDFromContext(ctx)
	input := ports.CreateClassListEntry{ClassListEntryFields: ports.ClassListEntryFields{
		FirstName: row.FirstName, LastName: row.LastName, SchoolClass: row.SchoolClass,
	}}
	if changedBy > 0 {
		input.CreatedBy = &changedBy
	}
	entry, err := c.deps.Membership.CreateClassListEntry(ctx, input)
	if err != nil {
		if errors.Is(err, ports.ErrMembershipClassListEntryDuplicate) {
			// A concurrent create slipped past the advisory check above; the
			// DB index is the race-safe backstop — report it as the duplicate
			// it is, not as a server error.
			return 0, ErrClassListEntryDuplicate
		}
		return 0, fmt.Errorf("create class list entry: %w", err)
	}
	if c.deps.Audit != nil {
		display := strings.TrimSpace(entry.FirstName) + " " + strings.TrimSpace(entry.LastName) + " (" + strings.TrimSpace(entry.SchoolClass) + ")"
		if err := c.deps.Audit.Append(ctx, &auditModels.ClassListEntryChange{
			EntryID: entry.ID, Action: auditModels.ClassListEntryActionCreated, OldValue: "", NewValue: display, ChangedBy: changedBy,
		}); err != nil {
			return 0, fmt.Errorf("record class list entry change: %w", err)
		}
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
