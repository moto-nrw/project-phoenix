package postgres_test

import (
	"context"
	"strings"
	"testing"

	deliveryContract "github.com/moto-ogs/project-phoenix/modules/delivery/contract"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLetterDeliveryReportsTheCurrentDepartureHold(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	store, ctx := testStore(t, db)
	school := testpkg.Tenant(t)
	guardian := testpkg.CreateTestGuardianProfile(t, db, "letter-departure")
	child := testpkg.CreateTestStudent(t, db, "Kind", "Muster", "2a")
	testpkg.CreateTestStudentGuardianLink(t, db, child.ID, guardian.ID, "primary_guardian")
	mail := relatedMail(t, db, store, ctx, school, "letter-departure", *guardian.Email, child.ID)
	_, err := db.ExecContext(ctx, `UPDATE platform.email_outbox SET status = 'cancelled', cancelled_at = NOW(), last_error = ? WHERE id = ?`,
		deliveryContract.MailHeldByDeparture, mail)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `INSERT INTO platform.delivery_email_deliveries
		(tenant_id, related_entity_type, related_entity_id, outbox_id, guardian_profile_id, recipient_email, reachability)
		VALUES (?, 'parent_announcement', ?, ?, ?, ?, 'ok')`, school, child.ID, mail, guardian.ID, *guardian.Email)
	require.NoError(t, err)

	for _, phase := range []struct {
		status   string
		departed bool
	}{{status: "inactive", departed: true}, {status: "active"}} {
		_, err := db.ExecContext(ctx, `UPDATE users.student_school_memberships SET status = ? WHERE tenant_id = ? AND student_profile_id = ?`,
			phase.status, school, child.ID)
		require.NoError(t, err)
		inSchool(t, ctx, db, school, func(txCtx context.Context) error {
			rows, err := store.EmailDeliveryStatuses(txCtx, school, "parent_announcement", child.ID)
			require.NoError(t, err)
			require.Len(t, rows, 1)
			assert.Equal(t, phase.departed, rows[0].Departed, phase.status)
			assert.False(t, rows[0].Stopped, "departure is not a manual stop")
			assert.Equal(t, "failed", rows[0].EmailStatus, "the past mail keeps its result")
			assert.Equal(t, deliveryContract.EmailHeldByDeparture, rows[0].Outcome)
			return nil
		})
	}
}

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
