package holidayallocation

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
)

func TestDecisionKeepsLocksAndContributionsInOneUnit(t *testing.T) {
	t.Parallel()

	for _, failure := range []string{"", "invoices", "period", "decide", "contributions", "commit"} {
		t.Run("failure="+failure, func(t *testing.T) {
			t.Parallel()
			reviewer := enrollment.HolidayReviewer{OrganizationID: 7, AccountID: 9}
			choices := []enrollment.HolidayAllocationChoice{{BookingID: 12, Placement: "placed", Proposal: "placed"}}
			board := enrollment.HolidayAllocationBoard{AllocationMode: "criteria", PlacedRegistrations: []int64{14, 15}}
			unitCtx, cancel := context.WithCancel(t.Context())
			defer cancel()
			refused := errors.New("injected failure")
			var steps []string
			step := func(ctx context.Context, name string) error {
				require.Equal(t, unitCtx, ctx, "each owner must join the same unit")
				steps = append(steps, name)
				if failure == name {
					return refused
				}
				return nil
			}
			owners := allocationOwners{
				holdInvoices: func(ctx context.Context, org, account, period int64) error {
					require.Equal(t, []int64{7, 9, 11}, []int64{org, account, period})
					return step(ctx, "invoices")
				},
				holdPeriod: func(ctx context.Context, got enrollment.HolidayReviewer, period int64) error {
					require.Equal(t, reviewer, got)
					require.EqualValues(t, 11, period)
					return step(ctx, "period")
				},
				decide: func(ctx context.Context, got enrollment.HolidayReviewer, period int64, gotChoices []enrollment.HolidayAllocationChoice) (enrollment.HolidayAllocationBoard, error) {
					require.Equal(t, reviewer, got)
					require.EqualValues(t, 11, period)
					require.Equal(t, choices, gotChoices)
					return board, step(ctx, "decide")
				},
				refreeze: func(ctx context.Context, org, account, period int64, placed []int64) error {
					require.Equal(t, []int64{7, 9, 11}, []int64{org, account, period})
					require.Equal(t, board.PlacedRegistrations, placed, "include direct allocations and promotions")
					return step(ctx, "contributions")
				},
			}
			within := func(ctx context.Context, org int64, fn func(context.Context) error) error {
				require.Equal(t, t.Context(), ctx)
				require.Equal(t, reviewer.OrganizationID, org)
				steps = append(steps, "begin")
				if err := fn(unitCtx); err != nil {
					steps = append(steps, "rollback")
					return err
				}
				return step(unitCtx, "commit")
			}
			allocation := New(owners, owners, owners, within)
			got, err := allocation.DecideHolidayAllocation(t.Context(), reviewer, 11, choices)
			ordered := []string{"begin", "invoices", "period", "decide", "contributions", "commit"}
			if failure == "" {
				require.NoError(t, err)
				assert.Equal(t, board, got)
				assert.Equal(t, ordered, steps)
				return
			}
			require.ErrorIs(t, err, refused)
			assert.Empty(t, got, "failed units must not expose a decision board")
			for i, name := range ordered {
				if failure == name {
					expected := ordered[: i+1 : i+1]
					if failure != "commit" {
						expected = append(expected, "rollback")
					}
					assert.Equal(t, expected, steps, "stop at the first failed owner command")
					break
				}
			}
		})
	}
}

type allocationOwners struct {
	enrollment.HolidayAllocation
	holdInvoices func(context.Context, int64, int64, int64) error
	holdPeriod   func(context.Context, enrollment.HolidayReviewer, int64) error
	decide       func(context.Context, enrollment.HolidayReviewer, int64, []enrollment.HolidayAllocationChoice) (enrollment.HolidayAllocationBoard, error)
	refreeze     func(context.Context, int64, int64, int64, []int64) error
}

func (o allocationOwners) HoldHolidayAllocationInvoices(ctx context.Context, org, account, period int64) error {
	return o.holdInvoices(ctx, org, account, period)
}

func (o allocationOwners) HoldHolidayPeriod(ctx context.Context, reviewer enrollment.HolidayReviewer, period int64) error {
	return o.holdPeriod(ctx, reviewer, period)
}

func (o allocationOwners) DecideHolidayAllocation(ctx context.Context, reviewer enrollment.HolidayReviewer, period int64, choices []enrollment.HolidayAllocationChoice) (enrollment.HolidayAllocationBoard, error) {
	return o.decide(ctx, reviewer, period, choices)
}

func (o allocationOwners) RefreezeHolidayAllocationContributions(ctx context.Context, org, account, period int64, placed []int64) error {
	return o.refreeze(ctx, org, account, period, placed)
}
