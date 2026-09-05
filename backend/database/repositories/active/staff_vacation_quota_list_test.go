package active_test

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/models/active"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStaffVacationQuotaRepositoryListPreservesQueryOptionsBehavior(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repo := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db)).StaffVacationQuota
	ctx := testpkg.Ctx(t)
	firstStaff := testpkg.CreateTestStaff(t, db, "Quota", "First")
	secondStaff := testpkg.CreateTestStaff(t, db, "Quota", "Second")

	first := &active.StaffVacationQuota{StaffID: firstStaff.ID, Year: 2026, EntitledDays: 29}
	second := &active.StaffVacationQuota{StaffID: secondStaff.ID, Year: 2026, EntitledDays: 31}
	first.SetTenantID(testpkg.Tenant(t))
	second.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, repo.Upsert(ctx, first))
	require.NoError(t, repo.Upsert(ctx, second))

	options := modelBase.NewQueryOptions().WithPagination(1, 1)
	options.Filter.Equal("year", 2026)
	options.Sorting = (&modelBase.Sorting{}).AddField("entitled_days", modelBase.SortDesc)

	rows, err := repo.List(ctx, options)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, secondStaff.ID, rows[0].StaffID)

}

func TestStaffVacationQuotaRepositoryListPreservesEmptyNilSlice(t *testing.T) {
	t.Parallel()

	repo := repositories.NewFactory(testpkg.SetupTestDB(t), repositories.NewUnobservedTimetableDependencies(testpkg.SetupTestDB(t))).StaffVacationQuota
	options := modelBase.NewQueryOptions()
	options.Filter.Equal("year", 2099)
	empty, err := repo.List(testpkg.Ctx(t), options)
	require.NoError(t, err)
	assert.Nil(t, empty)
}
