package parentportal_test

import (
	"context"
	"log/slog"
	"testing"
	"time"

	parentportal "github.com/moto-nrw/project-phoenix/workflows/parentportal"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/modules/communication/communicationtest"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	parentportalcompose "github.com/moto-nrw/project-phoenix/workflows/parentportal/compose"
)

// TestSubmitCareException_EmitsSelfServiceMirrorPill wires a real emitter into
// the parent write service so the self-service mirror pill path actually runs:
// a guardian-authored care exception must drop a "care_exception" mirror pill
// into the child's thread (the parent's own timeline of what they changed), and
// deleting it drops a correction pill.
func TestSubmitCareException_EmitsSelfServiceMirrorPill(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repos := api.NewRepositoryTestFactory(testpkg.NewDatabase(t, db))
	reposMessages := api.NewParentMessagingSuiteRepositories(testpkg.NewDatabase(t, db))
	reposPeople := api.NewPeopleRepositorySuiteFactory(testpkg.NewDatabase(t, db))

	emitter := api.NewParentEventEmitter(testpkg.MustDatabase(db), testpkg.TenantRuntime(t, db), reposMessages.ParentMessageThread, communicationtest.NewParentMessageRepository(db),
		parentSettingsStub{boolDefault: true}, testpkg.NewRecordingBroadcaster(), slog.Default())
	settings := parentSettingsStub{
		boolValues: map[string]bool{
			parentportal.KeyParentPickupChangeEnabled: true,
			parentportal.KeyParentNotesEnabled:        true,
		},
	}
	svc := parentportalcompose.New(parentportalcompose.Dependencies{
		GuardianChanges:   api.ParentGuardianChanges(),
		DeparturePolicy:   api.ParentDeparturePolicy(),
		GuardianPolicy:    api.NewParentGuardianPolicy(),
		Fingerprints:      api.SecurityFingerprints{},
		Declarations:      api.ParentDeclarationPolicy(),
		Locales:           api.ParentPortalLocales(),
		Runtime:           api.ParentPortalRuntime{},
		MessagingEnabled:  api.BindParentMessagingGate(settings, slog.Default()),
		CarePlan:          api.NewParentCarePlanCommands(repos.CarePlan(), nil),
		People:            api.MustNewPeopleDirectory(testpkg.MustDatabase(db)),
		CareProfiles:      careProfiles(t, db),
		ChildRepo:         repos.ParentChildReads(),
		StatusDayRepo:     repos.StudentStatusDay,
		StudentRepo:       api.NewParentStudentReads(reposPeople.Student),
		CareExceptions:    api.NewParentCareExceptions(repos.CarePlan()),
		Settings:          settings,
		StudentUpdates:    api.BindParentStudentUpdates(testpkg.NewRecordingBroadcaster()),
		MessageThreadRepo: api.BindParentMessageThreadAccess(reposMessages.ParentMessageThread),
		MessageRepo:       api.BindParentMessageReads(communicationtest.NewParentMessageRepository(db)),
		MessageReadRepo:   api.BindParentMessageInboxReads(reposMessages.ParentMessageRead),
		Conversations:     api.NewParentConversationCore(reposMessages.ParentMessageThread, communicationtest.NewParentMessageRepository(db), reposMessages.ParentMessageRead, testpkg.NewRecordingBroadcaster(), slog.Default()),
		Emitter:           api.BindParentChildEvents(emitter),
		Logger:            slog.Default(),

		DataRequests: api.NewParentDataRequestCommands(repos.CarePlan()),

		CarePlanPolicy: api.NewParentCarePlanPolicy(),
	})

	chain := testpkg.CreateTestParentGuardianChain(t, db)

	// A future in-window date; a fixed clock time is fine (wall-clock only).
	today := calendar.TodayDate()
	date := today.AddDays(3)
	pickup := time.Date(2000, 1, 1, 15, 30, 0, 0, time.UTC)

	exc, err := svc.SubmitCareExceptionWithReason(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID, date, &pickup, "Arzttermin")
	require.NoError(t, err)
	require.NotNil(t, exc)

	// The mirror pill (care_exception) must be in the thread's timeline.
	_, msgs := loadThreadPills(t, db, reposMessages, chain)
	assert.GreaterOrEqual(t, countCareExceptionPills(msgs), 1,
		"a submitted care exception drops a self-service mirror pill")

	// Deleting the exception drops a correction pill (self-service mirror again).
	require.NoError(t, svc.DeleteCareException(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID, date))
	_, msgs = loadThreadPills(t, db, reposMessages, chain)
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
	repos := api.NewRepositoryTestFactory(testpkg.NewDatabase(t, db))
	reposMessages := api.NewParentMessagingSuiteRepositories(testpkg.NewDatabase(t, db))
	reposPeople := api.NewPeopleRepositorySuiteFactory(testpkg.NewDatabase(t, db))

	// Capture the EMITTER's broadcaster (distinct from the service's own): the
	// guardian fan-out rides BroadcastChildUpdateToGuardians on the emitter.
	emitterBC := testpkg.NewRecordingBroadcaster()
	emitter := api.NewParentEventEmitter(testpkg.MustDatabase(db), testpkg.TenantRuntime(t, db), reposMessages.ParentMessageThread, communicationtest.NewParentMessageRepository(db),
		parentSettingsStub{boolDefault: true}, emitterBC, slog.Default())
	settings := parentSettingsStub{
		boolValues: map[string]bool{
			parentportal.KeyParentPickupChangeEnabled: true,
			parentportal.KeyParentNotesEnabled:        true,
		},
	}
	svc := parentportalcompose.New(parentportalcompose.Dependencies{
		GuardianChanges:   api.ParentGuardianChanges(),
		DeparturePolicy:   api.ParentDeparturePolicy(),
		GuardianPolicy:    api.NewParentGuardianPolicy(),
		Fingerprints:      api.SecurityFingerprints{},
		Declarations:      api.ParentDeclarationPolicy(),
		Locales:           api.ParentPortalLocales(),
		Runtime:           api.ParentPortalRuntime{},
		MessagingEnabled:  api.BindParentMessagingGate(settings, slog.Default()),
		CarePlan:          api.NewParentCarePlanCommands(repos.CarePlan(), nil),
		People:            api.MustNewPeopleDirectory(testpkg.MustDatabase(db)),
		CareProfiles:      careProfiles(t, db),
		ChildRepo:         repos.ParentChildReads(),
		StatusDayRepo:     repos.StudentStatusDay,
		StudentRepo:       api.NewParentStudentReads(reposPeople.Student),
		CareExceptions:    api.NewParentCareExceptions(repos.CarePlan()),
		Settings:          settings,
		StudentUpdates:    api.BindParentStudentUpdates(testpkg.NewRecordingBroadcaster()),
		MessageThreadRepo: api.BindParentMessageThreadAccess(reposMessages.ParentMessageThread),
		MessageRepo:       api.BindParentMessageReads(communicationtest.NewParentMessageRepository(db)),
		MessageReadRepo:   api.BindParentMessageInboxReads(reposMessages.ParentMessageRead),
		Conversations:     api.NewParentConversationCore(reposMessages.ParentMessageThread, communicationtest.NewParentMessageRepository(db), reposMessages.ParentMessageRead, testpkg.NewRecordingBroadcaster(), slog.Default()),
		Emitter:           api.BindParentChildEvents(emitter),
		Logger:            slog.Default(),

		DataRequests: api.NewParentDataRequestCommands(repos.CarePlan()),

		CarePlanPolicy: api.NewParentCarePlanPolicy(),
	})

	chain := testpkg.CreateTestParentGuardianChain(t, db)

	date := calendar.TodayDate().AddDays(3)
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
func loadThreadPills(t *testing.T, db *bun.DB, repos *api.ParentMessagingSuiteRepositories, c testpkg.ParentChain) (int64, []*parentportal.ParentMessage) {
	t.Helper()
	var threadID int64
	var msgs []*parentportal.ParentMessage
	err := testpkg.WithTenantTx(t, testpkg.WithPackageTenantRuntime(context.Background()), db, c.TenantID, func(txCtx context.Context, _ bun.Tx) error {
		th, ferr := repos.ParentMessageThread.FindByStudentGuardian(txCtx, c.StudentID, c.AccountID)
		if ferr != nil || th == nil {
			return ferr
		}
		threadID = th.ID
		ms, merr := api.ParentConversationMessagesFixture(txCtx, communicationtest.NewParentMessageRepository(db), th.ID, 100)
		msgs = ms
		return merr
	})
	require.NoError(t, err)
	return threadID, msgs
}

func countCareExceptionPills(msgs []*parentportal.ParentMessage) int {
	n := 0
	for _, m := range msgs {
		if m.EventType == "care_exception" || m.EventType == "care_exception_correction" {
			n++
		}
	}
	return n
}
