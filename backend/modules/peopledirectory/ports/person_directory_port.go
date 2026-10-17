package ports

import (
	"context"

	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// PersonDirectory is the retained person service: persons, their accounts
// and RFID cards, the student reads and locks the legacy flows use, and the
// staff directory it carries. The application implements it; the roots and
// the HTTP layer name it here (#3752, #4084).
type PersonDirectory interface {
	StaffDirectory
	CountStudentsByGroupIDs(ctx context.Context, groupIDs []int64) (map[int64]int, error)
	Create(ctx context.Context, person *users.Person) error
	Delete(ctx context.Context, id interface{}) error
	FindActiveCaregiverByAccountID(ctx context.Context, accountID int64) (*users.ActiveCaregiver, error)
	FindByAccountID(ctx context.Context, accountID int64) (*users.Person, error)
	FindByTagID(ctx context.Context, tagID string) (*users.Person, error)
	Get(ctx context.Context, id interface{}) (*users.Person, error)
	GetAllStudentsWithGroups(ctx context.Context) ([]users.StudentWithGroupInfo, error)
	GetByIDs(ctx context.Context, ids []int64) (map[int64]*users.Person, error)
	GetEligibleStudentsByGroupIDsOnDate(ctx context.Context, groupIDs []int64, date calendar.Date, today calendar.Date) ([]*users.Student, error)
	GetParticipationCandidatesByGroupIDs(ctx context.Context, groupIDs []int64) ([]*users.Student, error)
	GetStudentByID(ctx context.Context, id int64) (*users.Student, error)
	GetStudentByIDForUpdate(ctx context.Context, id int64) (*users.Student, error)
	GetStudentByPersonID(ctx context.Context, personID int64) (*users.Student, error)
	GetStudentsByGroupID(ctx context.Context, groupID int64) ([]*users.Student, error)
	GetStudentsByGroupIDs(ctx context.Context, groupIDs []int64) ([]*users.Student, error)
	GetStudentsByIDs(ctx context.Context, ids []int64) (map[int64]*users.Student, error)
	GetStudentsWithGroupsByTeacher(ctx context.Context, teacherID int64) ([]users.StudentWithGroupInfo, error)
	GetStudentsWithGroupsByTeacherStaffIDs(ctx context.Context, staffIDs []int64) ([]users.StudentWithGroupInfo, error)
	LinkStudentToRFIDCard(ctx context.Context, studentID int64, tagID string) error
	LinkToAccount(ctx context.Context, personID int64, accountID int64) error
	LinkToRFIDCard(ctx context.Context, personID int64, tagID string) error
	ListActiveCaregivers(ctx context.Context) ([]*users.ActiveCaregiver, error)
	ResolveStaffIDByAccountID(ctx context.Context, accountID int64) (int64, error)
	UnlinkFromAccount(ctx context.Context, personID int64) error
	UnlinkFromRFIDCard(ctx context.Context, personID int64) error
	Update(ctx context.Context, person *users.Person) error
	WithCareParticipation(resolve CareParticipationResolver) PersonDirectory
}
