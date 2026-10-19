package ports

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/domain/userscontract"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// PersonDirectoryDependencies contains all dependencies required by the
// person directory.
type PersonDirectoryDependencies struct {
	// PersonDirectory is the owner's person write path (#3349); the
	// composition root binds NewPersonWrites.
	PersonDirectory PersonWriter
	// StudentDirectory is the owner's locked child read (#3349); the
	// composition root binds NewStudentDirectory.
	StudentDirectory StudentLocker
	// Repository dependencies
	PersonRepo    PersonRepository
	RFIDRepo      RFIDCards
	AccountExists func(context.Context, int64) (bool, error)
	StudentRepo   StudentRepository
	TeacherRepo   TeacherRepository
	// StaffDirectory serves the staff and teacher lookups and the two staff
	// writes (#3752); the composition root binds NewStaffDirectory.
	StaffDirectory StaffDirectory
	// CareParticipation is the Care Plan seam behind the dated roster read.
	// Care Plan is composed after this directory, so the root binds a resolver
	// that reaches it at call time.
	CareParticipation CareParticipationResolver
}

// CareParticipationResolver is the Care Plan seam behind the dated visibility
// decision this directory read applies (#3350). Care Plan owns the exit and
// withdrawal boundaries it resolves; the directory only asks which of the
// children it selected still take part in care on the given day.
type CareParticipationResolver func(
	ctx context.Context, studentIDs []int64, on, today calendar.Date,
) (map[int64]bool, error)

// IsPersonNotFound reports whether err is the directory's missing person
// (userscontract.ErrPersonNotFound), for root-composition callers that may not
// import userscontract.
func IsPersonNotFound(err error) bool {
	return errors.Is(err, userscontract.ErrPersonNotFound)
}
