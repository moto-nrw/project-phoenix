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
//
// The messages are the user-facing German text of the Klassenliste screen —
// the API renders them verbatim, so they are part of the contract, not
// developer prose.
var (
	ErrClassListEntryNotFound = errors.New("Klassenlisteneintrag nicht gefunden") //nolint:staticcheck // ST1005: user-facing German message
	// ErrClassListEntryDuplicate: an entry with the same name already exists
	// in the same class (case-insensitive, backed by the DB unique index).
	ErrClassListEntryDuplicate = errors.New("Ein Eintrag mit diesem Namen existiert in dieser Klasse bereits") //nolint:staticcheck // ST1005: user-facing German message
	// ErrClassListEntryStudentExists: a regular student with the same name
	// already exists in the same class — the child is already on the class
	// list, a duplicate entry would list them twice.
	ErrClassListEntryStudentExists = errors.New("Ein Kind mit diesem Namen ist in dieser Klasse bereits angelegt") //nolint:staticcheck // ST1005: user-facing German message
	// ErrClassListEntryStudentNotFound: the resolve target does not exist.
	ErrClassListEntryStudentNotFound = errors.New("Das ausgewählte Kind wurde nicht gefunden") //nolint:staticcheck // ST1005: user-facing German message
	// ErrClassListEntryAssignMismatch: the resolve target is a real student
	// but does not carry the entry's name and class — resolving the entry
	// into it would delete one child's row in favor of a different child.
	ErrClassListEntryAssignMismatch = errors.New("Das ausgewählte Kind stimmt nicht mit dem Eintrag überein: Name und Klasse müssen übereinstimmen") //nolint:staticcheck // ST1005: user-facing German message
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

// The audited class-list administration (#2382): the Klassenliste screen's
// create, rename, delete and "Zuordnen". Each command guards the name and
// class against both a second entry AND a regular student, and appends an
// audit.class_list_entry_changes row in the same transaction as the write.
// The plain Create/Update/DeleteClassListEntry commands stay unaudited on
// purpose — the import and the grade-transition ledger record their own
// trail and must not produce a second one.
type (
	AddClassListEntry struct {
		ClassListEntryFields
		// ChangedBy is the acting account; 0 for system paths without one.
		ChangedBy int64
	}

	ReviseClassListEntry struct {
		ID int64
		ClassListEntryFields
		ChangedBy int64
	}

	RemoveClassListEntry struct {
		ID        int64
		ChangedBy int64
	}

	// ResolveClassListEntry records the entry as a duplicate of an existing
	// student: the entry is deleted and the trail names the student it was
	// attached to. The child keeps exactly one row on the class list.
	ResolveClassListEntry struct {
		ID        int64
		StudentID int64
		ChangedBy int64
	}
)

// ClassListEntryStudents is the consumer-owned People Directory port behind
// the administration commands. A list entry never references a student, so
// both the duplicate guard and the match hint have to ask the student owner
// whether the name and class are already taken; the module issues no query
// against users.students itself.
type ClassListEntryStudents interface {
	// ListStudentIDsByNameAndClass returns the still-enrolled students of the
	// class carrying exactly that name, matched case-insensitively and
	// trimmed on all three keys.
	ListStudentIDsByNameAndClass(ctx context.Context, firstName, lastName, schoolClass string) ([]int64, error)
	// IsEnrolledStudent reports whether the ID names a student that has not
	// graduated.
	IsEnrolledStudent(ctx context.Context, studentID int64) (bool, error)
}

// ClassListEntryChange is one row of the append-only change trail.
type ClassListEntryChange struct {
	EntryID          int64
	Action           string
	OldValue         string
	NewValue         string
	MatchedStudentID *int64
	ChangedBy        int64
}

// ClassListEntryTrail is the consumer-owned Audit platform port. The change
// is appended inside the transaction of the write it describes, so a refused
// trail never leaves an unaudited change behind.
type ClassListEntryTrail interface {
	AppendClassListEntryChange(context.Context, ClassListEntryChange) error
}

// ClassListEntries is the class-list slice of the capability: everything the
// /api/class-list-entries routes and the class-list exports reach.
type ClassListEntries interface {
	ListClassListEntriesInDisplayOrder(context.Context, ClassListEntryFilter) ([]ClassListEntry, error)
	MatchingStudentIDs(context.Context, ClassListEntryFields) ([]int64, error)
	AddClassListEntry(context.Context, AddClassListEntry) (ClassListEntry, error)
	ReviseClassListEntry(context.Context, ReviseClassListEntry) (ClassListEntry, error)
	RemoveClassListEntry(context.Context, RemoveClassListEntry) error
	ResolveClassListEntry(context.Context, ResolveClassListEntry) error
}

// ClassListEntryFailure classifies a class-list entry outcome for the HTTP
// surfaces: the error contract that used to live in the legacy service
// composition.
type ClassListEntryFailure string

const (
	ClassListEntryFailureInvalidRequest ClassListEntryFailure = "invalid_request"
	ClassListEntryFailureNotFound       ClassListEntryFailure = "not_found"
	ClassListEntryFailureInternal       ClassListEntryFailure = "internal"
)

// ClassifyClassListEntryFailure maps the owner's sentinels onto the outcome
// the class-list routes render: an unknown entry is a 404, every refused
// name, class or resolve target a 400, everything else a 500.
//
// ErrInvalidMembership is the one classification that differs from the legacy
// service, which let its validation errors fall through to a 500. A rejected
// field is the caller's fault, not the server's. No route can reach it — the
// request payload is validated before the command runs — so no rendered
// response changes.
func ClassifyClassListEntryFailure(err error) ClassListEntryFailure {
	switch {
	case err == nil:
		return ""
	case errors.Is(err, ErrClassListEntryNotFound):
		return ClassListEntryFailureNotFound
	case errors.Is(err, ErrClassListEntryDuplicate),
		errors.Is(err, ErrClassListEntryStudentExists),
		errors.Is(err, ErrClassListEntryStudentNotFound),
		errors.Is(err, ErrClassListEntryAssignMismatch),
		errors.Is(err, ErrInvalidMembership):
		return ClassListEntryFailureInvalidRequest
	default:
		return ClassListEntryFailureInternal
	}
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
	return m.engine.ListClassListEntries(ctx, normalizeClassListEntryFilter(filter))
}

// ListClassListEntriesInDisplayOrder lists the entries the way a class list
// reads them: class (grade-aware, so "2a" precedes "10a"), then last and
// first name in German collation. Every surface that renders entries next to
// students uses it, so the two orders cannot drift apart.
func (m *Module) ListClassListEntriesInDisplayOrder(ctx context.Context, filter ClassListEntryFilter) ([]ClassListEntry, error) {
	return m.engine.ListClassListEntriesInDisplayOrder(ctx, normalizeClassListEntryFilter(filter))
}

// MatchingStudentIDs names the still-enrolled students sharing an entry's
// name and class — the candidates for a deliberate "Zuordnen" resolution.
// Matching is a hint only: resolution is ALWAYS an explicit admin action,
// never automatic (#2382: gleichnamige Kinder dürfen nicht verwechselt
// werden).
// It is a read over whatever the rows carry, so it trims but does not
// validate: an entry with an empty field would otherwise fail the whole
// listing instead of simply matching nobody.
func (m *Module) MatchingStudentIDs(ctx context.Context, fields ClassListEntryFields) ([]int64, error) {
	fields.FirstName = strings.TrimSpace(fields.FirstName)
	fields.LastName = strings.TrimSpace(fields.LastName)
	fields.SchoolClass = strings.TrimSpace(fields.SchoolClass)
	return m.engine.MatchingStudentIDs(ctx, fields)
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

func (m *Module) AddClassListEntry(ctx context.Context, input AddClassListEntry) (ClassListEntry, error) {
	if err := validateClassListEntryFields(&input.ClassListEntryFields); err != nil {
		return ClassListEntry{}, err
	}
	return m.engine.AddClassListEntry(ctx, input)
}

func (m *Module) ReviseClassListEntry(ctx context.Context, input ReviseClassListEntry) (ClassListEntry, error) {
	if input.ID <= 0 {
		return ClassListEntry{}, invalid("class list entry ID is required")
	}
	if err := validateClassListEntryFields(&input.ClassListEntryFields); err != nil {
		return ClassListEntry{}, err
	}
	return m.engine.ReviseClassListEntry(ctx, input)
}

func (m *Module) RemoveClassListEntry(ctx context.Context, input RemoveClassListEntry) error {
	if input.ID <= 0 {
		return invalid("class list entry ID is required")
	}
	return m.engine.RemoveClassListEntry(ctx, input)
}

func (m *Module) ResolveClassListEntry(ctx context.Context, input ResolveClassListEntry) error {
	if input.ID <= 0 {
		return invalid("class list entry ID is required")
	}
	if input.StudentID <= 0 {
		return invalid("student ID is required")
	}
	return m.engine.ResolveClassListEntry(ctx, input)
}

// BindClassListEntryAdministration installs the student directory and the
// audit trail the administration commands depend on. The composition root
// builds both only after this module exists, so they arrive late; without
// them the four commands and the match hint refuse with a wiring error while
// every other capability keeps working.
func (m *Module) BindClassListEntryAdministration(students ClassListEntryStudents, trail ClassListEntryTrail) {
	if students == nil || trail == nil {
		panic("school membership: class list entry administration needs a student directory and an audit trail")
	}
	m.engine.BindClassListEntryAdministration(students, trail)
}

func normalizeClassListEntryFilter(filter ClassListEntryFilter) ClassListEntryFilter {
	filter.IDs = uniquePositive(filter.IDs)
	filter.FirstName = strings.TrimSpace(filter.FirstName)
	filter.LastName = strings.TrimSpace(filter.LastName)
	filter.SchoolClass = strings.TrimSpace(filter.SchoolClass)
	return filter
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
