package parent_test

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	repositories "github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	configModels "github.com/moto-nrw/project-phoenix/models/config"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	configService "github.com/moto-nrw/project-phoenix/services/config"
	parentService "github.com/moto-nrw/project-phoenix/services/parent"
	"github.com/moto-nrw/project-phoenix/services/parentmessaging"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// careExcSettings turns on the pickup-change feature (so a care exception is
// accepted) and parent notes (so the emitter writes the self-service pill).
type careExcSettings struct {
	configService.SettingsService
}

func (careExcSettings) ResolveBoolForTenant(_ context.Context, _ int64, key string) (bool, error) {
	switch key {
	case configModels.KeyParentPickupChangeEnabled, configModels.KeyParentNotesEnabled:
		return true, nil
	default:
		return false, nil
	}
}

type notesResolver struct{}

func (notesResolver) ResolveBoolForTenant(context.Context, int64, string) (bool, error) {
	return true, nil
}

// TestSubmitCareException_EmitsSelfServiceMirrorPill wires a real emitter into
// the parent write service so the self-service mirror pill path actually runs:
// a guardian-authored care exception must drop a "care_exception" mirror pill
// into the child's thread (the parent's own timeline of what they changed), and
// deleting it drops a correction pill.
func TestSubmitCareException_EmitsSelfServiceMirrorPill(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { require.NoError(t, db.Close()) }()
	repos := repositories.NewFactory(db)

	emitter := parentmessaging.NewEmitter(db, repos.ParentMessageThread, repos.ParentMessage,
		notesResolver{}, testpkg.NewRecordingBroadcaster(), slog.Default())
	svc := parentService.NewService(parentService.ServiceConfig{
		ChildRepo:            repos.ParentChild,
		StatusDayRepo:        repos.StudentStatusDay,
		StudentRepo:          repos.Student,
		PickupExceptionRepo:  repos.StudentPickupException,
		ArrivalExceptionRepo: repos.StudentArrivalException,
		Settings:             careExcSettings{},
		Broadcaster:          testpkg.NewRecordingBroadcaster(),
		MessageThreadRepo:    repos.ParentMessageThread,
		MessageRepo:          repos.ParentMessage,
		MessageReadRepo:      repos.ParentMessageRead,
		Emitter:              emitter,
		DB:                   db,
		Logger:               slog.Default(),
	})

	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	// A future in-window date; a fixed clock time is fine (wall-clock only).
	today := timezone.TodayDate()
	date := today.AddDays(3)
	pickup := time.Date(2000, 1, 1, 15, 30, 0, 0, time.UTC)

	exc, err := svc.SubmitCareException(context.Background(), chain.AccountID, chain.StudentID, date, &pickup, nil)
	require.NoError(t, err)
	require.NotNil(t, exc)

	// The mirror pill (care_exception) must be in the thread's timeline.
	_, msgs := loadThreadPills(t, db, repos, chain)
	assert.GreaterOrEqual(t, countCareExceptionPills(msgs), 1,
		"a submitted care exception drops a self-service mirror pill")

	// Deleting the exception drops a correction pill (self-service mirror again).
	require.NoError(t, svc.DeleteCareException(context.Background(), chain.AccountID, chain.StudentID, date))
	_, msgs = loadThreadPills(t, db, repos, chain)
	assert.GreaterOrEqual(t, len(msgs), 2,
		"deleting the exception drops a correction pill onto the thread")
}

// loadThreadPills reads the (student, guardian) thread messages under the tenant
// tx (RLS), returning an empty slice when no thread exists.
func loadThreadPills(t *testing.T, db *bun.DB, repos *repositories.Factory, c testpkg.ParentChain) (int64, []*usersModels.ParentMessage) {
	t.Helper()
	var threadID int64
	var msgs []*usersModels.ParentMessage
	err := tenant.WithTenantTx(context.Background(), db, c.TenantID, func(txCtx context.Context, _ bun.Tx) error {
		th, ferr := repos.ParentMessageThread.FindByStudentGuardian(txCtx, c.StudentID, c.AccountID)
		if ferr != nil || th == nil {
			return ferr
		}
		threadID = th.ID
		ms, merr := repos.ParentMessage.ListByThread(txCtx, th.ID, 100)
		msgs = ms
		return merr
	})
	require.NoError(t, err)
	return threadID, msgs
}

func countCareExceptionPills(msgs []*usersModels.ParentMessage) int {
	n := 0
	for _, m := range msgs {
		if m.EventType == "care_exception" || m.EventType == "care_exception_correction" {
			n++
		}
	}
	return n
}
