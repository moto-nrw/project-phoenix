package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/modules/mealplan"
	"github.com/moto-nrw/project-phoenix/modules/mealplan/internal/domain"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// The Mensa-Monatsfreigabe names its school in every statement (#4686). The
// store runs here as the superuser, below row level security, so each
// assertion holds only because of that filter: another school's release of
// the same month, inserted by the same transaction, never shows.
func TestBillingMonthFiltersKeepTheSchoolBelowRLS(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()
	unique := testpkg.UniqueTestTenantID(t)
	organization := testpkg.CreateTestOrganization(t, db, &testpkg.Organization{Name: "Träger Mensa", Slug: fmt.Sprintf("mensa-scope-%d", unique)}).ID
	own := testpkg.CreateTestSchoolInOrganization(t, db, organization, "Eigene Mensa")
	other := testpkg.CreateTestSchoolInOrganization(t, db, organization, "Andere Mensa")
	account := testpkg.CreateTestAccount(t, db, fmt.Sprintf("mensa-%d@example.org", unique)).ID
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()
	var bypasses bool
	require.NoError(t, tx.NewRaw(`SELECT rolsuper OR rolbypassrls FROM pg_roles WHERE rolname = current_user`).Scan(ctx, &bypasses))
	require.True(t, bypasses, "the test runs below row level security")
	storeOf := func(school int64) *BillingMonthStore {
		return NewBillingMonthStore(func(context.Context) (bun.IDB, int64, error) { return tx, school, nil },
			func(context.Context, int64) (bun.IDB, error) { return tx, nil })
	}
	march := calendar.NewDate(2026, 3, 1)
	for _, school := range []int64{other, own} {
		_, err := storeOf(school).InsertRelease(ctx, organization, account, domain.StoredBillingMonth{
			Month: march, ChildCount: 1, MealCount: int(school % 7),
			Children: []domain.BillingChild{{StudentID: school, FirstName: "Kind", LastName: fmt.Sprint(school), RegisteredDays: 7, MealDays: int(school % 7)}},
		})
		require.NoError(t, err)
	}

	months, err := storeOf(own).ReleasedMonths(ctx, march, march)
	require.NoError(t, err)
	require.Len(t, months, 1, "ReleasedMonths")
	assert.Equal(t, int(own%7), months[0].MealCount)

	month, released, err := storeOf(own).ReleasedMonth(ctx, march)
	require.NoError(t, err)
	require.True(t, released)
	require.Len(t, month.Children, 1, "ReleasedMonth")
	assert.Equal(t, own, month.Children[0].StudentID)

	_, err = storeOf(own).InsertRelease(ctx, organization, account, domain.StoredBillingMonth{Month: march})
	assert.ErrorIs(t, err, mealplan.ErrBillingMonthReleased, "a month is released once per school")
}
