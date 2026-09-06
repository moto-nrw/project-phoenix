package enrollment_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	capability "github.com/moto-nrw/project-phoenix/modules/enrollment"
	enrollmentService "github.com/moto-nrw/project-phoenix/services/enrollment"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func TestPhaseExpiryProjection_ListSnapshots_RequiresDatesAndTenant(t *testing.T) {
	t.Parallel()

	repo := enrollmentService.NewPhaseExpiryProjection(nil, nil, nil)
	_, err := repo.ListSnapshots(context.Background(), timezone.Date(""), timezone.Date(""))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "dates are required")

	_, err = repo.ListSnapshots(
		context.Background(),
		timezone.NewDate(2027, 2, 1),
		timezone.NewDate(2027, 1, 31),
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "horizon")

	_, err = repo.ListSnapshots(
		context.Background(),
		timezone.NewDate(2027, 1, 2),
		timezone.NewDate(2027, 2, 1),
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tenant context")
}

type failingExpiryStudents struct {
	enrollmentService.PhaseExpiryStudents
	err error
}

func (s failingExpiryStudents) ListEnrolledStudents(context.Context) ([]enrollmentService.PhaseExpiryStudent, error) {
	return nil, s.err
}

type failingExpiryOfferings struct{ err error }

func (s failingExpiryOfferings) ListCareOfferings(context.Context) ([]enrollmentService.PhaseExpiryOffering, error) {
	return nil, s.err
}

type failingExpiryOwner struct{ err error }

func (s failingExpiryOwner) PhaseExpirySnapshots(context.Context, capability.PhaseExpiryInput) ([]*capability.PhaseExpirySnapshot, error) {
	return nil, s.err
}

func TestPhaseExpiryProjection_ListSnapshots_PreservesDependencyFailures(t *testing.T) {
	t.Parallel()
	failure := errors.New("injected phase expiry failure")
	for _, stage := range []string{"students", "care_plan", "enrollment"} {
		t.Run(stage, func(t *testing.T) {
			t.Parallel()
			testpkg.OwnTenant(t)
			var studentsErr, offeringsErr, ownerErr error
			switch stage {
			case "students":
				studentsErr = failure
			case "care_plan":
				offeringsErr = failure
			case "enrollment":
				ownerErr = failure
			}
			repo := enrollmentService.NewPhaseExpiryProjection(failingExpiryOwner{err: ownerErr}, failingExpiryStudents{err: studentsErr}, failingExpiryOfferings{err: offeringsErr})
			snapshots, err := repo.ListSnapshots(testpkg.Ctx(t), timezone.NewDate(2027, 1, 2), timezone.NewDate(2027, 2, 1))
			require.ErrorIs(t, err, failure)
			require.Nil(t, snapshots)
			if stage == "care_plan" {
				assert.EqualError(t, err, "list care offerings for phase expiry report: injected phase expiry failure")
			} else {
				assert.EqualError(t, err, failure.Error())
			}
		})
	}
}
