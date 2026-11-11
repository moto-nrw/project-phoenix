package holidayallocation

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
)

// ADR 0206, acceptance criterion 1: every command takes B, then P, then its
// Enrollment command, then the new contributions, all in the one unit of the
// reviewer's Träger; it stops at the first failure and hands Parent Billing
// every registration that gained a place.
func TestHolidayAllocationWorkflowOrdersLocksAndStopsAtTheFirstFailure(t *testing.T) {
	t.Parallel()
	reviewer := enrollment.HolidayReviewer{OrganizationID: 7, AccountID: 9}
	capacity := 3
	commands := []struct {
		name     string
		run      func(context.Context, *Workflow) (any, error)
		want     any
		affected []int64
	}{
		{
			name: "rebooking",
			run: func(ctx context.Context, w *Workflow) (any, error) {
				return w.RebookHolidayChild(ctx, reviewer, 11, 21, 31, []enrollment.HolidayBookingInput{{WeekID: 41, TimeModelID: 51}})
			},
			want:     enrollment.HolidayRegistrationDetail{ID: 21, Promotions: []enrollment.HolidayPromotion{{RegistrationID: 15}, {RegistrationID: 14}, {RegistrationID: 15}}},
			affected: []int64{15, 14},
		},
		{
			name: "capacity",
			run: func(ctx context.Context, w *Workflow) (any, error) {
				return w.SetHolidayWeekCapacity(ctx, reviewer, 11, 41, &capacity)
			},
			want:     enrollment.HolidayCapacityChange{Week: enrollment.HolidayWeekPlaces{WeekID: 41}, Promotions: []enrollment.HolidayPromotion{{RegistrationID: 16}}},
			affected: []int64{16},
		},
		{
			name: "decision",
			run: func(ctx context.Context, w *Workflow) (any, error) {
				return w.DecideHolidayAllocation(ctx, reviewer, 11, []enrollment.HolidayAllocationChoice{{BookingID: 12, Placement: "placed", Proposal: "placed"}})
			},
			// Direct places and promotions alike.
			want:     enrollment.HolidayAllocationBoard{AllocationMode: "criteria", PlacedRegistrations: []int64{14, 15, 17}},
			affected: []int64{14, 15, 17},
		},
	}
	ordered := []string{"begin", "billing", "period", "enrollment", "refreeze", "commit"}
	for _, command := range commands {
		for _, failure := range []string{"", "billing", "period", "enrollment", "refreeze", "commit"} {
			t.Run(command.name+"/failure="+failure, func(t *testing.T) {
				t.Parallel()
				f := newFakeOwners(t, reviewer, failure, command.want, command.affected)
				got, err := command.run(t.Context(), New(f.ports()))
				if failure == "" {
					require.NoError(t, err)
					assert.Equal(t, command.want, got)
					assert.Equal(t, ordered, f.steps)
					return
				}
				require.ErrorIs(t, err, f.refused)
				assert.Zero(t, got, "a failed unit shows no result")
				var want []string
				for _, step := range ordered {
					want = append(want, step)
					if step == failure {
						break
					}
				}
				if failure != "commit" {
					want = append(want, "rollback")
				}
				assert.Equal(t, want, f.steps, "no owner command runs after the first failure")
			})
		}
	}
}

func TestHolidayAllocationWorkflowRefusesAMissingReviewerBeforeTheUnit(t *testing.T) {
	t.Parallel()
	for _, reviewer := range []enrollment.HolidayReviewer{{AccountID: 9}, {OrganizationID: 7}} {
		f := newFakeOwners(t, reviewer, "", enrollment.HolidayAllocationBoard{}, nil)
		_, err := New(f.ports()).DecideHolidayAllocation(t.Context(), reviewer, 11, nil)
		require.ErrorIs(t, err, errReviewerRequired)
		assert.Empty(t, f.steps, "no transaction, lock or owner command without the Büro's keys")
	}
}

// fakeOwners records the order of the owner commands and checks that each
// runs in the unit's context with the reviewer's keys and the period.
type fakeOwners struct {
	t        *testing.T
	reviewer enrollment.HolidayReviewer
	failure  string
	result   any
	affected []int64
	unit     context.Context
	steps    []string
	refused  error
}

type unitKey struct{}

func newFakeOwners(t *testing.T, reviewer enrollment.HolidayReviewer, failure string, result any, affected []int64) *fakeOwners {
	return &fakeOwners{t: t, reviewer: reviewer, failure: failure, result: result, affected: affected,
		unit: context.WithValue(t.Context(), unitKey{}, "träger unit"), refused: errors.New("injected failure")}
}

func (f *fakeOwners) ports() Ports {
	return Ports{Billing: f, Periods: f, Places: f, Allocation: f, Within: f.within}
}

func (f *fakeOwners) step(ctx context.Context, name string) error {
	require.Same(f.t, f.unit, ctx, "%s must join the workflow's unit", name)
	f.steps = append(f.steps, name)
	if f.failure == name {
		return f.refused
	}
	return nil
}

func (f *fakeOwners) within(ctx context.Context, organizationID int64, fn func(context.Context) error) error {
	require.Equal(f.t, f.reviewer.OrganizationID, organizationID, "the unit belongs to the reviewer's Träger")
	require.Nil(f.t, ctx.Value(unitKey{}), "the workflow opens or joins the unit itself")
	f.steps = append(f.steps, "begin")
	if err := fn(f.unit); err != nil {
		f.steps = append(f.steps, "rollback")
		return err
	}
	return f.step(f.unit, "commit")
}

func (f *fakeOwners) keys(organizationID, accountID, periodID int64) {
	require.Equal(f.t, []int64{f.reviewer.OrganizationID, f.reviewer.AccountID, 11}, []int64{organizationID, accountID, periodID})
}

func (f *fakeOwners) HoldHolidayAllocationInvoices(ctx context.Context, organizationID, accountID, periodID int64) error {
	f.keys(organizationID, accountID, periodID)
	return f.step(ctx, "billing")
}

func (f *fakeOwners) RefreezeHolidayAllocationContributions(ctx context.Context, organizationID, accountID, periodID int64, registrationIDs []int64) error {
	f.keys(organizationID, accountID, periodID)
	require.Equal(f.t, f.affected, registrationIDs, "every registration that gained a place, once")
	return f.step(ctx, "refreeze")
}

func (f *fakeOwners) HoldHolidayPeriod(ctx context.Context, reviewer enrollment.HolidayReviewer, periodID int64) error {
	f.keys(reviewer.OrganizationID, reviewer.AccountID, periodID)
	return f.step(ctx, "period")
}

func (f *fakeOwners) RebookHolidayChild(ctx context.Context, reviewer enrollment.HolidayReviewer, periodID, registrationID, childID int64, bookings []enrollment.HolidayBookingInput) (enrollment.HolidayRegistrationDetail, error) {
	f.keys(reviewer.OrganizationID, reviewer.AccountID, periodID)
	require.Equal(f.t, []int64{21, 31}, []int64{registrationID, childID})
	require.Equal(f.t, []enrollment.HolidayBookingInput{{WeekID: 41, TimeModelID: 51}}, bookings)
	result, _ := f.result.(enrollment.HolidayRegistrationDetail)
	return result, f.step(ctx, "enrollment")
}

func (f *fakeOwners) SetHolidayWeekCapacity(ctx context.Context, reviewer enrollment.HolidayReviewer, periodID, weekID int64, capacity *int) (enrollment.HolidayCapacityChange, error) {
	f.keys(reviewer.OrganizationID, reviewer.AccountID, periodID)
	require.EqualValues(f.t, 41, weekID)
	require.Equal(f.t, 3, *capacity)
	result, _ := f.result.(enrollment.HolidayCapacityChange)
	return result, f.step(ctx, "enrollment")
}

func (f *fakeOwners) DecideHolidayAllocation(ctx context.Context, reviewer enrollment.HolidayReviewer, periodID int64, choices []enrollment.HolidayAllocationChoice) (enrollment.HolidayAllocationBoard, error) {
	f.keys(reviewer.OrganizationID, reviewer.AccountID, periodID)
	require.Equal(f.t, []enrollment.HolidayAllocationChoice{{BookingID: 12, Placement: "placed", Proposal: "placed"}}, choices)
	result, _ := f.result.(enrollment.HolidayAllocationBoard)
	return result, f.step(ctx, "enrollment")
}
