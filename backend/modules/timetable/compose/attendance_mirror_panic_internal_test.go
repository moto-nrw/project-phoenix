package compose

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/getsentry/sentry-go"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mirrorSentryTransport chan *sentry.Event

func (t mirrorSentryTransport) Configure(sentry.ClientOptions)        {}
func (t mirrorSentryTransport) Flush(time.Duration) bool              { return true }
func (t mirrorSentryTransport) FlushWithContext(context.Context) bool { return true }
func (t mirrorSentryTransport) Close()                                {}
func (t mirrorSentryTransport) SendEvent(event *sentry.Event)         { t <- event }

// panickingInstanceRepo panics from every method: its embedded repository
// is nil, as a broken wiring would leave it.
type panickingInstanceRepo struct {
	scheduleModel.ActivityInstanceRepository
}

// Issue #3640: the attendance sync turns a panic into an error for the
// transaction owner. The panic itself must reach Sentry, since the error no
// longer carries its stack, and the next sync runs normally.
func TestAttendanceMirrorReportsRecoveredPanic(t *testing.T) {
	t.Parallel()

	transport := make(mirrorSentryTransport, 4)
	client, err := sentry.NewClient(sentry.ClientOptions{Transport: transport})
	require.NoError(t, err)
	ctx := sentry.SetHubOnContext(context.Background(), sentry.NewHub(client, sentry.NewScope()))
	ctx = tenant.WithTenantID(ctx, 44)
	mirror := &attendanceMirror{
		instanceRepo: panickingInstanceRepo{},
		logger:       slog.New(slog.DiscardHandler),
	}

	snapshot, err := mirror.MirrorCheckInForVisit(ctx, timetable.AttendanceVisit{StudentID: 1, ActiveGroupID: 2, EntryTime: time.Now()})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "visit check-in sync panic")
	assert.Nil(t, snapshot)
	require.Len(t, transport, 1, "the recovered panic is exactly one event")
	event := <-transport
	assert.Equal(t, sentry.LevelFatal, event.Level)
	assert.Equal(t, "44", event.Tags["school_id"], "the sync ran in school 44's transaction")

	snapshot, err = mirror.MirrorCheckInForVisit(ctx, timetable.AttendanceVisit{StudentID: 1, EntryTime: time.Now()})

	require.NoError(t, err)
	assert.Nil(t, snapshot)
	assert.Empty(t, transport, "the next sync runs normally and reports nothing")
}

func TestMirrorSchoolIDIsAbsentOutsideATenantTransaction(t *testing.T) {
	t.Parallel()

	assert.Zero(t, mirrorSchoolID(context.Background()))
	assert.Equal(t, int64(9), mirrorSchoolID(tenant.WithTenantID(context.Background(), 9)))
}
