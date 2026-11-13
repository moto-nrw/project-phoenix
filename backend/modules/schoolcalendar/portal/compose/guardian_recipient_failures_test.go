package compose_test

import (
	"testing"
	"time"

	"github.com/moto-ogs/project-phoenix/roots/owners"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestGuardianRecipientReadFailuresRollBackCancellation(t *testing.T) {
	t.Parallel()
	for _, table := range []string{"calendar.appointment_recipients", "users.guardian_profiles"} {
		t.Run(table, func(t *testing.T) {
			db := testpkg.SetupIsolatedTestDB(t)
			cfg := calendarTestConfig(t, db)
			ctx, ids, _ := recipientReadAppointments(t, db, cfg, 1)
			outbox, notifier := &recordingOutbox{}, &reminderCaptureNotifier{}
			cfg.Outbox, cfg.Notifier, cfg.Preferences = outbox, notifier, reminderPreferences{}
			service := owners.NewCalendarPortal(cfg)
			_, err := db.ExecContext(ctx, "REVOKE SELECT ON "+table+" FROM phoenix_tenant")
			require.NoError(t, err)
			_, err = service.GuardianNotificationAudiences(ctx, ids)
			require.Equal(t, "42501", testpkg.PostgresSQLState(err))
			_, err = service.CancelStaffAppointment(ctx, ids[0])
			require.Equal(t, "42501", testpkg.PostgresSQLState(err))
			var cancelledAt *time.Time
			require.NoError(t, db.NewRaw(`SELECT cancelled_at FROM calendar.appointments WHERE id = ?`, ids[0]).Scan(ctx, &cancelledAt))
			require.Nil(t, cancelledAt, "the earlier conditional cancellation must roll back")
			require.Empty(t, outbox.enqueued)
			require.Empty(t, notifier.events)
		})
	}
}

// E1: isolate a fault of the redundant SELECT * without breaking the embedded
// link projection, which only reads recipient_id, tenant_id and student_id.
func TestGuardianRecipientReadNeedsOnlyProjectedLinkColumns(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	cfg := calendarTestConfig(t, db)
	ctx, ids, family := recipientReadAppointments(t, db, cfg, 1)
	_, err := db.ExecContext(ctx, `REVOKE SELECT ON calendar.appointment_recipient_students FROM phoenix_tenant;
		GRANT SELECT (recipient_id, tenant_id, student_id) ON calendar.appointment_recipient_students TO phoenix_tenant`)
	require.NoError(t, err)
	var recipientID int64
	require.NoError(t, db.NewRaw(`SELECT id FROM calendar.appointment_recipients WHERE appointment_id = ?`, ids[0]).Scan(ctx, &recipientID))
	_, err = cfg.Appointments.FindAppointmentRecipientStudents(ctx, []int64{recipientID})
	require.Equal(t, "42501", testpkg.PostgresSQLState(err), "the retained reader still exposes its original error")
	service := owners.NewCalendarPortal(cfg)
	audiences, err := service.GuardianNotificationAudiences(ctx, ids)
	require.NoError(t, err)
	require.Equal(t, []int64{family.StudentID}, audiences[ids[0]].StudentsByGuardian[family.GuardianProfileID])
	cfg.Outbox, cfg.Notifier, cfg.Preferences = &recordingOutbox{}, &reminderCaptureNotifier{}, reminderPreferences{}
	_, err = owners.NewCalendarPortal(cfg).CancelStaffAppointment(ctx, ids[0])
	require.NoError(t, err, "neither mail nor push may attempt the removed SELECT")
}
