package compose

import (
	"context"

	usersRepo "github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/adapters/peoplepostgres"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/application"
	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
	"github.com/uptrace/bun"
)

// StudentRepositoryDependencies are what the retained student repository
// composes: the child table's retained read surface, the owner it routes the
// reads and writes to, and the School Membership facts it cannot hold itself.
type StudentRepositoryDependencies struct {
	DB       *bun.DB
	Students usersRepo.StudentDirectoryRows
	// Directory is the People Directory owner of the child's row.
	Directory application.StudentRepositoryOwner
	// Enrollment is the School Membership status of the child's enrollment.
	Enrollment application.StudentEnrollmentStatus
	// TeacherGroupIDs and TeacherStaffGroupIDs resolve the groups a teacher
	// (or the teachers of the staff members) is assigned to. The assignment
	// is School Membership's; without a resolver the teacher-scoped roster
	// reads fail closed.
	TeacherGroupIDs      func(context.Context, int64) ([]int64, error)
	TeacherStaffGroupIDs func(context.Context, []int64) ([]int64, error)
}

func NewStudentRepository(dependencies StudentRepositoryDependencies) peoplePorts.StudentRepository {
	repository := usersRepo.NewStudentRepository(NewLegacyRepositoryRuntime(dependencies.DB), dependencies.Students)
	return application.NewBoundStudentRepository(repository, dependencies.Directory, dependencies.Enrollment, dependencies.TeacherGroupIDs, dependencies.TeacherStaffGroupIDs)
}
