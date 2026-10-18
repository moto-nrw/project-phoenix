package care_test

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/communication"

	"github.com/moto-nrw/project-phoenix/workflows/parentportal"

	"github.com/moto-nrw/project-phoenix/api"

	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// buildNoticeFeedService wires the parent service with the optional news feed
// set explicitly. The cancellation notice (#2601) has no school switch since
// #3731: a sent notice is always visible to the family.
func buildNoticeFeedService(t *testing.T, newsEnabled bool) (*parentportal.Portal, *bun.DB, *api.ParentMessagingSuiteRepositories) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	repos := api.NewRepositoryTestFactory(testpkg.NewDatabase(t, db))
	reposPeople := api.NewPeopleRepositorySuiteFactory(testpkg.NewDatabase(t, db))
	reposMessages := api.NewParentMessagingSuiteRepositories(testpkg.NewDatabase(t, db))
	svc := parentportal.NewMessagingFlows(parentportal.MessagingConfig{
		GuardianPolicy:        api.NewParentGuardianPolicy(),
		Fingerprints:          api.SecurityFingerprints{},
		Declarations:          api.ParentDeclarationPolicy(),
		Runtime:               api.ParentPortalRuntime{},
		Children:              childResolver(db, repos, reposPeople),
		ChildRepo:             repos.ParentChildReads(),
		EnrollmentRequestRepo: repos.ParentEnrollmentReads(),
		AnnouncementRepo:      api.BindParentAnnouncementAccess(reposMessages.ParentAnnouncement),
		StudentRepo:           api.NewParentStudentReads(reposPeople.Student),
		Settings: messagingSettingsStub{
			boolValues: map[string]bool{
				parentportal.KeyParentNewsEnabled: newsEnabled,
			},
		},
		Logger: slog.Default(),
	})
	return svc, db, reposMessages
}

// seedCareCancellationNotice publishes a system-authored notice aimed at one
// child, the shape PublishCareCancellation writes.
func seedCareCancellationNotice(t *testing.T, ctx context.Context, repo api.ParentAnnouncementFixtures, createdBy, tenantID, studentID int64) *communication.ParentAnnouncementFixture {
	t.Helper()
	kind := "care_cancellation"
	a := &communication.ParentAnnouncementFixture{
		Title:      "Betreuung am Dienstag entfällt",
		Body:       "Die Betreuung am Dienstag von 14:00 bis 15:30 fällt aus.",
		Priority:   "important",
		Active:     true,
		CreatedBy:  createdBy,
		SystemKind: &kind,
	}
	a.TenantID = tenantID
	require.NoError(t, repo.Create(ctx, a))
	id := studentID
	require.NoError(t, repo.ReplaceTargets(ctx, tenantID, a.ID, []*communication.ParentAnnouncementTargetFixture{
		{TargetType: "student", TargetRefID: &id},
	}))
	now := time.Now().Add(-time.Minute)
	require.NoError(t, repo.SetPublished(ctx, a.ID, &now))
	persisted, err := repo.FindByID(ctx, a.ID)
	require.NoError(t, err)
	require.NotNil(t, persisted)
	return persisted
}

func TestAnnouncementFeed_NoticeVisibleWhenNewsIsOff(t *testing.T) {
	t.Parallel()
	svc, db, repos := buildNoticeFeedService(t, false)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	seedCtx := tenant.WithTenantID(testpkg.WithPackageTenantRuntime(context.Background()), chain.TenantID)
	handWritten := seedPublishedAnnouncement(t, seedCtx, api.NewParentAnnouncementFixtures(repos.ParentAnnouncement), chain.AccountID, chain.TenantID, false)
	notice := seedCareCancellationNotice(t, seedCtx, api.NewParentAnnouncementFixtures(repos.ParentAnnouncement), chain.AccountID, chain.TenantID, chain.StudentID)

	ctx := testpkg.WithPackageTenantRuntime(context.Background())
	feed, err := svc.ListAnnouncements(ctx, chain.AccountID)
	require.NoError(t, err)
	require.Len(t, feed, 1, "only the notice shows where news is off")
	assert.Equal(t, notice.ID, feed[0].ID)
	require.NotNil(t, feed[0].SystemKind)
	assert.Equal(t, "care_cancellation", *feed[0].SystemKind)

	unread, err := svc.UnreadAnnouncementCount(ctx, chain.AccountID)
	require.NoError(t, err)
	assert.Equal(t, 1, unread)

	// The family can confirm the notice; the hand-written row stays hidden.
	require.NoError(t, svc.MarkAnnouncementRead(ctx, chain.AccountID, notice.ID, *notice.PublishedAt))
	err = svc.MarkAnnouncementRead(ctx, chain.AccountID, handWritten.ID, *handWritten.PublishedAt)
	assert.ErrorIs(t, err, parentportal.ErrAnnouncementNotFound)

	unread, err = svc.UnreadAnnouncementCount(ctx, chain.AccountID)
	require.NoError(t, err)
	assert.Zero(t, unread)
}

func TestAnnouncementFeed_NewsOnShowsNoticeAndLetters(t *testing.T) {
	t.Parallel()
	svc, db, repos := buildNoticeFeedService(t, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	seedCtx := tenant.WithTenantID(testpkg.WithPackageTenantRuntime(context.Background()), chain.TenantID)
	seedPublishedAnnouncement(t, seedCtx, api.NewParentAnnouncementFixtures(repos.ParentAnnouncement), chain.AccountID, chain.TenantID, false)
	seedCareCancellationNotice(t, seedCtx, api.NewParentAnnouncementFixtures(repos.ParentAnnouncement), chain.AccountID, chain.TenantID, chain.StudentID)

	feed, err := svc.ListAnnouncements(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID)
	require.NoError(t, err)
	assert.Len(t, feed, 2)
}
