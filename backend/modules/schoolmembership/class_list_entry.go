package schoolmembership

import (
	"context"
	"errors"
	"strings"
	"time"
)

// Class-list entries (#2382) are children of the class cohort without an
// OGS record: only a name and the free-text school class. They exist so class
// lists and the Lehrkraft class-day view show the complete Klassenverband.
// Nothing else may reference them, which the schema guarantees structurally.
var (
	ErrClassListEntryNotFound = errors.New("class list entry not found")
	// ErrClassListEntryDuplicate: the unique index over the case-folded name
	// and class refused the write. The wrapped chain keeps the driver error
	// so legacy callers can still classify it by the index name.
	ErrClassListEntryDuplicate = errors.New("class list entry already exists in this class")
)

// ClassListEntry is one class-list-only child. SchoolClass keeps the display
// form as entered; every comparison folds case and surrounding whitespace.
type ClassListEntry struct {
	ID          int64     `json:"id"`
	TenantID    int64     `json:"tenant_id"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	FirstName   string    `json:"first_name"`
	LastName    string    `json:"last_name"`
	SchoolClass string    `json:"school_class"`
	// CreatedBy is the creating account; nil for rows created by system paths
	// without an authenticated account.
	CreatedBy *int64 `json:"created_by,omitempty"`
}

// ClassListEntryFields is the writable part of an entry shared by create and
// update. Values are trimmed; an empty field is rejected.
type ClassListEntryFields struct {
	FirstName   string
	LastName    string
	SchoolClass string
}

type CreateClassListEntry struct {
	ClassListEntryFields
	CreatedBy *int64
}

type UpdateClassListEntry struct {
	ID int64
	ClassListEntryFields
}

// ClassListEntryFilter narrows a listing. Every field is optional; the name
// and class matches are case-insensitive and ignore surrounding whitespace,
// exactly like the duplicate guard the legacy repository offered.
type ClassListEntryFilter struct {
	IDs         []int64
	FirstName   string
	LastName    string
	SchoolClass string
}

func (m *Module) FindClassListEntry(ctx context.Context, id int64) (ClassListEntry, error) {
	if id <= 0 {
		return ClassListEntry{}, invalid("class list entry ID is required")
	}
	return m.engine.FindClassListEntry(ctx, id, "")
}

func (m *Module) FindClassListEntryForMutation(ctx context.Context, id int64) (ClassListEntry, error) {
	if id <= 0 {
		return ClassListEntry{}, invalid("class list entry ID is required")
	}
	return m.engine.FindClassListEntry(ctx, id, "UPDATE")
}

func (m *Module) ListClassListEntries(ctx context.Context, filter ClassListEntryFilter) ([]ClassListEntry, error) {
	filter.IDs = uniquePositive(filter.IDs)
	filter.FirstName = strings.TrimSpace(filter.FirstName)
	filter.LastName = strings.TrimSpace(filter.LastName)
	filter.SchoolClass = strings.TrimSpace(filter.SchoolClass)
	return m.engine.ListClassListEntries(ctx, filter)
}

func (m *Module) CreateClassListEntry(ctx context.Context, input CreateClassListEntry) (ClassListEntry, error) {
	if err := validateClassListEntryFields(&input.ClassListEntryFields); err != nil {
		return ClassListEntry{}, err
	}
	if input.CreatedBy != nil && *input.CreatedBy <= 0 {
		input.CreatedBy = nil
	}
	return m.engine.CreateClassListEntry(ctx, input)
}

func (m *Module) UpdateClassListEntry(ctx context.Context, input UpdateClassListEntry) (ClassListEntry, error) {
	if input.ID <= 0 {
		return ClassListEntry{}, invalid("class list entry ID is required")
	}
	if err := validateClassListEntryFields(&input.ClassListEntryFields); err != nil {
		return ClassListEntry{}, err
	}
	return m.engine.UpdateClassListEntry(ctx, input)
}

func (m *Module) DeleteClassListEntry(ctx context.Context, id int64) error {
	if id <= 0 {
		return invalid("class list entry ID is required")
	}
	return m.engine.DeleteClassListEntry(ctx, id)
}

func validateClassListEntryFields(fields *ClassListEntryFields) error {
	fields.FirstName = strings.TrimSpace(fields.FirstName)
	fields.LastName = strings.TrimSpace(fields.LastName)
	fields.SchoolClass = strings.TrimSpace(fields.SchoolClass)
	switch {
	case fields.FirstName == "":
		return invalid("first name is required")
	case fields.LastName == "":
		return invalid("last name is required")
	case fields.SchoolClass == "":
		return invalid("school class is required")
	}
	return nil
}
