package audience

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/staffannouncementaudience/contract"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// audienceFixture stages one reader in the test's school and one in a
// foreign school of another organisation, with one published announcement
// addressed to each school.
type audienceFixture struct {
	projection                      *Projection
	tenantID, organizationID        int64
	foreignID, foreignOrganization  int64
	readerID, foreignReaderID       int64
	ownAnnouncement, foreignMessage int64
	publishedAt                     time.Time
}

func setupAudience(t *testing.T) (*bun.DB, audienceFixture) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()
	fix := audienceFixture{projection: New(func(context.Context) (bun.IDB, error) { return db, nil })}
	fix.tenantID = testpkg.Tenant(t)
	fix.foreignID = testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, fix.foreignID)
	require.NoError(t, db.NewRaw("SELECT organization_id FROM platform.schools WHERE id = ?", fix.tenantID).Scan(ctx, &fix.organizationID))
	require.NoError(t, db.NewRaw("SELECT organization_id FROM platform.schools WHERE id = ?", fix.foreignID).Scan(ctx, &fix.foreignOrganization))
	require.NotEqual(t, fix.organizationID, fix.foreignOrganization)

	fix.readerID = testpkg.CreateTestAccount(t, db, "audience-reader").ID
	foreignReader := testpkg.CreateTestAccount(t, db, "audience-foreign-reader")
	testpkg.UnclaimTestAccount(t, db, foreignReader.ID)
	testpkg.MapAccountToTenant(t, db, foreignReader.ID, fix.foreignID)
	fix.foreignReaderID = foreignReader.ID

	operator := testpkg.CreateTestOperator(t, db)
	t.Cleanup(func() {
		_, err := db.ExecContext(context.Background(), `DELETE FROM platform.announcements WHERE created_by = ?`, operator.ID)
		require.NoError(t, err)
	})
	fix.ownAnnouncement = publishAnnouncement(t, db, operator.ID, "Own school", fix.tenantID)
	fix.foreignMessage = publishAnnouncement(t, db, operator.ID, "Foreign school", fix.foreignID)
	require.NoError(t, db.NewRaw("SELECT published_at FROM platform.announcements WHERE id = ?", fix.foreignMessage).Scan(ctx, &fix.publishedAt))
	return db, fix
}

func publishAnnouncement(t *testing.T, db *bun.DB, operatorID int64, title string, tenantID int64) int64 {
	t.Helper()
	var id int64
	require.NoError(t, db.NewRaw(`INSERT INTO platform.announcements
		(title, content, type, severity, active, published_at, target_tenant_ids, created_by)
		VALUES (?, 'Relevant information', 'announcement', 'info', TRUE, NOW(), ARRAY[?]::bigint[], ?)
		RETURNING id`, title, tenantID, operatorID).Scan(context.Background(), &id))
	return id
}

func announcementIDs(values []contract.Announcement) []int64 {
	ids := make([]int64, 0, len(values))
	for _, value := range values {
		ids = append(ids, value.ID)
	}
	return ids
}

// TestProjection_UnreadStaysInTheReadersSchool pins the result rows and that
// an announcement addressed to another school never reaches a reader, even
// when the read names that school.
func TestProjection_UnreadStaysInTheReadersSchool(t *testing.T) {
	t.Parallel()
	_, fix := setupAudience(t)
	ctx := context.Background()

	unread, stats, err := fix.projection.Unread(ctx, fix.readerID, nil, fix.tenantID, fix.organizationID)
	require.NoError(t, err)
	assert.Contains(t, announcementIDs(unread), fix.ownAnnouncement)
	assert.NotContains(t, announcementIDs(unread), fix.foreignMessage, "an announcement for another school must not appear")
	assert.Equal(t, contract.Stats{Queries: 1, Rows: int64(len(unread)), StatementDuration: stats.StatementDuration}, stats)
	for _, value := range unread {
		if value.ID == fix.ownAnnouncement {
			assert.Equal(t, "Own school", value.Title)
			assert.Equal(t, []int64{fix.tenantID}, value.TargetTenantIDs)
			assert.True(t, value.Active)
			require.NotNil(t, value.PublishedAt)
		}
	}
	count, _, err := fix.projection.CountUnread(ctx, fix.readerID, nil, fix.tenantID, fix.organizationID)
	require.NoError(t, err)
	assert.Len(t, unread, count)

	foreign, _, err := fix.projection.Unread(ctx, fix.foreignReaderID, nil, fix.foreignID, fix.foreignOrganization)
	require.NoError(t, err)
	assert.Contains(t, announcementIDs(foreign), fix.foreignMessage)
	assert.NotContains(t, announcementIDs(foreign), fix.ownAnnouncement, "an announcement for another school must not appear")

	crossed, _, err := fix.projection.Unread(ctx, fix.readerID, nil, fix.foreignID, fix.foreignOrganization)
	require.NoError(t, err)
	assert.Empty(t, crossed, "a reader without a mapping to the named school sees none of its announcements")
	crossedCount, _, err := fix.projection.CountUnread(ctx, fix.readerID, nil, fix.foreignID, fix.foreignOrganization)
	require.NoError(t, err)
	assert.Zero(t, crossedCount)
}

// TestProjection_TargetsAndViewsStayInTheAddressedSchool pins the audience
// count and the view details: neither counts nor lists a reader of another
// school.
func TestProjection_TargetsAndViewsStayInTheAddressedSchool(t *testing.T) {
	t.Parallel()
	db, fix := setupAudience(t)
	ctx := context.Background()

	targets, stats, err := fix.projection.TargetCount(ctx, contract.Target{PublishedAt: &fix.publishedAt, TargetTenantIDs: []int64{fix.foreignID}})
	require.NoError(t, err)
	assert.Equal(t, 1, targets, "only the foreign school's reader is addressed")
	assert.Equal(t, 1, stats.Queries)

	_, err = db.ExecContext(ctx, `INSERT INTO platform.announcement_views (user_id, announcement_id) VALUES (?, ?)`, fix.foreignReaderID, fix.foreignMessage)
	require.NoError(t, err)
	views, _, err := fix.projection.ViewDetails(ctx, fix.foreignMessage)
	require.NoError(t, err)
	require.Len(t, views, 1)
	assert.Equal(t, fix.foreignReaderID, views[0].UserID)
	assert.False(t, views[0].Dismissed)

	ownViews, _, err := fix.projection.ViewDetails(ctx, fix.ownAnnouncement)
	require.NoError(t, err)
	assert.Empty(t, ownViews, "a view of another school's announcement must not appear")
}

type announcementView struct {
	ID    int64
	Title string
}

// TestMap_AnswersInConsumerValues pins the mapped reads the owner's
// composition binds, including the target mapping and the foreign-school
// exclusion.
func TestMap_AnswersInConsumerValues(t *testing.T) {
	t.Parallel()
	_, fix := setupAudience(t)
	ctx := context.Background()
	mapped := Map(fix.projection, contract.Rows[announcementView, int64, int]{
		Announcement: func(row contract.Announcement) announcementView {
			return announcementView{ID: row.ID, Title: row.Title}
		},
		Target: func(value announcementView) contract.Target {
			return contract.Target{PublishedAt: &fix.publishedAt, TargetTenantIDs: []int64{fix.foreignID}}
		},
		View:  func(row contract.ViewDetail) int64 { return row.UserID },
		Stats: func(stats contract.Stats) int { return stats.Queries },
	})

	unread, queries, err := mapped.Unread(ctx, fix.readerID, nil, fix.tenantID, fix.organizationID)
	require.NoError(t, err)
	assert.Equal(t, 1, queries)
	assert.Contains(t, unread, announcementView{ID: fix.ownAnnouncement, Title: "Own school"})
	assert.NotContains(t, unread, announcementView{ID: fix.foreignMessage, Title: "Foreign school"})

	targets, _, err := mapped.TargetCount(ctx, announcementView{ID: fix.foreignMessage})
	require.NoError(t, err)
	assert.Equal(t, 1, targets)

	views, _, err := mapped.ViewDetails(ctx, fix.ownAnnouncement)
	require.NoError(t, err)
	assert.NotNil(t, views)
	assert.Empty(t, views)
}

func TestMap_RequiresEveryMapping(t *testing.T) {
	t.Parallel()
	projection := New(func(context.Context) (bun.IDB, error) { return nil, nil })
	assert.Panics(t, func() {
		Map(projection, contract.Rows[contract.Announcement, contract.ViewDetail, contract.Stats]{
			Announcement: func(row contract.Announcement) contract.Announcement { return row },
			View:         func(row contract.ViewDetail) contract.ViewDetail { return row },
			Stats:        func(stats contract.Stats) contract.Stats { return stats },
		})
	})
}
