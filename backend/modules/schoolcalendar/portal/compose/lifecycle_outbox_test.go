package compose_test

import (
	"context"
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/api"
	appointments "github.com/moto-nrw/project-phoenix/modules/appointments"
	portal "github.com/moto-nrw/project-phoenix/modules/schoolcalendar/portal"
	owners "github.com/moto-nrw/project-phoenix/roots/owners"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func TestCalendarLifecyclePushJoinsTriggeringTransaction(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{portal.EmailKindAppointmentPublished, portal.EmailKindAppointmentUpdated, portal.EmailKindAppointmentCancelled} {
		t.Run(kind, func(t *testing.T) {
			testpkg.OwnTenant(t)
			db := testpkg.SetupTestDB(t)
			notifications, err := api.NewDeliveryTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
			require.NoError(t, err)
			cfg := calendarTestConfig(t, db)
			cfg.Notifier, cfg.Preferences = api.CalendarOutboxNotifierForTest(notifications), reminderPreferences{}
			calendar := owners.NewCalendarPortal(cfg)
			_, organizer := testpkg.CreateTestCalendarStaff(t, db, "Atomic", "Calendar")
			parent := testpkg.CreateTestParentGuardianChain(t, db)
			ctx := calendarContext(t, organizer.ID)
			date := portalDate(2030, 6, 12)
			request := portal.CreateAppointmentRequest{Title: "Lifecycle push", StartDate: date, EndDate: date, StartTime: wallClock(14, 0), EndTime: wallClock(15, 0), DeliveryMode: appointments.DeliveryModeInformational, SendEmail: true, Targets: []portal.AppointmentTarget{{Type: appointments.TargetTypeGuardianProfile, ID: &parent.GuardianProfileID}}}
			var appointmentID int64
			if kind != portal.EmailKindAppointmentPublished {
				created, err := calendar.CreateStaffAppointment(ctx, request)
				require.NoError(t, err)
				appointmentID = created.Appointment.ID
			}
			count := func(source bun.IDB, queryCtx context.Context, expected int) {
				t.Helper()
				n, err := source.NewSelect().TableExpr("platform.push_outbox").Where("tenant_id = ? AND idempotency_key LIKE ?", testpkg.Tenant(t), "%:"+kind).Count(queryCtx)
				require.NoError(t, err)
				assert.EqualValues(t, expected, n)
			}
			rollback := errors.New("rollback appointment lifecycle")
			trigger := func(commit bool) error {
				return testpkg.WithinTenantTransaction(ctx, db, testpkg.Tenant(t), func(txCtx context.Context, tx bun.Tx) error {
					var err error
					switch kind {
					case portal.EmailKindAppointmentPublished:
						_, err = calendar.CreateStaffAppointment(txCtx, request)
					case portal.EmailKindAppointmentUpdated:
						_, err = calendar.UpdateStaffAppointment(txCtx, appointmentID, portal.UpdateAppointmentRequest{Title: "Updated lifecycle push", StartDate: date, EndDate: date, StartTime: wallClock(14, 0), EndTime: wallClock(15, 0), SendEmail: true, SendEmailSet: true})
					default:
						_, err = calendar.CancelStaffAppointment(txCtx, appointmentID)
					}
					if err != nil {
						return err
					}
					count(tx, txCtx, 1)
					if !commit {
						return rollback
					}
					return nil
				})
			}
			require.ErrorIs(t, trigger(false), rollback)
			count(db, ctx, 0)
			require.NoError(t, trigger(true))
			count(db, ctx, 1)
		})
	}
}
