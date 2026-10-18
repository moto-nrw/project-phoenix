package studentdeletionhttp

import (
	context "context"

	common "github.com/moto-nrw/project-phoenix/api/common"
	peopleModule "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	securityruntime "github.com/moto-nrw/project-phoenix/modules/securityruntime"
)

// CallerContext is the slice of the Identity & Access caller context the
// student routes read: the verified staff record, the caller's staff member
// (found is false, without an error, for a caller who is no staff member)
// and the student access decision.
type CallerContext interface {
	HasCurrentStaff(ctx context.Context) (bool, error)
	CurrentStaffID(ctx context.Context) (staffID int64, found bool, err error)
	common.StudentAccessSource
	// The note audience (#3632) asks which children this caller actually has:
	// their own and substituted OGS groups, the activities they supervise, and
	// the school classes assigned to them via education.class_teachers.
	MyGroupIDs(ctx context.Context) ([]int64, error)
	MyTeacherGroupIDs(ctx context.Context) ([]int64, error)
	MyActivityGroupIDs(ctx context.Context) ([]int64, error)
	MySchoolClasses(ctx context.Context) ([]string, error)
}

func canDeleteStudent(ctx context.Context, userPermissions []string, student *peopleModule.StudentState, ucs CallerContext) (bool, error) {
	return securityruntime.CanDeleteStudent(ctx, userPermissions, student, ucs)
}
