package parent

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	parentModels "github.com/moto-nrw/project-phoenix/models/parent"
	enrollmentSvc "github.com/moto-nrw/project-phoenix/services/enrollment"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

type courseWriteChangesStub struct {
	enrollmentSvc.OfferingChangeRequestService
	createCalls   int
	withdrawCalls int
}

func (s *courseWriteChangesStub) CreateCourseRequest(
	context.Context,
	enrollmentSvc.CreateCourseRequestInput,
) (*enrollmentModels.OfferingChangeRequest, error) {
	s.createCalls++
	return &enrollmentModels.OfferingChangeRequest{}, nil
}

func (s *courseWriteChangesStub) WithdrawCourseRequest(
	context.Context,
	int64, int64, int64,
) error {
	s.withdrawCalls++
	return nil
}

func TestCourseWritesRequireCatalogPermissionBeforeMutation(t *testing.T) {
	t.Parallel()

	child := &parentModels.ChildSummary{
		StudentID: 22,
		TenantID:  testpkg.Tenant(t),
		GuardianPermissions: map[string]interface{}{
			authorize.GuardianPermissionEnrollmentSubmit: true,
		},
	}
	changes := &courseWriteChangesStub{}
	svc := &service{ServiceConfig: ServiceConfig{
		ChildRepo:       careOfferingsChildRepoStub{child: child},
		StudentRepo:     careOfferingsStudentRepoStub{},
		DB:              careOfferingsTestDB(t),
		OfferingChanges: changes,
	}}
	ctx := testpkg.WithPackageTenantRuntime(context.Background())

	_, err := svc.RequestChildCourse(ctx, 11, child.StudentID, 33, "Bitte")
	require.ErrorIs(t, err, ErrGuardianPermissionDenied)

	_, err = svc.WithdrawChildCourseRequest(ctx, 11, child.StudentID, 44)
	require.ErrorIs(t, err, ErrGuardianPermissionDenied)

	assert.Zero(t, changes.createCalls)
	assert.Zero(t, changes.withdrawCalls)
}
