package enrollmenthttp_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	capability "github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func TestPhaseExpiryProjection_ListSnapshots_RequiresDatesAndTenant(t *testing.T) {
	t.Parallel()

	repo := NewTestPhaseExpirySnapshots(nil, nil, nil, nil)
	_, err := repo.ListSnapshots(context.Background(), calendar.Date(""), calendar.Date(""))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "dates are required")

	_, err = repo.ListSnapshots(
		context.Background(),
		calendar.NewDate(2027, 2, 1),
		calendar.NewDate(2027, 1, 31),
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "horizon")

	_, err = repo.ListSnapshots(
		context.Background(),
		calendar.NewDate(2027, 1, 2),
		calendar.NewDate(2027, 2, 1),
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tenant context")
}

type failingExpiryStudents struct {
	capability.PhaseExpiryStudents
	err error
}

func (s failingExpiryStudents) ListEnrolledStudents(context.Context) ([]capability.PhaseExpiryStudent, error) {
	return nil, s.err
}

type failingExpiryOfferings struct{ err error }

func (s failingExpiryOfferings) ListCareOfferings(context.Context) ([]capability.PhaseExpiryOffering, error) {
	return nil, s.err
}

type failingExpiryOwner struct{ err error }

type failingExpiryBookings struct{ err error }

func (s failingExpiryBookings) AllCareOfferingLinks(context.Context) ([]capability.CareOfferingLink, error) {
	return nil, s.err
}

func (s failingExpiryOwner) PhaseExpirySnapshots(context.Context, capability.PhaseExpiryInput) ([]*capability.PhaseExpirySnapshot, error) {
	return nil, s.err
}

func TestPhaseExpiryProjection_ListSnapshots_PreservesDependencyFailures(t *testing.T) {
	t.Parallel()
	failure := errors.New("injected phase expiry failure")
	for _, stage := range []string{"students", "care_plan", "bookings", "enrollment"} {
		t.Run(stage, func(t *testing.T) {
			t.Parallel()
			testpkg.OwnTenant(t)
			var studentsErr, offeringsErr, bookingsErr, ownerErr error
			switch stage {
			case "students":
				studentsErr = failure
			case "care_plan":
				offeringsErr = failure
			case "bookings":
				bookingsErr = failure
			case "enrollment":
				ownerErr = failure
			}
			repo := NewTestPhaseExpirySnapshots(failingExpiryOwner{err: ownerErr}, failingExpiryStudents{err: studentsErr}, failingExpiryOfferings{err: offeringsErr}, failingExpiryBookings{err: bookingsErr})
			snapshots, err := repo.ListSnapshots(testpkg.Ctx(t), calendar.NewDate(2027, 1, 2), calendar.NewDate(2027, 2, 1))
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
