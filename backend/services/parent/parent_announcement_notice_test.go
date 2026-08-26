package parent_test

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	configModels "github.com/moto-nrw/project-phoenix/models/config"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	parentService "github.com/moto-nrw/project-phoenix/services/parent"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// buildNoticeFeedService wires the parent service with both feature flags set
// explicitly: the optional news feed and the cancellation notice (#2601).
func buildNoticeFeedService(t *testing.T, newsEnabled, noticeEnabled bool) (parentService.Service, *bun.DB, *repositories.Factory) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	repos := repositories.NewFactory(db)
	svc := parentService.NewService(parentService.ServiceConfig{
		ChildRepo:             repos.ParentChild,
		EnrollmentRequestRepo: repos.ParentEnrollmentRequest,
		AnnouncementRepo:      repos.ParentAnnouncement,
		StudentRepo:           repos.Student,
		Settings: parentSettingsStub{
			boolValues: map[string]bool{
				configModels.KeyParentNewsEnabled:                 newsEnabled,
				configModels.KeyNotificationsCareCancelledEnabled: noticeEnabled,
			},
		},
		DB:     db,
		Logger: slog.Default(),
	})
	return svc, db, repos
}

// seedCareCancellationNotice publishes a system-authored notice aimed at one
// child, the shape PublishCareCancellation writes.
func seedCareCancellationNotice(t *testing.T, ctx context.Context, repo usersModels.ParentAnnouncementRepository, createdBy, tenantID, studentID int64) *usersModels.ParentAnnouncement {
	t.Helper()
	kind := usersModels.ParentAnnouncementSystemKindCareCancellation
	a := &usersModels.ParentAnnouncement{
		Title:      "Betreuung am Dienstag entfällt",
		Body:       "Die Betreuung am Dienstag von 14:00 bis 15:30 fällt aus.",
		Priority:   usersModels.ParentAnnouncementPriorityImportant,
		Active:     true,
		CreatedBy:  createdBy,
		SystemKind: &kind,
	}
	a.SetTenantID(tenantID)
	require.NoError(t, repo.Create(ctx, a))
	id := studentID
	require.NoError(t, repo.ReplaceTargets(ctx, tenantID, a.ID, []*usersModels.ParentAnnouncementTarget{
		{TargetType: usersModels.AnnouncementTargetStudent, TargetRefID: &id},
	}))
	now := time.Now()
	require.NoError(t, repo.SetPublished(ctx, a.ID, &now))
	persisted, err := repo.FindByID(ctx, a.ID)
	require.NoError(t, err)
	require.NotNil(t, persisted)
	return persisted
}

func TestAnnouncementFeed_NoticeVisibleWhenNewsIsOff(t *testing.T) {
	t.Parallel()
	svc, db, repos := buildNoticeFeedService(t, false, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	seedCtx := tenant.WithTenantID(context.Background(), chain.TenantID)
	handWritten := seedPublishedAnnouncement(t, seedCtx, repos.ParentAnnouncement, chain.AccountID, chain.TenantID, false)
	notice := seedCareCancellationNotice(t, seedCtx, repos.ParentAnnouncement, chain.AccountID, chain.TenantID, chain.StudentID)

	ctx := context.Background()
	feed, err := svc.ListAnnouncements(ctx, chain.AccountID)
	require.NoError(t, err)
	require.Len(t, feed, 1, "only the notice shows where news is off")
	assert.Equal(t, notice.ID, feed[0].ID)
	require.NotNil(t, feed[0].SystemKind)
	assert.Equal(t, usersModels.ParentAnnouncementSystemKindCareCancellation, *feed[0].SystemKind)

	unread, err := svc.UnreadAnnouncementCount(ctx, chain.AccountID)
	require.NoError(t, err)
	assert.Equal(t, 1, unread)

	// The family can confirm the notice; the hand-written row stays hidden.
	require.NoError(t, svc.MarkAnnouncementRead(ctx, chain.AccountID, notice.ID, *notice.PublishedAt))
	err = svc.MarkAnnouncementRead(ctx, chain.AccountID, handWritten.ID, *handWritten.PublishedAt)
	assert.ErrorIs(t, err, parentService.ErrAnnouncementNotFound)

	unread, err = svc.UnreadAnnouncementCount(ctx, chain.AccountID)
	require.NoError(t, err)
	assert.Zero(t, unread)
}

func TestAnnouncementFeed_NoticeHiddenWhenBothSwitchesAreOff(t *testing.T) {
	t.Parallel()
	svc, db, repos := buildNoticeFeedService(t, false, false)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	seedCtx := tenant.WithTenantID(context.Background(), chain.TenantID)
	notice := seedCareCancellationNotice(t, seedCtx, repos.ParentAnnouncement, chain.AccountID, chain.TenantID, chain.StudentID)

	ctx := context.Background()
	feed, err := svc.ListAnnouncements(ctx, chain.AccountID)
	require.NoError(t, err)
	assert.Empty(t, feed)

	err = svc.MarkAnnouncementRead(ctx, chain.AccountID, notice.ID, *notice.PublishedAt)
	assert.ErrorIs(t, err, parentService.ErrAnnouncementNotFound)
}

func TestAnnouncementFeed_NewsOnShowsNoticeAndLetters(t *testing.T) {
	t.Parallel()
	svc, db, repos := buildNoticeFeedService(t, true, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	seedCtx := tenant.WithTenantID(context.Background(), chain.TenantID)
	seedPublishedAnnouncement(t, seedCtx, repos.ParentAnnouncement, chain.AccountID, chain.TenantID, false)
	seedCareCancellationNotice(t, seedCtx, repos.ParentAnnouncement, chain.AccountID, chain.TenantID, chain.StudentID)

	feed, err := svc.ListAnnouncements(context.Background(), chain.AccountID)
	require.NoError(t, err)
	assert.Len(t, feed, 2)
}

func TestAnnouncementFeed_NewsOnHidesNoticeWhenNoticeSwitchIsOff(t *testing.T) {
	t.Parallel()
	svc, db, repos := buildNoticeFeedService(t, true, false)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	seedCtx := tenant.WithTenantID(context.Background(), chain.TenantID)
	handWritten := seedPublishedAnnouncement(t, seedCtx, repos.ParentAnnouncement, chain.AccountID, chain.TenantID, false)
	notice := seedCareCancellationNotice(t, seedCtx, repos.ParentAnnouncement, chain.AccountID, chain.TenantID, chain.StudentID)

	feed, err := svc.ListAnnouncements(context.Background(), chain.AccountID)
	require.NoError(t, err)
	require.Len(t, feed, 1)
	assert.Equal(t, handWritten.ID, feed[0].ID)

	unread, err := svc.UnreadAnnouncementCount(context.Background(), chain.AccountID)
	require.NoError(t, err)
	assert.Equal(t, 1, unread)

	require.NoError(t, svc.MarkAnnouncementRead(context.Background(), chain.AccountID, handWritten.ID, *handWritten.PublishedAt))
	err = svc.MarkAnnouncementRead(context.Background(), chain.AccountID, notice.ID, *notice.PublishedAt)
	assert.ErrorIs(t, err, parentService.ErrAnnouncementNotFound)
}
