package workforce_test

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	api "github.com/moto-nrw/project-phoenix/roots/owners"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestPublicWorkforceContractUpsertsAndReadsOneVacationQuota(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	database := testpkg.NewDatabase(t, db)
	membership, err := api.NewSchoolMembership(database)
	require.NoError(t, err)
	module, err := api.NewWorkforceWithClock(database, membership, func() time.Time { return time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC) })
	require.NoError(t, err)
	staff := testpkg.CreateTestStaff(t, db, "Workforce", "Contract")
	quota := workforce.StaffVacationQuota{StaffID: staff.ID, Year: 2026, EntitledDays: 30}
	require.NoError(t, module.UpsertStaffVacationQuota(testpkg.Ctx(t), quota))
	rows, err := module.ListStaffVacationQuotas(testpkg.Ctx(t), workforce.StaffVacationFilter{StaffID: staff.ID, Year: 2026})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, quota.EntitledDays, rows[0].EntitledDays)
	id := rows[0].ID
	quota.EntitledDays = 28
	require.NoError(t, module.UpsertStaffVacationQuota(testpkg.Ctx(t), quota))
	rows, err = module.ListStaffVacationQuotas(testpkg.Ctx(t), workforce.StaffVacationFilter{StaffID: staff.ID, Year: 2026})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, id, rows[0].ID)
	require.Equal(t, quota.EntitledDays, rows[0].EntitledDays)
}
