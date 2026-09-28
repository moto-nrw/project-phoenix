package students

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	peopleModule "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
)

// The routes read and write a child's own row through People Directory's
// public capability (ResourceConfig.PeopleDirectory, #3349). The ports below
// name the rest of the People Directory surface they need, each in the owner's
// public types, so the composition root decides what serves it (#2731).

// PersonRecords is the person half the routes cannot yet reach through the
// owner's capability, because the retained person service still decides it:
// the person writes with their account and RFID-card checks, the student-aware
// bracelet assignment, the dated roster of the group day log and the staff
// member behind a person. The root binds the retained person service
// (services.NewStudentRoutePersons).
type PersonRecords interface {
	// CreatePerson validates and inserts a person, refusing a tag or account
	// that does not exist.
	CreatePerson(ctx context.Context, person peopleModule.CreatePerson) (peopleModule.Person, error)
	// UpdatePerson rewrites a person, refusing a re-pointed tag or account
	// that does not exist.
	UpdatePerson(ctx context.Context, person peopleModule.UpdatePerson) (peopleModule.Person, error)
	DeletePerson(ctx context.Context, personID int64) error
	// AssignStudentTag links the bracelet to the child's person, creating the
	// card and releasing it from a previous holder. A graduated or missing
	// child is refused with peopledirectory.ErrStudentNotFound, re-checked
	// under the child's row lock.
	AssignStudentTag(ctx context.Context, studentID int64, tagID string) error
	UnassignTag(ctx context.Context, personID int64) error
	// ListStudentsForDay returns the children of the groups whose care covers
	// date, as the day log counts them; today is the caller's calendar day.
	ListStudentsForDay(ctx context.Context, groupIDs []int64, date, today timezone.Date) ([]peopleModule.StudentRecord, error)
	// StaffIDForPerson resolves the staff member of a person; found is false
	// for a person who is no staff member.
	StaffIDForPerson(ctx context.Context, personID int64) (staffID int64, found bool, err error)
}

// StudentChangeHistory is the owner's per-child change history (#1455): the
// append the student write records atomically and the history route reads.
// The root binds the People Directory capability.
type StudentChangeHistory interface {
	RecordStudentChanges(ctx context.Context, before, after peopleModule.StudentAuditSnapshot, editedBy int64, editedByName string) error
	ListStudentChangeHistory(ctx context.Context, studentID int64) ([]peopleModule.StudentFieldEdit, error)
}

// StudentPhotoLifecycle is the owner's photo lifecycle (#3349): the feature
// gate, the consent it depends on, the row lock and the file bookkeeping all
// live with People Directory. The root binds its capability.
type StudentPhotoLifecycle interface {
	FindStudentPhoto(ctx context.Context, studentID int64, filename string) (string, error)
	CommitStudentPhoto(ctx context.Context, studentID int64, storedURL string, consentAck bool) error
	ClearStudentPhoto(ctx context.Context, studentID int64) (string, error)
	ApplyStudentPhotoConsent(ctx context.Context, current peopleModule.StudentPhotoState, requestedConsent *bool) peopleModule.StudentPhotoState
}

// StudentConsentCapability is the consent surface of the student rows these
// routes hold: the shared portal projection People Directory folds together,
// and the Audit Platform trail every effective change appends to (#3349).
type StudentConsentCapability interface {
	CurrentStudentConsents(ctx context.Context, snapshot peopleModule.StudentConsentSnapshot, canManagePhoto bool) ([]peopleModule.StudentConsentState, error)
	RecordStudentConsentTransitions(ctx context.Context, before, after peopleModule.StudentConsentSnapshot, source string, actorAccountID *int64, changedAt time.Time) error
}
