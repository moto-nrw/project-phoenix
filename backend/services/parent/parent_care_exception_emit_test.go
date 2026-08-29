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
	"github.com/moto-nrw/project-phoenix/realtime"
	parentService "github.com/moto-nrw/project-phoenix/services/parent"
	"github.com/moto-nrw/project-phoenix/services/parentmessaging"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// TestSubmitCareException_EmitsSelfServiceMirrorPill wires a real emitter into
// the parent write service so the self-service mirror pill path actually runs:
// a guardian-authored care exception must drop a "care_exception" mirror pill
// into the child's thread (the parent's own timeline of what they changed), and
// deleting it drops a correction pill.
func TestSubmitCareException_EmitsSelfServiceMirrorPill(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repos := repositories.NewFactory(db)

	emitter := parentmessaging.NewEmitter(db, repos.ParentMessageThread, repos.ParentMessage,
		parentSettingsStub{boolDefault: true}, testpkg.NewRecordingBroadcaster(), slog.Default())
	testpkg.SetTenantRuntime(t, emitter, db)
	svc := parentService.NewService(parentService.ServiceConfig{
		ChildRepo:            repos.ParentChild,
		StatusDayRepo:        repos.StudentStatusDay,
		StudentRepo:          repos.Student,
		PickupExceptionRepo:  repos.StudentPickupException,
		ArrivalExceptionRepo: repos.StudentArrivalException,
		Settings: parentSettingsStub{
			boolValues: map[string]bool{
				configModels.KeyParentPickupChangeEnabled: true,
				configModels.KeyParentNotesEnabled:        true,
			},
		},
		Broadcaster:       testpkg.NewRecordingBroadcaster(),
		MessageThreadRepo: repos.ParentMessageThread,
		MessageRepo:       repos.ParentMessage,
		MessageReadRepo:   repos.ParentMessageRead,
		Emitter:           emitter,
		DB:                db,
		Logger:            slog.Default(),
	})

	chain := testpkg.CreateTestParentGuardianChain(t, db)

	// A future in-window date; a fixed clock time is fine (wall-clock only).
	today := timezone.TodayDate()
	date := today.AddDays(3)
	pickup := time.Date(2000, 1, 1, 15, 30, 0, 0, time.UTC)

	exc, err := svc.SubmitCareExceptionWithReason(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID, date, &pickup, "Arzttermin")
	require.NoError(t, err)
	require.NotNil(t, exc)

	// The mirror pill (care_exception) must be in the thread's timeline.
	_, msgs := loadThreadPills(t, db, repos, chain)
	assert.GreaterOrEqual(t, countCareExceptionPills(msgs), 1,
		"a submitted care exception drops a self-service mirror pill")

	// Deleting the exception drops a correction pill (self-service mirror again).
	require.NoError(t, svc.DeleteCareException(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID, date))
	_, msgs = loadThreadPills(t, db, repos, chain)
	assert.GreaterOrEqual(t, len(msgs), 2,
		"deleting the exception drops a correction pill onto the thread")
}

// TestSubmitCareException_WakesEveryGuardian is the regression guard for the
// #1725 review finding that a guardian's self-service write only woke the acting
// guardian: emitSelfServicePill appends a pill to the acting guardian's own
// thread, and broadcastStudentUpdated is a staff-only tenant event that never
// reaches the parents SSE stream — so a co-guardian's open child-detail view kept
// showing a stale pickup time until they refocused or reloaded. The write must
// now fan a message-independent parent_child_updated wake to EVERY guardian.
func TestSubmitCareException_WakesEveryGuardian(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repos := repositories.NewFactory(db)

	// Capture the EMITTER's broadcaster (distinct from the service's own): the
	// guardian fan-out rides BroadcastChildUpdateToGuardians on the emitter.
	emitterBC := testpkg.NewRecordingBroadcaster()
	emitter := parentmessaging.NewEmitter(db, repos.ParentMessageThread, repos.ParentMessage,
		parentSettingsStub{boolDefault: true}, emitterBC, slog.Default())
	testpkg.SetTenantRuntime(t, emitter, db)
	svc := parentService.NewService(parentService.ServiceConfig{
		ChildRepo:            repos.ParentChild,
		StatusDayRepo:        repos.StudentStatusDay,
		StudentRepo:          repos.Student,
		PickupExceptionRepo:  repos.StudentPickupException,
		ArrivalExceptionRepo: repos.StudentArrivalException,
		Settings: parentSettingsStub{
			boolValues: map[string]bool{
				configModels.KeyParentPickupChangeEnabled: true,
				configModels.KeyParentNotesEnabled:        true,
			},
		},
		Broadcaster:       testpkg.NewRecordingBroadcaster(),
		MessageThreadRepo: repos.ParentMessageThread,
		MessageRepo:       repos.ParentMessage,
		MessageReadRepo:   repos.ParentMessageRead,
		Emitter:           emitter,
		DB:                db,
		Logger:            slog.Default(),
	})

	chain := testpkg.CreateTestParentGuardianChain(t, db)

	date := timezone.TodayDate().AddDays(3)
	pickup := time.Date(2000, 1, 1, 15, 30, 0, 0, time.UTC)

	_, err := svc.SubmitCareExceptionWithReason(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID, date, &pickup, "Arzttermin")
	require.NoError(t, err)

	guardianCalls := emitterBC.CallsByMethod("guardian")
	require.NotEmpty(t, guardianCalls,
		"a parent care-exception submit must wake the child's guardians (#1725 review)")
	woke := false
	for _, c := range guardianCalls {
		if c.GuardianID == chain.AccountID {
			woke = true
			assert.Equal(t, realtime.EventParentChildUpdated, c.Event.Type,
				"the wake must carry a parent_child_updated invalidation")
			assert.Equal(t, chain.TenantID, c.TenantID)
		}
	}
	assert.True(t, woke,
		"the child's guardian account %d must be among the woken guardians", chain.AccountID)
}

// loadThreadPills reads the (student, guardian) thread messages under the tenant
// tx (RLS), returning an empty slice when no thread exists.
func loadThreadPills(t *testing.T, db *bun.DB, repos *repositories.Factory, c testpkg.ParentChain) (int64, []*usersModels.ParentMessage) {
	t.Helper()
	var threadID int64
	var msgs []*usersModels.ParentMessage
	err := testpkg.WithTenantTx(t, testpkg.WithPackageTenantRuntime(context.Background()), db, c.TenantID, func(txCtx context.Context, _ bun.Tx) error {
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
