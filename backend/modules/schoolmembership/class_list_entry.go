package schoolmembership

import (
	"context"
	"errors"
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
	ErrClassListEntryNotFound = errors.New("klassenlisteneintrag nicht gefunden")
	// ErrClassListEntryDuplicate: an entry with the same name already exists
	// in the same class (case-insensitive, backed by the DB unique index).
	ErrClassListEntryDuplicate = errors.New("ein Eintrag mit diesem Namen existiert in dieser Klasse bereits")
	// ErrClassListEntryStudentExists: a regular student with the same name
	// already exists in the same class — the child is already on the class
	// list, a duplicate entry would list them twice.
	ErrClassListEntryStudentExists = errors.New("ein Kind mit diesem Namen ist in dieser Klasse bereits angelegt")
	// ErrClassListEntryStudentNotFound: the resolve target does not exist.
	ErrClassListEntryStudentNotFound = errors.New("das ausgewählte Kind wurde nicht gefunden")
	// ErrClassListEntryAssignMismatch: the resolve target is a real student
	// but does not carry the entry's name and class — resolving the entry
	// into it would delete one child's row in favor of a different child.
	ErrClassListEntryAssignMismatch = errors.New("das ausgewählte Kind stimmt nicht mit dem Eintrag überein: Name und Klasse müssen übereinstimmen")
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
