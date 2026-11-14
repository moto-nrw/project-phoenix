package postgres_test

import (
	"strings"
	"testing"

	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSchoolMailHoldsUseTheLastCareDayBeforeTheStatusTick(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	store, ctx := testStore(t, db)
	school := testpkg.Tenant(t)
	guardian := eventGuardian(t, db)
	account := testpkg.CreateTestAccount(t, db, "parent")
	testpkg.EnsureAccountTenant(t, db, account.ID, school)
	_, err := db.ExecContext(ctx, `UPDATE users.guardian_profiles SET account_id = ?, has_account = TRUE WHERE id = ?`, account.ID, guardian.ID)
	require.NoError(t, err)
	child := testpkg.CreateTestStudent(t, db, "Kind", "Muster", "2a")
	testpkg.CreateTestStudentGuardianLink(t, db, child.ID, guardian.ID, "primary_guardian")
	var today calendar.Date
	require.NoError(t, db.NewRaw(`SELECT (CURRENT_TIMESTAMP AT TIME ZONE 'Europe/Berlin')::date`).Scan(ctx, &today))
	tomorrow, yesterday := today.AddDays(1), today.AddDays(-1)
	for _, status := range []string{"active", "pending"} {
		for _, tc := range []struct {
			name     string
			lastDay  *calendar.Date
			departed bool
		}{
			{name: "no end date"},
			{name: "last care day", lastDay: &today},
			{name: "future end date", lastDay: &tomorrow},
			{name: "care ended yesterday", lastDay: &yesterday, departed: true},
		} {
			t.Run(status+"/"+tc.name, func(t *testing.T) {
				_, err := db.ExecContext(ctx, `UPDATE users.student_school_memberships SET status = ?, enrolled_until = ?
					WHERE tenant_id = ? AND student_profile_id = ?`, status, tc.lastDay, school, child.ID)
				require.NoError(t, err)
				for _, address := range []string{eventAddress, account.Email} {
					holds, err := store.SchoolEmailHolds(ctx, school, address)
					require.NoError(t, err)
					assert.Equal(t, tc.departed, holds.Departed, address)
					var known bool
					require.NoError(t, db.NewRaw(`SELECT EXISTS (SELECT 1 FROM platform.delivery_known_email_addresses
						WHERE tenant_id = ? AND address = ?)`, school, strings.ToLower(address)).Scan(ctx, &known))
					assert.Equal(t, !tc.departed, known, "the suppression mark follows the same care boundary: %s", address)
				}
			})
		}
	}
}
