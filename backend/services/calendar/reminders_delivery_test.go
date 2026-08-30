package calendar_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/email"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	calModels "github.com/moto-nrw/project-phoenix/models/calendar"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	calendarSvc "github.com/moto-nrw/project-phoenix/services/calendar"
	platformService "github.com/moto-nrw/project-phoenix/services/platform"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// revisingCandidateLock bumps the appointment's revision out of band on the nth
// LockReminderCandidate call. The scan locks the candidate once while queueing
// and once while dispatching, so bumping on the second call reproduces an edit
// landing exactly between a reminder's push claim and the push itself.
type revisingCandidateLock struct {
	calModels.AppointmentRepository
	db    *bun.DB
	on    int
	mu    sync.Mutex
	calls int
}

func (r *revisingCandidateLock) LockReminderCandidate(ctx context.Context, id int64) (*calModels.Appointment, error) {
	r.mu.Lock()
	r.calls++
	revise := r.calls == r.on
	r.mu.Unlock()
	if revise {
		// Deliberately on its own connection and before the row lock is taken:
		// this is another request editing the appointment, not this scan.
		if _, err := r.db.ExecContext(context.Background(),
			`UPDATE calendar.appointments SET revision = revision + 1 WHERE id = ?`, id); err != nil {
			return nil, err
		}
	}
	return r.AppointmentRepository.LockReminderCandidate(ctx, id)
}

// A reminder push is claimed under the appointment revision the scan saw. An
// edit in the seconds before dispatch invalidates that claim — and the scan
// boundary has already moved past the occurrence, so no later tick offers it
// again. The claim has to be re-taken under the new revision instead of being
// dropped, or the reminder is lost for an occurrence that is still due.
func TestCalendarServiceIntegration_ReminderPushSurvivesAnEditBeforeDispatch(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	outbox := &recordingOutbox{}
	notifier := &reminderCaptureNotifier{}
	cfg := calendarTestConfig(db)
	cfg.Outbox = outbox
	cfg.ParentsURL = "https://parents.test"
	cfg.ReminderNotifier = notifier
	cfg.Preferences = reminderPreferences{}
	baseRepo := cfg.AppointmentRepo
	cfg.AppointmentRepo = &revisingCandidateLock{AppointmentRepository: baseRepo, db: db, on: 2}
	service := calendarSvc.NewService(cfg)

	_, organizerAccount := testpkg.CreateTestCalendarStaff(t, db, "Reminder", "Revised")
	parentChain := testpkg.CreateTestParentGuardianChain(t, db)

	ctx := calendarContext(t, organizerAccount.ID)
	appointmentDate := timezone.NewDate(2026, 4, 2)
	detail, err := service.CreateStaffAppointment(ctx, calendarSvc.CreateAppointmentRequest{
		Title:        "Elternabend",
		StartDate:    appointmentDate,
		EndDate:      appointmentDate,
		StartTime:    wallClock(18, 0),
		EndTime:      wallClock(19, 0),
		DeliveryMode: calModels.DeliveryModeInformational,
		SendEmail:    true,
		Targets: []calendarSvc.AppointmentTarget{
			{Type: calModels.TargetTypeGuardianProfile, ID: &parentChain.GuardianProfileID},
		},
	})
	require.NoError(t, err)
	notifier.events = nil

	startsAt := berlinInstant(t, appointmentDate, 18, 0)
	from, to := startsAt.Add(-5*time.Minute), startsAt.Add(5*time.Minute)

	queued, err := service.EnqueueDueAppointmentReminders(ctx, from, to)
	require.NoError(t, err)
	assert.Equal(t, 1, queued)

	revised, err := baseRepo.FindByID(context.Background(), detail.Appointment.ID)
	require.NoError(t, err)
	require.NotNil(t, revised)
	require.Greater(t, revised.Revision, detail.Appointment.Revision,
		"the fixture must actually have revised the appointment mid-flight")

	require.Len(t, notifier.events, 1,
		"an edit between claim and dispatch must not swallow a reminder that is still due")
	assert.Equal(t, []int64{parentChain.AccountID}, notifier.events[0].Audience.GuardianAccountIDs)

	// The replacement claim has to hold, or the same occurrence would be pushed
	// a second time by any scan that sees this window again — which is exactly
	// what the scheduler does after a failed tenant pass.
	_, err = service.EnqueueDueAppointmentReminders(ctx, from, to)
	require.NoError(t, err)
	assert.Len(t, notifier.events, 1, "the re-taken claim must suppress a repeat push")
}

// An appointment mail names a title, a time and a place, and a reminder can wait
// in the outbox for the school's whole lead time. Whoever the row was addressed
// to must still be let through by the child it concerns when it is actually
// sent — the recipient list resolved at queue time is too old to trust.
func TestCalendarServiceIntegration_QueuedAppointmentMailStopsAtRevokedChildAccess(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	outbox := &recordingOutbox{}
	service := setupCalendarServiceWithOutbox(t, db, outbox)
	_, organizerAccount := testpkg.CreateTestCalendarStaff(t, db, "Reminder", "Revoked")
	parentChain := testpkg.CreateTestParentGuardianChain(t, db)

	ctx := calendarContext(t, organizerAccount.ID)
	appointmentDate := timezone.NewDate(2026, 4, 2)
	_, err := service.CreateStaffAppointment(ctx, calendarSvc.CreateAppointmentRequest{
		Title:        "Elternabend",
		StartDate:    appointmentDate,
		EndDate:      appointmentDate,
		StartTime:    wallClock(18, 0),
		EndTime:      wallClock(19, 0),
		DeliveryMode: calModels.DeliveryModeInformational,
		SendEmail:    true,
		Targets: []calendarSvc.AppointmentTarget{
			{Type: calModels.TargetTypeGuardianProfile, ID: &parentChain.GuardianProfileID},
		},
	})
	require.NoError(t, err)

	startsAt := berlinInstant(t, appointmentDate, 18, 0)
	queued, err := service.EnqueueDueAppointmentReminders(ctx, startsAt.Add(-5*time.Minute), startsAt.Add(5*time.Minute))
	require.NoError(t, err)
	require.Equal(t, 1, queued)

	reminder := outbox.enqueued[len(outbox.enqueued)-1]
	require.Equal(t, platformModels.EmailKindAppointmentReminder, reminder.Kind)
	require.Contains(t, reminder.Payload, "guardian_account_id",
		"without the scope the renderer has nothing to re-check")
	require.Contains(t, reminder.Payload, "student_ids")

	row := &platformModels.EmailOutbox{Kind: reminder.Kind, Payload: reminder.Payload}
	row.SetTenantID(parentChain.TenantID)
	render := calendarSvc.NewAppointmentRenderer(calendarSvc.EmailConfig{
		DefaultFrom: email.NewEmail("moto", "no-reply@example.com"),
		DB:          db,
		Guardians:   repositories.NewFactory(db).StudentGuardian,
	})

	msg, err := render(testpkg.WithPackageTenantRuntime(context.Background()), row)
	require.NoError(t, err, "the guardian still has access to the child")
	require.NotNil(t, msg)

	_, err = db.ExecContext(context.Background(), `
		UPDATE users.students_guardians SET permissions = '{}'::jsonb
		WHERE tenant_id = ? AND student_id = ? AND guardian_profile_id = ?
	`, parentChain.TenantID, parentChain.StudentID, parentChain.GuardianProfileID)
	require.NoError(t, err)

	_, err = render(testpkg.WithPackageTenantRuntime(context.Background()), row)
	require.ErrorIs(t, err, platformService.ErrRenderCancelled,
		"a revoked guardian must not be mailed, and retrying cannot change that")
}
