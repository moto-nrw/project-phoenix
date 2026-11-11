package postgres

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// The discard of an empty open run (#6626) names its Träger in its own
// statements below row level security: the Träger transaction drops its
// request role, so another Träger's open run stays open only because the
// adapter filters by the Träger. Removing the filter turns it red.
func TestRunDiscardsKeepTheTraegerBelowRLS(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	own := newCareScope(t, db, "Träger Verwerfen A")
	other := newCareScope(t, db, "Träger Verwerfen B")
	store := NewStore()
	rollback := errors.New("roll the discard rows back")

	ctx := testpkg.ContextWithTenantRuntime(context.Background(), testpkg.TenantRuntime(t, db))
	err := testpkg.WithinTraeger(ctx, own.organization, func(ctx context.Context) error {
		require.NoError(t, testpkg.WithoutRequestRole(ctx))
		tx, err := traegerDB(ctx, own.organization)
		require.NoError(t, err)
		own = seedCareScope(t, ctx, tx, own, "Träger Verwerfen A")
		other = seedCareScope(t, ctx, tx, other, "Träger Verwerfen B")
		seedCareReviewScope(t, ctx, tx, own)
		seedCareReviewScope(t, ctx, tx, other)
		ownNotices := seedNoticeScope(t, ctx, tx, own, 1000)
		otherNotices := seedNoticeScope(t, ctx, tx, other, 2000)

		_, err = store.MarkCareRunDiscarded(ctx, own.organization, other.open)
		assert.ErrorIs(t, err, parentbilling.ErrBillingRunReleased, "MarkCareRunDiscarded of another Träger's run")
		_, err = store.MarkCareNoticeRunDiscarded(ctx, own.organization, otherNotices.openRun)
		assert.ErrorIs(t, err, parentbilling.ErrBillingRunReleased, "MarkCareNoticeRunDiscarded of another Träger's run")
		var status string
		var open bool
		require.NoError(t, tx.NewRaw(`SELECT run.status, school.open FROM parent_billing.care_runs AS run
			JOIN parent_billing.care_run_schools AS school ON school.run_id = run.id WHERE run.id = ?`, other.open).
			Scan(ctx, &status, &open))
		assert.Equal(t, parentbilling.RunCalculated, status, "the other Träger's monthly run stays open")
		assert.True(t, open, "and keeps its school")
		require.NoError(t, tx.NewRaw(`SELECT status FROM parent_billing.care_notice_runs WHERE id = ?`, otherNotices.openRun).
			Scan(ctx, &status))
		assert.Equal(t, parentbilling.RunCalculated, status, "the other Träger's notice run stays open")

		run, err := store.MarkCareRunDiscarded(ctx, own.organization, own.open)
		require.NoError(t, err)
		assert.Equal(t, parentbilling.RunDiscarded, run.Status)
		assert.NotNil(t, run.DiscardedAt)
		assert.Equal(t, []int64{own.school}, run.SchoolIDs, "the schools it closed")
		openSchools, err := store.OpenCareRunSchools(ctx, own.organization, calendar.NewDate(2029, 10, 1))
		require.NoError(t, err)
		assert.Empty(t, openSchools, "the month is free for a new run")
		notice, err := store.MarkCareNoticeRunDiscarded(ctx, own.organization, ownNotices.openRun)
		require.NoError(t, err)
		assert.Equal(t, parentbilling.RunDiscarded, notice.Status)
		assert.NotNil(t, notice.DiscardedAt)

		_, err = store.MarkCareRunDiscarded(ctx, own.organization, own.released)
		assert.ErrorIs(t, err, parentbilling.ErrBillingRunReleased, "a released run is never discarded")
		_, err = store.MarkCareNoticeRunDiscarded(ctx, own.organization, ownNotices.noticeRun)
		assert.ErrorIs(t, err, parentbilling.ErrBillingRunReleased, "a released notice run is never discarded")
		return rollback
	})
	require.ErrorIs(t, err, rollback)
}
