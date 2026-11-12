package owners

import (
	"context"

	enrollment "github.com/moto-ogs/project-phoenix/modules/enrollment"
	enrollmentCompose "github.com/moto-ogs/project-phoenix/modules/enrollment/compose"
	"github.com/moto-ogs/project-phoenix/modules/peopledirectory"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

// EnrollmentStudentMatches is People Directory's exact enrolled-child read
// and ordered row lock, bound to the intake's consumer-owned port below.
type EnrollmentStudentMatches interface {
	ExistsEnrolledByNameAndBirthday(context.Context, int64, string, string, calendar.Date) (bool, error)
	FindEnrolledStudentIDByNameAndBirthday(context.Context, int64, string, string, calendar.Date) (*int64, error)
	FindEnrolledStudentIDsByIdentities(context.Context, int64, []peopledirectory.StudentIdentity) ([][]int64, error)
	FindByIDsForUpdate(context.Context, []int64) (map[int64]*peopledirectory.StudentState, error)
}

type enrollmentStudentMatches struct{ EnrollmentStudentMatches }

func enrollmentMatches(source EnrollmentStudentMatches) enrollmentCompose.StudentMatches {
	if source == nil {
		return nil
	}
	return enrollmentStudentMatches{source}
}

func (s enrollmentStudentMatches) FindEnrolledStudentIDsByIdentities(ctx context.Context, tenantID int64, identities []enrollment.StudentIdentity) ([][]int64, error) {
	probes := make([]peopledirectory.StudentIdentity, len(identities))
	for i, identity := range identities {
		probes[i] = peopledirectory.StudentIdentity{FirstName: identity.FirstName, LastName: identity.LastName, Birthday: identity.Birthday}
	}
	return s.EnrollmentStudentMatches.FindEnrolledStudentIDsByIdentities(ctx, tenantID, probes)
}

func (s enrollmentStudentMatches) LockStudents(ctx context.Context, ids []int64) error {
	_, err := s.FindByIDsForUpdate(ctx, ids)
	return err
}
