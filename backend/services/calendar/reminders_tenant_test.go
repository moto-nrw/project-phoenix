package calendar_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/appointments"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func TestReminderNamedTablesIsolateTwoTenants(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := calendarTestConfig(db).Appointments
	secondTenant := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, secondTenant)
	tenantIDs := []int64{testpkg.Tenant(t), secondTenant}
	date := appointments.NewDate(2026, time.April, 2)
	type claim struct {
		appointmentID int64
		revision      int
		guardianID    int64
	}
	claims := make(map[int64]claim)
	for _, tenantID := range tenantIDs {
		ctx := testpkg.ContextForTenant(testpkg.WithPackageTenantRuntime(context.Background()), tenantID)
		staff, account := testpkg.CreateTestStaffWithAccountForTenant(t, db, tenantID, "Reminder", "Isolation")
		guardian := testpkg.CreateTestGuardianProfileForTenant(t, db, tenantID, "Reminder", "Isolation", fmt.Sprintf("rls-%d@example.com", tenantID))
		appointment, _, err := module.CreateAppointment(ctx, appointments.CreateAppointment{AppointmentFields: appointments.AppointmentFields{
			OrganizerStaffID: staff.ID, Title: "Reminder isolation", StartDate: date, EndDate: date,
			StartTime: wallClock(18, 0), EndTime: wallClock(19, 0), DeliveryMode: appointments.DeliveryModeInformational,
		}})
		require.NoError(t, err)
		_, _, err = module.CreateAppointmentRecipients(ctx, appointment.ID, []appointments.AppointmentRecipientFields{{
			RecipientType: appointments.RecipientTypeGuardianProfile, GuardianProfileID: &guardian.ID, Status: appointments.ResponseStatusPending,
		}})
		require.NoError(t, err)
		claimed, err := module.ClaimReminderPushDelivery(ctx, appointment.ID, appointment.Revision, date, guardian.ID)
		require.NoError(t, err)
		require.True(t, claimed)
		claims[tenantID] = claim{appointment.ID, appointment.Revision, guardian.ID}
		require.NoError(t, testpkg.WithTenantTx(t, ctx, db, tenantID, func(txCtx context.Context, tx bun.Tx) error {
			_, err := tx.ExecContext(txCtx, `INSERT INTO platform.email_outbox (tenant_id, kind, payload, recipient) VALUES (?, 'appointment_reminder', '{}'::jsonb, '{}'::jsonb)`, tenantID)
			if err != nil {
				return err
			}
			_, err = tx.ExecContext(txCtx, `INSERT INTO iot.push_subscriptions (tenant_id, account_id, portal, endpoint, p256dh, auth) VALUES (?, ?, 'staff', ?, 'fixture-key', 'fixture-auth')`, tenantID, account.ID, fmt.Sprintf("https://fcm.googleapis.com/rls-%d", tenantID))
			return err
		}))
	}

	for index, tenantID := range tenantIDs {
		ctx := testpkg.ContextForTenant(testpkg.WithPackageTenantRuntime(context.Background()), tenantID)
		otherTenant := tenantIDs[1-index]
		for _, table := range []string{"calendar.appointments", "calendar.appointment_recipients", "platform.email_outbox", "iot.push_subscriptions"} {
			require.NoError(t, testpkg.WithTenantTx(t, ctx, db, tenantID, func(txCtx context.Context, tx bun.Tx) error {
				var visibleTenants []int64
				err := tx.NewSelect().TableExpr(table).Column("tenant_id").Distinct().Scan(txCtx, &visibleTenants)
				require.NoError(t, err)
				assert.Equal(t, []int64{tenantID}, visibleTenants, "%s unfiltered reads", table)
				result, err := tx.NewUpdate().TableExpr(table).Set("tenant_id = tenant_id").Where("tenant_id = ?", otherTenant).Exec(txCtx)
				require.NoError(t, err)
				affected, err := result.RowsAffected()
				require.NoError(t, err)
				assert.Zero(t, affected, "%s cross-tenant writes", table)
				return nil
			}))
		}

		// Claims deliberately have no direct tenant grants. Their SECURITY
		// DEFINER functions, rather than table RLS, enforce the tenant boundary.
		err := testpkg.WithTenantTx(t, ctx, db, tenantID, func(txCtx context.Context, tx bun.Tx) error {
			_, err := tx.NewSelect().TableExpr("calendar.appointment_reminder_push_deliveries").Count(txCtx)
			return err
		})
		require.ErrorContains(t, err, "permission denied")
		other := claims[otherTenant]
		require.NoError(t, module.ReleaseReminderPushDelivery(ctx, other.appointmentID, other.revision, date, other.guardianID))
		_, err = module.ClaimReminderPushDelivery(ctx, other.appointmentID, other.revision, date, other.guardianID)
		require.Error(t, err, "a tenant cannot claim another school's appointment")
	}
	for _, tenantID := range tenantIDs {
		ctx := testpkg.ContextForTenant(testpkg.WithPackageTenantRuntime(context.Background()), tenantID)
		own := claims[tenantID]
		claimed, err := module.ClaimReminderPushDelivery(ctx, own.appointmentID, own.revision, date, own.guardianID)
		require.NoError(t, err)
		assert.False(t, claimed, "the other tenant must not have released this claim")
	}
}
