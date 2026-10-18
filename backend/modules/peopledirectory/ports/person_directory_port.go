package ports

import (
	"context"

	peopleValues "github.com/moto-nrw/project-phoenix/modules/peopledirectory"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// PersonDirectory is the retained person service: persons, their accounts
// and RFID cards, the student reads and locks the legacy flows use, and the
// staff directory it carries. The application implements it; the roots and
// the HTTP layer name it here (#3752, #4084).
type PersonDirectory interface {
	StaffDirectory
	CountStudentsByGroupIDs(ctx context.Context, groupIDs []int64) (map[int64]int, error)
	Create(ctx context.Context, person *peopleValues.DirectoryPersonSnapshot) error
	Delete(ctx context.Context, id interface{}) error
	FindActiveCaregiverByAccountID(ctx context.Context, accountID int64) (*peopleValues.ActiveCaregiver, error)
	FindByAccountID(ctx context.Context, accountID int64) (*peopleValues.DirectoryPersonSnapshot, error)
	FindByTagID(ctx context.Context, tagID string) (*peopleValues.DirectoryPersonSnapshot, error)
	Get(ctx context.Context, id interface{}) (*peopleValues.DirectoryPersonSnapshot, error)
	GetAllStudentsWithGroups(ctx context.Context) ([]peopleValues.StudentRosterEntry, error)
	GetByIDs(ctx context.Context, ids []int64) (map[int64]*peopleValues.DirectoryPersonSnapshot, error)
	GetEligibleStudentsByGroupIDsOnDate(ctx context.Context, groupIDs []int64, date calendar.Date, today calendar.Date) ([]*peopleValues.StudentState, error)
	GetParticipationCandidatesByGroupIDs(ctx context.Context, groupIDs []int64) ([]*peopleValues.StudentState, error)
	GetStudentByID(ctx context.Context, id int64) (*peopleValues.StudentState, error)
	GetStudentByIDForUpdate(ctx context.Context, id int64) (*peopleValues.StudentState, error)
	GetStudentByPersonID(ctx context.Context, personID int64) (*peopleValues.StudentState, error)
	GetStudentsByGroupID(ctx context.Context, groupID int64) ([]*peopleValues.StudentState, error)
	GetStudentsByGroupIDs(ctx context.Context, groupIDs []int64) ([]*peopleValues.StudentState, error)
	GetStudentsByIDs(ctx context.Context, ids []int64) (map[int64]*peopleValues.StudentState, error)
	GetStudentsWithGroupsByTeacher(ctx context.Context, teacherID int64) ([]peopleValues.StudentRosterEntry, error)
	GetStudentsWithGroupsByTeacherStaffIDs(ctx context.Context, staffIDs []int64) ([]peopleValues.StudentRosterEntry, error)
	LinkStudentToRFIDCard(ctx context.Context, studentID int64, tagID string) error
	LinkToAccount(ctx context.Context, personID int64, accountID int64) error
	LinkToRFIDCard(ctx context.Context, personID int64, tagID string) error
	ListActiveCaregivers(ctx context.Context) ([]*peopleValues.ActiveCaregiver, error)
	ResolveStaffIDByAccountID(ctx context.Context, accountID int64) (int64, error)
	UnlinkFromAccount(ctx context.Context, personID int64) error
	UnlinkFromRFIDCard(ctx context.Context, personID int64) error
	Update(ctx context.Context, person *peopleValues.DirectoryPersonSnapshot) error
	WithCareParticipation(resolve CareParticipationResolver) PersonDirectory
}
