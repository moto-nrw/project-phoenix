package care

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/careplan"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	parentModels "github.com/moto-nrw/project-phoenix/models/parent"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

type courseWriteChangesStub struct {
	OfferingChangeRequests
	createCalls   int
	withdrawCalls int
}

func (s *courseWriteChangesStub) CreateCourseRequest(
	context.Context,
	careplan.CreateCourseRequestInput,
) (*careplan.OfferingChangeRequest, error) {
	s.createCalls++
	return &careplan.OfferingChangeRequest{}, nil
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
	svc := &Service{Config: Config{
		ChildRepo:       careOfferingsChildRepoStub{child: child},
		StudentRepo:     careOfferingsStudentRepoStub{},
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
