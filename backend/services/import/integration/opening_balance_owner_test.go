package integration

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/services"
	"github.com/moto-nrw/project-phoenix/services/import/ports"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// The public Workforce boundary must preserve calendar days, signed hours,
// quota components and error kinds when the import stops using legacy types.
func TestDataImportCutover_OpeningBalanceOwnerContract(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	now := time.Date(2026, time.September, 12, 12, 0, 0, 0, time.UTC)
	module, err := services.NewWorkforceTestModule(db, testpkg.TenantRuntime(t, db), func() time.Time { return now })
	require.NoError(t, err)
	staff := testpkg.CreateTestStaff(t, db, "Opening", "Subject")
	actor := newImporter(t, db)
	hours := services.OpeningBalanceBookingCapability(module.StaffBalanceAdjust)
	vacation := services.VacationTakeoverCapability(module.StaffAbsence)
	date := now.AddDate(0, 0, -1).Format(time.DateOnly)

	require.NoError(t, testpkg.WithTenantTx(t, testpkg.Ctx(t), db, testpkg.Tenant(t), func(ctx context.Context, _ bun.Tx) error {
		require.Error(t, hours.ValidateOpeningBalance(ctx, staff.ID, actor.staffID, "not-a-date", -195, "Migration"))
		require.Error(t, vacation.ValidateVacationOpeningAbsencesBefore(ctx, staff.ID, "not-a-date"))
		require.NoError(t, hours.ValidateOpeningBalance(ctx, staff.ID, actor.staffID, date, -195, "Migration"))
		opening, err := hours.CreateOpeningBalance(ctx, staff.ID, actor.staffID, date, -195, "Migration")
		require.NoError(t, err)
		require.NotNil(t, opening)
		assert.Equal(t, date, opening.EffectiveDate)
		assert.Equal(t, staff.ID, opening.StaffID)
		assert.Equal(t, actor.staffID, opening.DecidedBy)
		assert.Equal(t, "opening", opening.Type)
		assert.ErrorIs(t, hours.ValidateOpeningBalance(ctx, staff.ID, actor.staffID, date, -195, "Migration"), ports.ErrOpeningAlreadyExists)

		year := now.Year()
		require.NoError(t, vacation.UpsertVacationQuota(ctx, staff.ID, year, 30, 4))
		summary, err := vacation.VacationQuotaSummary(ctx, staff.ID, year)
		require.NoError(t, err)
		require.NotNil(t, summary)
		assert.Equal(t, 30.0, summary.EntitledDays)
		assert.Equal(t, 4.0, summary.CarryoverDays)
		require.NoError(t, vacation.ValidateVacationOpeningAbsencesBefore(ctx, staff.ID, date))
		return nil
	}))
}
