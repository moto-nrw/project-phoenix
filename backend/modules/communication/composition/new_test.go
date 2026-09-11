package compose

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/communication"
	organizationCompose "github.com/moto-nrw/project-phoenix/modules/organizationtenancy/compose"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

var errInjectedAudit = errors.New("injected audit failure")

type auditRecorder struct {
	mu      sync.Mutex
	failing bool
	entries []AuditEntry
}

type retryableAuditError struct{}

func (retryableAuditError) Error() string { return "serialization failure" }

func (retryableAuditError) Field(field byte) string {
	if field == 'C' {
		return "40001"
	}
	return ""
}

type retryOnceAudit struct{ attempts int }

func (a *retryOnceAudit) AppendAnnouncementAudit(context.Context, AuditEntry) error {
	a.attempts++
	if a.attempts == 1 {
		return retryableAuditError{}
	}
	return nil
}

func (a *auditRecorder) AppendAnnouncementAudit(_ context.Context, entry AuditEntry) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.failing {
		return errInjectedAudit
	}
	a.entries = append(a.entries, entry)
	return nil
}

func (a *auditRecorder) fail(value bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.failing = value
}

func buildCommunication(t *testing.T, db *bun.DB, audit Audit, observations ...func(Observation)) *communication.Module {
	t.Helper()
	organizations, err := organizationCompose.New(organizationCompose.Dependencies{DB: db, Observe: func(organizationCompose.Observation) {}})
	require.NoError(t, err)
	people, err := peopleCompose.New(peopleCompose.Dependencies{DB: db, Observe: func(peopleCompose.Observation) {}})
	require.NoError(t, err)
	observe := func(Observation) {}
	if len(observations) > 0 {
		observe = observations[0]
	}
	module, err := New(Dependencies{DB: db, Organizations: organizations, People: people, Audit: audit, Observe: observe})
	require.NoError(t, err)
	return module
}

func tenantOrganizationID(t *testing.T, db *bun.DB, tenantID int64) int64 {
	t.Helper()
	var organizationID int64
	require.NoError(t, db.NewSelect().TableExpr("platform.schools").Column("organization_id").Where("id = ?", tenantID).Scan(context.Background(), &organizationID))
	return organizationID
}

func newAnnouncement(title string, targetTenantIDs ...int64) *communication.Announcement {
	return &communication.Announcement{
		Title: title, Content: "Relevant information", Type: communication.TypeAnnouncement,
		Severity: communication.SeverityInfo, Active: true, TargetTenantIDs: targetTenantIDs,
	}
}

func ownAnnouncementsByOperator(t *testing.T, db *bun.DB, operatorID int64) {
	t.Helper()
	t.Cleanup(func() {
		_, err := db.ExecContext(context.Background(), `DELETE FROM platform.announcements WHERE created_by = ?`, operatorID)
		require.NoError(t, err)
	})
}

func TestPlatformAnnouncementsPreserveTargetingReadStateAndViewerNames(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	audit := &auditRecorder{}
	module := buildCommunication(t, db, audit)
	ctx := testpkg.Ctx(t)
	operator := testpkg.CreateTestOperator(t, db)
	ownAnnouncementsByOperator(t, db, operator.ID)
	_, account := testpkg.CreateTestPersonWithAccount(t, db, "Ada", "Leserin")
	tenantID := testpkg.Tenant(t)
	organizationID := tenantOrganizationID(t, db, tenantID)

	otherTenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, otherTenantID)
	otherAccount := testpkg.CreateTestAccount(t, db, "other-reader")
	testpkg.UnclaimTestAccount(t, db, otherAccount.ID)
	testpkg.MapAccountToTenant(t, db, otherAccount.ID, otherTenantID)
	otherOrganizationID := tenantOrganizationID(t, db, otherTenantID)

	announcement := newAnnouncement("Tenant A only", tenantID)
	require.NoError(t, module.CreateAnnouncement(ctx, announcement, operator.ID, nil))
	require.NoError(t, module.PublishAnnouncement(ctx, announcement.ID, operator.ID, nil))

	unread, err := module.GetUnreadForUser(ctx, account.ID, nil, tenantID, organizationID)
	require.NoError(t, err)
	require.Len(t, unread, 1)
	assert.Equal(t, announcement.ID, unread[0].ID)

	otherUnread, err := module.GetUnreadForUser(ctx, otherAccount.ID, nil, otherTenantID, otherOrganizationID)
	require.NoError(t, err)
	assert.Empty(t, otherUnread)

	require.NoError(t, module.MarkSeen(ctx, account.ID, announcement.ID))
	count, err := module.CountUnread(ctx, account.ID, nil, tenantID, organizationID)
	require.NoError(t, err)
	assert.Zero(t, count)

	details, err := module.GetViewDetails(ctx, announcement.ID)
	require.NoError(t, err)
	require.Len(t, details, 1)
	assert.Equal(t, "Ada Leserin", details[0].UserName)
	assert.Equal(t, account.Email, details[0].AccountEmail)

	stats, err := module.GetStats(ctx, announcement.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, stats.SeenCount)
	assert.Equal(t, 1, stats.TargetCount)
}

func TestAnnouncementWritesRollBackAfterAuditFailureAndRetryOnce(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	audit := &auditRecorder{failing: true}
	module := buildCommunication(t, db, audit)
	ctx := testpkg.Ctx(t)
	operator := testpkg.CreateTestOperator(t, db)
	ownAnnouncementsByOperator(t, db, operator.ID)
	announcement := newAnnouncement("Rollback create", testpkg.Tenant(t))

	require.ErrorIs(t, module.CreateAnnouncement(ctx, announcement, operator.ID, nil), errInjectedAudit)
	var count int
	require.NoError(t, db.NewSelect().TableExpr("platform.announcements").ColumnExpr("COUNT(*)").Where("title = ?", announcement.Title).Scan(context.Background(), &count))
	assert.Zero(t, count)

	audit.fail(false)
	require.NoError(t, module.CreateAnnouncement(ctx, announcement, operator.ID, nil))
	require.Positive(t, announcement.ID)

	announcement.Title = "Rollback update"
	audit.fail(true)
	require.ErrorIs(t, module.UpdateAnnouncement(ctx, announcement, operator.ID, nil), errInjectedAudit)
	persisted, err := module.GetAnnouncement(ctx, announcement.ID)
	require.NoError(t, err)
	assert.Equal(t, "Rollback create", persisted.Title)

	audit.fail(false)
	require.NoError(t, module.UpdateAnnouncement(ctx, announcement, operator.ID, nil))
	audit.fail(true)
	require.ErrorIs(t, module.PublishAnnouncement(ctx, announcement.ID, operator.ID, nil), errInjectedAudit)
	persisted, err = module.GetAnnouncement(ctx, announcement.ID)
	require.NoError(t, err)
	assert.Nil(t, persisted.PublishedAt)

	audit.fail(false)
	require.NoError(t, module.PublishAnnouncement(ctx, announcement.ID, operator.ID, nil))
	audit.fail(true)
	require.ErrorIs(t, module.UnpublishAnnouncement(ctx, announcement.ID, operator.ID, nil), errInjectedAudit)
	persisted, err = module.GetAnnouncement(ctx, announcement.ID)
	require.NoError(t, err)
	assert.NotNil(t, persisted.PublishedAt)

	audit.fail(false)
	require.NoError(t, module.UnpublishAnnouncement(ctx, announcement.ID, operator.ID, nil))
	audit.fail(true)
	require.ErrorIs(t, module.DeleteAnnouncement(ctx, announcement.ID, operator.ID, nil), errInjectedAudit)
	_, err = module.GetAnnouncement(ctx, announcement.ID)
	require.NoError(t, err)

	audit.fail(false)
	require.NoError(t, module.DeleteAnnouncement(ctx, announcement.ID, operator.ID, nil))
	_, err = module.GetAnnouncement(ctx, announcement.ID)
	assert.ErrorIs(t, err, communication.ErrAnnouncementNotFound)
}

func TestCreateAnnouncementRetriesWholeTransactionAfterSerializationFailure(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	audit := &retryOnceAudit{}
	var observed Observation
	module := buildCommunication(t, db, audit, func(value Observation) { observed = value })
	operator := testpkg.CreateTestOperator(t, db)
	ownAnnouncementsByOperator(t, db, operator.ID)
	announcement := newAnnouncement("Automatic retry", testpkg.Tenant(t))

	require.NoError(t, module.CreateAnnouncement(testpkg.Ctx(t), announcement, operator.ID, nil))
	assert.Equal(t, 2, audit.attempts)
	assert.Equal(t, 6, observed.Stats.Queries)
	assert.EqualValues(t, 3, observed.Stats.Rows)
	assert.Positive(t, announcement.ID)

	var count int
	require.NoError(t, db.NewSelect().TableExpr("platform.announcements").ColumnExpr("COUNT(*)").Where("title = ?", announcement.Title).Scan(context.Background(), &count))
	assert.Equal(t, 1, count)
}

func TestAnnouncementViewWriteRollsBackWithOuterUnitOfWorkAndRetryIsIdempotent(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	observations := make([]Observation, 0, 3)
	var observationsMu sync.Mutex
	module := buildCommunication(t, db, &auditRecorder{}, func(observation Observation) {
		observationsMu.Lock()
		defer observationsMu.Unlock()
		observations = append(observations, observation)
	})
	ctx := testpkg.Ctx(t)
	operator := testpkg.CreateTestOperator(t, db)
	ownAnnouncementsByOperator(t, db, operator.ID)
	_, account := testpkg.CreateTestPersonWithAccount(t, db, "Ida", "Retry")
	announcement := newAnnouncement("Read state", testpkg.Tenant(t))
	require.NoError(t, module.CreateAnnouncement(ctx, announcement, operator.ID, nil))

	err := tenant.WithinAdmin(ctx, func(txCtx context.Context) error {
		require.NoError(t, module.MarkSeen(txCtx, account.ID, announcement.ID))
		return errInjectedAudit
	})
	require.ErrorIs(t, err, errInjectedAudit)
	var count int
	require.NoError(t, db.NewSelect().TableExpr("platform.announcement_views").ColumnExpr("COUNT(*)").Where("user_id = ? AND announcement_id = ?", account.ID, announcement.ID).Scan(context.Background(), &count))
	assert.Zero(t, count)

	require.NoError(t, module.MarkSeen(ctx, account.ID, announcement.ID))
	require.NoError(t, module.MarkSeen(ctx, account.ID, announcement.ID))
	observationsMu.Lock()
	defer observationsMu.Unlock()
	var conflicts int
	for _, observation := range observations {
		conflicts += observation.Stats.DuplicatePreventionConflicts
	}
	assert.Equal(t, 1, conflicts)
}

func TestInvalidTargetIsStableAndDoesNotWrite(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildCommunication(t, db, &auditRecorder{})
	operator := testpkg.CreateTestOperator(t, db)
	ownAnnouncementsByOperator(t, db, operator.ID)
	announcement := newAnnouncement("Bad target", testpkg.UniqueTestTenantID(t))

	err := module.CreateAnnouncement(testpkg.Ctx(t), announcement, operator.ID, nil)
	assert.ErrorIs(t, err, communication.ErrInvalidAnnouncement)
	var invalid *communication.InvalidDataError
	assert.ErrorAs(t, err, &invalid)
	assert.Zero(t, announcement.ID)
}
