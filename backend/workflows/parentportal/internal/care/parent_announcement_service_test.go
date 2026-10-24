package care_test

// Integration tests for the parent-side announcement feed
// (parent_announcement_service.go): the cross-tenant Neuigkeiten list, the
// unread badge, and the read/acknowledge write path with its audience
// authorization, parent-news feature gate, and stale-version guard. Uses a real
// guardian->student link and real published announcements so the audience
// resolution and read-state flow exercise the repository too.

import (
	"context"
	"log/slog"
	"testing"
	"time"

	parentportalcompose "github.com/moto-nrw/project-phoenix/workflows/parentportal/compose"

	"github.com/moto-nrw/project-phoenix/modules/communication"

	"github.com/moto-nrw/project-phoenix/workflows/parentportal"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/api"
	owners "github.com/moto-nrw/project-phoenix/roots/owners"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// buildAnnouncementService wires the parent service with just the repositories
// the announcement feed needs. newsEnabled controls the per-tenant feature gate.
func buildAnnouncementService(t *testing.T, newsEnabled bool) (*parentportal.Portal, *bun.DB, *api.ParentMessagingSuiteRepositories) {
	t.Helper()
	return buildAnnouncementServiceOn(t, testpkg.SetupIsolatedTestDB(t), newsEnabled)
}

// buildAnnouncementServiceOn wires the service on a database the caller set up.
func buildAnnouncementServiceOn(t *testing.T, db *bun.DB, newsEnabled bool) (*parentportal.Portal, *bun.DB, *api.ParentMessagingSuiteRepositories) {
	t.Helper()
	repos := api.NewRepositoryTestFactory(testpkg.NewDatabase(t, db))
	reposPeople := api.NewPeopleRepositorySuiteFactory(testpkg.NewDatabase(t, db))
	reposMessages := api.NewParentMessagingSuiteRepositories(testpkg.NewDatabase(t, db))
	svc := parentportal.NewMessagingFlows(parentportal.MessagingConfig{
		SharePolicy:           parentportalcompose.NewRequestSharePolicy(),
		GuardianPolicy:        owners.NewParentGuardianPolicy(),
		Fingerprints:          owners.SecurityFingerprints{},
		Declarations:          owners.ParentDeclarationPolicy(),
		Signatures:            owners.ParentDeclarationSignatures{Ledger: owners.SignatureLedger()},
		Runtime:               owners.ParentPortalRuntime{},
		Children:              childResolver(db, repos, reposPeople),
		ChildRepo:             repos.ParentChildReads(),
		EnrollmentRequestRepo: repos.ParentEnrollmentReads(),
		AnnouncementRepo:      owners.BindParentAnnouncementAccess(reposMessages.ParentAnnouncement),
		StudentRepo:           owners.NewParentStudentReads(reposPeople.Student),
		Settings: messagingSettingsStub{
			boolValues: map[string]bool{parentportal.KeyParentNewsEnabled: newsEnabled},
		},
		Logger: slog.Default(),
	})
	return svc, db, reposMessages
}

// seedPublishedAnnouncement creates a school-wide announcement in the tenant,
// publishes it, and registers cleanup. Returns it with PublishedAt reflecting
// the persisted value so callers can echo it to MarkRead/Acknowledge.
func seedPublishedAnnouncement(
	t *testing.T,
	ctx context.Context,
	repo api.ParentAnnouncementFixtures,
	createdBy, tenantID int64,
	requiresAck bool,
) *communication.ParentAnnouncementFixture {
	t.Helper()
	a := &communication.ParentAnnouncementFixture{
		Title:                   "Sommerfest",
		Body:                    "Wir feiern am Freitag.",
		Priority:                "info",
		RequiresAcknowledgement: requiresAck,
		Active:                  true,
		CreatedBy:               createdBy,
	}
	a.TenantID = tenantID
	require.NoError(t, testpkg.TenantCommand1(t, ctx, testpkg.SetupTestDB(t), repo.Create, a))
	require.NoError(t, testpkg.TenantCommand3(t, ctx, testpkg.SetupTestDB(t), repo.ReplaceTargets, tenantID, a.ID, []*communication.ParentAnnouncementTargetFixture{{TargetType: "school_all"}}))
	now := time.Now().Add(-time.Minute)
	require.NoError(t, testpkg.TenantCommand2(t, ctx, testpkg.SetupTestDB(t), repo.SetPublished, a.ID, &now))
	t.Cleanup(func() { _ = testpkg.TenantCommand1(t, ctx, testpkg.SetupTestDB(t), repo.Delete, a.ID) })
	// Re-read so PublishedAt reflects the DB-persisted value (Postgres truncates
	// TIMESTAMPTZ to microseconds). Callers echo this exact instant back to
	// MarkRead/Acknowledge; using the in-memory nanosecond `now` would flakily
	// miss the service's Equal() version guard and surface as ErrAnnouncementStale.
	persisted, err := testpkg.TenantQuery1(t, ctx, testpkg.SetupTestDB(t), repo.FindByID, a.ID)
	require.NoError(t, err)
	require.NotNil(t, persisted)
	require.NotNil(t, persisted.PublishedAt)
	a.PublishedAt = persisted.PublishedAt
	return a
}

func TestAnnouncementFeed_ListUnreadReadAcknowledge(t *testing.T) {
	t.Parallel()
	testpkg.OwnTenant(t)
	svc, db, repos := buildAnnouncementService(t, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	seedCtx := tenant.WithTenantID(testpkg.WithTestTenantRuntime(t, context.Background()), chain.TenantID)
	ann := seedPublishedAnnouncement(t, seedCtx, api.NewParentAnnouncementFixtures(repos.ParentAnnouncement), chain.AccountID, chain.TenantID, true)

	ctx := testpkg.WithTestTenantRuntime(t, context.Background())

	// The school-wide announcement reaches the linked guardian, unread.
	feed, err := svc.ListAnnouncements(ctx, chain.AccountID)
	require.NoError(t, err)
	require.Len(t, feed, 1)
	assert.Equal(t, ann.ID, feed[0].ID)
	assert.Nil(t, feed[0].ReadAt, "not read yet")

	unread, err := svc.UnreadAnnouncementCount(ctx, chain.AccountID)
	require.NoError(t, err)
	assert.Equal(t, 1, unread)

	// Reading alone does not clear the badge while confirmation is still due.
	require.NoError(t, svc.MarkAnnouncementRead(ctx, chain.AccountID, ann.ID, *ann.PublishedAt))

	unread, err = svc.UnreadAnnouncementCount(ctx, chain.AccountID)
	require.NoError(t, err)
	assert.Equal(t, 1, unread)

	feed, err = svc.ListAnnouncements(ctx, chain.AccountID)
	require.NoError(t, err)
	require.Len(t, feed, 1)
	assert.NotNil(t, feed[0].ReadAt, "read state now reflected in the feed")

	// Acknowledge is valid because the announcement requires it.
	require.NoError(t, svc.AcknowledgeAnnouncement(ctx, chain.AccountID, ann.ID, *ann.PublishedAt))
	unread, err = svc.UnreadAnnouncementCount(ctx, chain.AccountID)
	require.NoError(t, err)
	assert.Zero(t, unread)

	feed, err = svc.ListAnnouncements(ctx, chain.AccountID)
	require.NoError(t, err)
	require.Len(t, feed, 1)
	assert.NotNil(t, feed[0].AcknowledgedAt, "acknowledgement now reflected in the feed")
}

func TestAnnouncementFeed_StaleVersionRejected(t *testing.T) {
	t.Parallel()
	testpkg.OwnTenant(t)
	svc, db, repos := buildAnnouncementService(t, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	seedCtx := tenant.WithTenantID(testpkg.WithTestTenantRuntime(t, context.Background()), chain.TenantID)
	ann := seedPublishedAnnouncement(t, seedCtx, api.NewParentAnnouncementFixtures(repos.ParentAnnouncement), chain.AccountID, chain.TenantID, false)

	ctx := testpkg.WithTestTenantRuntime(t, context.Background())
	// A published_at the client never actually saw (wrong version) is rejected as
	// stale, not silently recorded.
	stale := ann.PublishedAt.Add(-time.Hour)
	err := svc.MarkAnnouncementRead(ctx, chain.AccountID, ann.ID, stale)
	assert.ErrorIs(t, err, parentportal.ErrAnnouncementStale)
}

func TestAnnouncementFeed_AckNotRequired(t *testing.T) {
	t.Parallel()
	testpkg.OwnTenant(t)
	svc, db, repos := buildAnnouncementService(t, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	seedCtx := tenant.WithTenantID(testpkg.WithTestTenantRuntime(t, context.Background()), chain.TenantID)
	ann := seedPublishedAnnouncement(t, seedCtx, api.NewParentAnnouncementFixtures(repos.ParentAnnouncement), chain.AccountID, chain.TenantID, false)

	ctx := testpkg.WithTestTenantRuntime(t, context.Background())
	err := svc.AcknowledgeAnnouncement(ctx, chain.AccountID, ann.ID, *ann.PublishedAt)
	assert.ErrorIs(t, err, parentportal.ErrAnnouncementAckNotRequired)
}

func TestAnnouncementFeed_UnknownAnnouncementIsNotFound(t *testing.T) {
	t.Parallel()

	testpkg.OwnTenant(t)
	svc, db, _ := buildAnnouncementService(t, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	ctx := testpkg.WithTestTenantRuntime(t, context.Background())
	// A high id that does not exist collapses to not-found (never leaks existence).
	err := svc.MarkAnnouncementRead(ctx, chain.AccountID, 999999999, time.Now())
	assert.ErrorIs(t, err, parentportal.ErrAnnouncementNotFound)
}

func TestAnnouncementFeed_NewsDisabledHidesEverything(t *testing.T) {
	t.Parallel()
	testpkg.OwnTenant(t)
	svc, db, repos := buildAnnouncementService(t, false) // feature OFF
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	seedCtx := tenant.WithTenantID(testpkg.WithTestTenantRuntime(t, context.Background()), chain.TenantID)
	ann := seedPublishedAnnouncement(t, seedCtx, api.NewParentAnnouncementFixtures(repos.ParentAnnouncement), chain.AccountID, chain.TenantID, true)

	ctx := testpkg.WithTestTenantRuntime(t, context.Background())
	// With the school's news feature off, the feed and badge are empty and a
	// direct read/ack collapses to not-found so no stats accrue for a hidden feed.
	feed, err := svc.ListAnnouncements(ctx, chain.AccountID)
	require.NoError(t, err)
	assert.Empty(t, feed)

	unread, err := svc.UnreadAnnouncementCount(ctx, chain.AccountID)
	require.NoError(t, err)
	assert.Zero(t, unread)

	err = svc.MarkAnnouncementRead(ctx, chain.AccountID, ann.ID, *ann.PublishedAt)
	assert.ErrorIs(t, err, parentportal.ErrAnnouncementNotFound)
}

func TestAnnouncementFeed_RejectsNonPositiveAccount(t *testing.T) {
	t.Parallel()

	svc, _, _ := buildAnnouncementService(t, true)
	ctx := testpkg.WithTestTenantRuntime(t, context.Background())

	_, err := svc.ListAnnouncements(ctx, 0)
	assert.Error(t, err)

	_, err = svc.UnreadAnnouncementCount(ctx, -1)
	assert.Error(t, err)

	err = svc.MarkAnnouncementRead(ctx, 0, 1, time.Now())
	assert.Error(t, err)
}
