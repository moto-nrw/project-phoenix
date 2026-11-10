package compose_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/appointments"
	portal "github.com/moto-nrw/project-phoenix/modules/schoolcalendar/portal"
	calendarCompose "github.com/moto-nrw/project-phoenix/modules/schoolcalendar/portal/compose"
	"github.com/moto-nrw/project-phoenix/roots/owners"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func recipientReadAppointments(t *testing.T, db *bun.DB, cfg calendarCompose.Config, count int) (context.Context, []int64, testpkg.ParentChain) {
	t.Helper()
	_, organizer := testpkg.CreateTestCalendarStaff(t, db, "Recipient", "Reads")
	family := testpkg.CreateTestParentGuardianChain(t, db)
	ctx := calendarContext(t, organizer.ID)
	service := owners.NewCalendarPortal(cfg)
	ids := make([]int64, 0, count)
	for range count {
		detail, err := service.CreateStaffAppointment(ctx, portal.CreateAppointmentRequest{
			Title: "Recipient read contract", StartDate: portalDate(2026, 10, 12), EndDate: portalDate(2026, 10, 12),
			StartTime: wallClock(14, 0), EndTime: wallClock(15, 0), DeliveryMode: appointments.DeliveryModeInformational,
			SendEmail: true, Targets: []portal.AppointmentTarget{{Type: appointments.TargetTypeGuardianProfile, ID: &family.GuardianProfileID}},
		})
		require.NoError(t, err)
		ids = append(ids, detail.Appointment.ID)
	}
	return ctx, ids, family
}

// Capture every caller separately: mail, push and the public batch projection.
// Setup has no delivery adapters; only the operation under test is measured.
func TestGuardianRecipientReadStatements(t *testing.T) {
	t.Parallel()
	for _, path := range []string{"email", "push", "both", "audience"} {
		for _, size := range []int{3, 8} {
			t.Run(fmt.Sprintf("%s/N%d", path, size), func(t *testing.T) {
				db := testpkg.SetupIsolatedTestDB(t)
				cfg := calendarTestConfig(t, db)
				ctx, ids, family := recipientReadAppointments(t, db, cfg, size)
				outbox, notifier := &recordingOutbox{}, &reminderCaptureNotifier{}
				if path == "email" || path == "both" {
					cfg.Outbox = outbox
				}
				if path == "push" || path == "both" {
					cfg.Notifier, cfg.Preferences = notifier, reminderPreferences{}
				}
				service := owners.NewCalendarPortal(cfg)
				counter := testpkg.CaptureQueries(t, db)
				require.NoError(t, testpkg.WithinTenantContext(t, ctx, db, testpkg.Tenant(t), func(txCtx context.Context) error {
					counter.Reset()
					if path == "audience" {
						audiences, err := service.GuardianNotificationAudiences(txCtx, ids)
						require.NoError(t, err)
						require.Len(t, audiences, size)
						for _, id := range ids {
							require.Equal(t, []int64{family.GuardianProfileID}, audiences[id].GuardianIDs)
							require.Equal(t, []int64{family.StudentID}, audiences[id].StudentsByGuardian[family.GuardianProfileID])
						}
						return nil
					}
					for _, id := range ids {
						detail, err := service.CancelStaffAppointment(txCtx, id)
						require.NoError(t, err)
						require.NotNil(t, detail.Appointment.CancelledAt)
						require.Equal(t, id, detail.Appointment.ID)
						require.Len(t, detail.Recipients, 1)
					}
					return nil
				}))
				links := counter.Matching(func(sql string) bool {
					return strings.Contains(sql, `from calendar.appointment_recipient_students as "appointment_recipient_student"`)
				})
				t.Logf("%s N%d: SELECT=%d standalone-link-SELECT=%d", path, size, len(counter.Operation("SELECT")), len(links))
				require.Empty(t, links, "all three callers reuse the recipient projection")
				if cfg.Outbox != nil {
					require.Len(t, outbox.enqueued, size)
					for _, mail := range outbox.enqueued {
						require.Equal(t, portal.EmailKindAppointmentCancelled, mail.Kind)
					}
				}
				if cfg.Notifier != nil {
					require.Len(t, notifier.events, size)
					for _, event := range notifier.events {
						require.Equal(t, []int64{family.AccountID}, event.Audience.GuardianAccountIDs)
						require.Equal(t, []int64{family.StudentID}, event.Audience.StudentIDs)
					}
				}
			})
		}
	}
}
