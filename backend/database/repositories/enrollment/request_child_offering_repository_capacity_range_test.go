package enrollment_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
)

func TestRequestChildOfferingRepository_CountMaxActiveByCareOfferingInRange_IncludesFutureBookings(t *testing.T) {
	t.Parallel()

	db, repo, tenantID, childID, offeringID := setupChildOfferingTest(t)
	from := timezone.TodayDate()
	futureFrom := from.AddDays(30)
	until := from.AddDays(90)

	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return repo.Create(ctx, &enrollmentModels.RequestChildOffering{
			RequestChildID: childID,
			CareOfferingID: offeringID,
			ValidFrom:      &futureFrom,
		})
	}))

	var activeToday, peak int
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		var err error
		activeToday, err = repo.CountActiveByCareOfferingOnDate(ctx, offeringID, from)
		if err != nil {
			return err
		}
		peak, err = repo.CountMaxActiveByCareOfferingInRange(ctx, offeringID, from, until)
		return err
	}))

	assert.Zero(t, activeToday, "a future booking is not active on the range start")
	assert.Equal(t, 1, peak, "a future booking must reserve its later capacity")
}

func TestRequestChildOfferingRepository_CountMaxActiveByCareOfferingInRangeExcludingRequestChild_ExcludesReplacedIntervals(t *testing.T) {
	t.Parallel()

	db, repo, tenantID, childID, offeringID := setupChildOfferingTest(t)
	from := timezone.TodayDate()
	until := from.AddDays(90)

	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return repo.Create(ctx, &enrollmentModels.RequestChildOffering{
			RequestChildID: childID,
			CareOfferingID: offeringID,
		})
	}))

	var peak int
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		var err error
		peak, err = repo.CountMaxActiveByCareOfferingInRangeExcludingRequestChild(ctx, offeringID, childID, from, until)
		return err
	}))

	assert.Zero(t, peak, "the replacing child's old interval must not consume a second slot")
}
