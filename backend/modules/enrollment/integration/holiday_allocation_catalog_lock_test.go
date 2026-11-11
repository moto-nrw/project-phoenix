package integration

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func TestHolidayAllocationHoldsTheCatalogUntilCommit(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	w := newPlacesWeek(t, db, "Träger Katalogsperre")
	account := testpkg.CreateTestAccount(t, db, "allocation-catalog")
	_, err := db.ExecContext(t.Context(), `UPDATE enrollment.holiday_periods SET allocation_mode = 'criteria' WHERE organization_id = ? AND id = ?`, w.organization, w.period)
	require.NoError(t, err)
	_, err = db.ExecContext(t.Context(), `UPDATE enrollment.holiday_registration_bookings SET placement = 'pending' WHERE organization_id = ? AND holiday_period_id = ?`, w.organization, w.period)
	require.NoError(t, err)
	var bookingID int64
	require.NoError(t, db.NewRaw(`SELECT min(id) FROM enrollment.holiday_registration_bookings WHERE organization_id = ? AND holiday_period_id = ?`, w.organization, w.period).Scan(t.Context(), &bookingID))
	// This fixture has no sealed health fields. Any cipher call must fail.
	_, _, allocation := compose.NewHolidayRegistrationReview(struct{ enrollment.HolidayHealthCipher }{}, &capturedMails{}, closureTraeger{}, nil)
	reviewer := enrollment.HolidayReviewer{OrganizationID: w.organization, AccountID: account.ID}
	// Both a decision and a replacement must hold the same organization lock,
	// including before any criteria exist. A second transaction cannot save
	// between the decision's criteria snapshot and its queue reads.
	probe := func(want bool) {
		t.Helper()
		require.NoError(t, db.RunInTx(t.Context(), nil, func(ctx context.Context, tx bun.Tx) error {
			var acquired bool
			err := tx.NewRaw(`SELECT pg_try_advisory_xact_lock(hashtextextended('enrollment.holiday_allocation_criteria:' || ?::text, 0))`, w.organization).Scan(ctx, &acquired)
			if err == nil {
				assert.Equal(t, want, acquired)
			}
			return err
		}))
	}
	withinTraeger(t, db, w.organization, func(ctx context.Context) {
		_, err := allocation.DecideHolidayAllocation(ctx, reviewer, w.period, []enrollment.HolidayAllocationChoice{{BookingID: bookingID, Placement: "placed", Proposal: "placed"}})
		require.NoError(t, err)
		probe(false)
	})
	probe(true)
	withinTraeger(t, db, w.organization, func(ctx context.Context) {
		_, err := allocation.SaveHolidayCriteria(ctx, reviewer, nil)
		require.NoError(t, err)
		probe(false)
	})
	probe(true)
}
