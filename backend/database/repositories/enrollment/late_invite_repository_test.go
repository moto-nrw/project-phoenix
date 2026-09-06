package enrollment_test

import (
	"context"
	"testing"
	"time"

	inviteOwner "github.com/moto-nrw/project-phoenix/modules/enrollment"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLateInviteOwner_DeleteByUsedRequestID_DeletesOnlyLinkedInvite(t *testing.T) {
	t.Parallel()

	// setupRequestRepoTest owns the package's testpkg.SetupTestDB lifecycle and
	// also creates the real tenant + phase fixtures this repository needs.
	db, requestRepo, tenantID, phaseID := setupRequestRepoTest(t)
	lateInviteRepo := requestRepo
	tokenPrefix := uniqueToken("deleteUsedLateInvite")
	creator := testpkg.CreateTestAccount(t, db, "late-invite-delete")
	t.Cleanup(func() {
		_, _ = db.NewDelete().
			TableExpr("enrollment.late_invites").
			Where("tenant_id = ? AND token_hash LIKE ?", tenantID, tokenPrefix+"%").
			Exec(context.Background())
		wipeRequests(db, tenantID, tokenPrefix)
	})

	requestA := makeOwnerRequest(phaseID, tokenPrefix+"-request-a", "a@example.test")
	requestB := makeOwnerRequest(phaseID, tokenPrefix+"-request-b", "b@example.test")
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		if err := requestRepo.InsertRequest(ctx, requestA); err != nil {
			return err
		}
		return requestRepo.InsertRequest(ctx, requestB)
	}))

	inviteA := &inviteOwner.LateInvite{
		PhaseID:       phaseID,
		TokenHash:     tokenPrefix + "-invite-a",
		GuardianEmail: "a@example.test",
		ExpiresAt:     time.Now().Add(24 * time.Hour),
		CreatedBy:     creator.ID,
	}
	inviteB := &inviteOwner.LateInvite{
		PhaseID:       phaseID,
		TokenHash:     tokenPrefix + "-invite-b",
		GuardianEmail: "b@example.test",
		ExpiresAt:     time.Now().Add(24 * time.Hour),
		CreatedBy:     creator.ID,
	}
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		if err := lateInviteRepo.InsertLateInvite(ctx, inviteA); err != nil {
			return err
		}
		if err := lateInviteRepo.InsertLateInvite(ctx, inviteB); err != nil {
			return err
		}
		if err := lateInviteRepo.MarkLateInviteUsed(ctx, inviteA.ID, requestA.ID, time.Now()); err != nil {
			return err
		}
		return lateInviteRepo.MarkLateInviteUsed(ctx, inviteB.ID, requestB.ID, time.Now())
	}))

	var linked *inviteOwner.LateInvite
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		var err error
		linked, err = lateInviteRepo.LateInviteByUsedRequestID(ctx, requestA.ID)
		return err
	}))
	require.NotNil(t, linked)
	assert.Equal(t, inviteA.ID, linked.ID)
	assert.Equal(t, "a@example.test", linked.GuardianEmail)

	var deleted int64
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		var err error
		deleted, err = lateInviteRepo.DeleteLateInvitesByUsedRequestID(ctx, requestA.ID)
		return err
	}))
	assert.EqualValues(t, 1, deleted)

	var remainingA, remainingB int
	require.NoError(t, db.NewRaw(`SELECT COUNT(*) FROM enrollment.late_invites WHERE id = ?`, inviteA.ID).Scan(context.Background(), &remainingA))
	require.NoError(t, db.NewRaw(`SELECT COUNT(*) FROM enrollment.late_invites WHERE id = ?`, inviteB.ID).Scan(context.Background(), &remainingB))
	assert.Zero(t, remainingA)
	assert.Equal(t, 1, remainingB)
}
