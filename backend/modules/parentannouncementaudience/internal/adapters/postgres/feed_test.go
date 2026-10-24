package parentaudience

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/parentannouncementaudience/contract"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func TestCountOutstandingForAccount_ReadParentSurveyStaysOutstandingUntilParticipation(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	var announcementID, surveyID int64
	require.NoError(t, db.NewRaw(`
		INSERT INTO users.parent_announcements
			(tenant_id, title, body, priority, active, created_by, published_at, expires_at, system_kind)
		VALUES (?, 'Umfrage', 'Bitte nehmen Sie teil.', 'info', TRUE, ?, NOW(), NOW() + interval '1 day', 'survey')
		RETURNING id`, chain.TenantID, chain.AccountID).Scan(ctx, &announcementID))
	_, err := db.NewRaw(`
		INSERT INTO users.parent_announcement_targets (tenant_id, announcement_id, target_type)
		VALUES (?, ?, 'school_all')`, chain.TenantID, announcementID).Exec(ctx)
	require.NoError(t, err)
	require.NoError(t, db.NewRaw(`
		INSERT INTO users.surveys
			(tenant_id, title, audience, anonymous, closes_at, published_at, announcement_id)
		VALUES (?, 'Umfrage', 'parents', TRUE, NOW() + interval '1 day', NOW(), ?)
		RETURNING id`, chain.TenantID, announcementID).Scan(ctx, &surveyID))

	projection := New(func(context.Context) (bun.IDB, int64, error) {
		return db, 0, nil
	}, noApplicants{}, testpkg.GuardianLinks{})
	scope := contract.ParentAnnouncementFeedScope{SystemOnlyTenantIDs: []int64{chain.TenantID}}

	count, err := projection.CountOutstandingForAccount(ctx, chain.AccountID, scope)
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	_, err = db.NewRaw(`
		INSERT INTO users.parent_announcement_reads (tenant_id, announcement_id, account_id)
		VALUES (?, ?, ?)`, chain.TenantID, announcementID, chain.AccountID).Exec(ctx)
	require.NoError(t, err)
	count, err = projection.CountOutstandingForAccount(ctx, chain.AccountID, scope)
	require.NoError(t, err)
	assert.Equal(t, 1, count, "reading a survey does not complete it")

	_, err = db.NewRaw(`
		INSERT INTO users.survey_participations (tenant_id, survey_id, account_id)
		VALUES (?, ?, ?)`, chain.TenantID, surveyID, chain.AccountID).Exec(ctx)
	require.NoError(t, err)
	count, err = projection.CountOutstandingForAccount(ctx, chain.AccountID, scope)
	require.NoError(t, err)
	assert.Zero(t, count, "participating completes the survey")
}
