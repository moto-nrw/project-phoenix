package enrollment_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	owner "github.com/moto-nrw/project-phoenix/modules/enrollment"
	enrollmentTest "github.com/moto-nrw/project-phoenix/modules/enrollment/enrollmenttest"
)

func TestOwnerOffering_CountMaxActiveByCareOfferingInRange_IncludesFutureBookings(t *testing.T) {
	t.Parallel()

	db, _, tenantID, childID, offeringID := setupChildOfferingTest(t)
	repo := enrollmentTest.New()
	from := timezone.NewDate(2026, 8, 24)
	futureFrom := from.AddDays(30)
	until := from.AddDays(90)

	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return repo.InsertRequestChildOffering(ctx, &owner.RequestChildOffering{
			RequestChildID: childID,
			CareOfferingID: offeringID,
			ValidFrom:      offeringDatePointer(futureFrom),
		})
	}))

	var activeToday, peak int
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		var err error
		activeToday, err = repo.OfferingCapacityPeak(ctx, offeringID, nil, owner.Date(from), owner.Date(from.AddDays(1)))
		if err != nil {
			return err
		}
		peak, err = repo.OfferingCapacityPeak(ctx, offeringID, nil, owner.Date(from), owner.Date(until))
		return err
	}))

	assert.Zero(t, activeToday, "a future booking is not active on the range start")
	assert.Equal(t, 1, peak, "a future booking must reserve its later capacity")
}

func TestOwnerOffering_CountMaxActiveByCareOfferingInRangeExcludingRequestChild_ExcludesReplacedIntervals(t *testing.T) {
	t.Parallel()

	db, _, tenantID, childID, offeringID := setupChildOfferingTest(t)
	repo := enrollmentTest.New()
	from := timezone.NewDate(2026, 8, 24)
	until := from.AddDays(90)

	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return repo.InsertRequestChildOffering(ctx, &owner.RequestChildOffering{
			RequestChildID: childID,
			CareOfferingID: offeringID,
		})
	}))

	var peak int
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		var err error
		peak, err = repo.OfferingCapacityPeak(ctx, offeringID, []int64{childID}, owner.Date(from), owner.Date(until))
		return err
	}))

	assert.Zero(t, peak, "the replacing child's old interval must not consume a second slot")
}

func offeringDatePointer(date timezone.Date) *owner.Date {
	value := owner.Date(date)
	return &value
}
