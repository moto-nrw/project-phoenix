package integration

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func TestLateInviteOwnerConsumptionRollsBackAndRejectsReuse(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	owner := enrollmentCompose.New()
	phase := testpkg.CreateTestEnrollmentPhase(t, db)
	creator := testpkg.CreateTestAccount(t, db, "invite-owner@example.test")
	request := &enrollment.Request{PhaseID: phase.ID, GuardianFirstName: "Fixture", GuardianLastName: "Guardian", GuardianEmail: "invite@example.test", StatusToken: "late-invite-owner"}
	require.NoError(t, owner.InsertRequest(ctx, request))
	now := time.Now().UTC()
	invite := &enrollment.LateInvite{PhaseID: phase.ID, TokenHash: "late-invite-owner", GuardianEmail: request.GuardianEmail, ExpiresAt: now.Add(time.Hour), CreatedBy: creator.ID}
	require.NoError(t, owner.InsertLateInvite(ctx, invite))
	require.NotZero(t, invite.ID)
	require.Equal(t, testpkg.Tenant(t), invite.TenantID)
	require.False(t, invite.CreatedAt.IsZero())
	failure := errors.New("injected after invite consumption")
	err := testpkg.WithTenantTx(t, ctx, db, testpkg.Tenant(t), func(txCtx context.Context, _ bun.Tx) error {
		locked, err := owner.UsableLateInvite(txCtx, invite.TokenHash, phase.ID, now, true)
		if err != nil {
			return err
		}
		require.Equal(t, invite.ID, locked.ID)
		if err := owner.MarkLateInviteUsed(txCtx, invite.ID, request.ID, now); err != nil {
			return err
		}
		return failure
	})
	require.ErrorIs(t, err, failure)
	restored, err := owner.UsableLateInvite(ctx, invite.TokenHash, phase.ID, now, false)
	require.NoError(t, err)
	require.Nil(t, restored.UsedAt)
	_, err = owner.UsableLateInvite(ctx, invite.TokenHash, phase.ID, invite.ExpiresAt, false)
	require.ErrorIs(t, err, enrollment.ErrLateInviteNotFound)
	require.NoError(t, owner.MarkLateInviteUsed(ctx, invite.ID, request.ID, now))
	require.EqualError(t, owner.MarkLateInviteUsed(ctx, invite.ID, request.ID, now), "late invite was already used")
	_, err = owner.UsableLateInvite(ctx, invite.TokenHash, phase.ID, now, false)
	require.ErrorIs(t, err, enrollment.ErrLateInviteNotFound)
	consumed, err := owner.LateInviteByUsedRequestID(ctx, request.ID)
	require.NoError(t, err)
	require.Equal(t, invite.ID, consumed.ID)
	t.Run("foreign tenant", func(t *testing.T) {
		testpkg.OwnTenant(t)
		foreignCtx := testpkg.Ctx(t)
		foreignPhase := testpkg.CreateTestEnrollmentPhase(t, db)
		foreignCreator := testpkg.CreateTestAccount(t, db, "foreign-invite-owner@example.test")
		crossTenant := &enrollment.LateInvite{PhaseID: foreignPhase.ID, TokenHash: "cross-tenant-used-request", GuardianEmail: "foreign@example.test", ExpiresAt: now.Add(time.Hour), CreatedBy: foreignCreator.ID, UsedRequestID: &request.ID, UsedAt: &now}
		require.ErrorIs(t, owner.InsertLateInvite(foreignCtx, crossTenant), sql.ErrNoRows)

		_, err := owner.LateInviteByUsedRequestID(foreignCtx, request.ID)
		require.ErrorIs(t, err, enrollment.ErrLateInviteNotFound)
		deleted, err := owner.DeleteLateInvitesByUsedRequestID(foreignCtx, request.ID)
		require.NoError(t, err)
		require.Zero(t, deleted)
	})
	deleted, err := owner.DeleteLateInvitesByUsedRequestID(ctx, request.ID)
	require.NoError(t, err)
	require.EqualValues(t, 1, deleted)
	_, err = owner.LateInviteByUsedRequestID(ctx, request.ID)
	require.ErrorIs(t, err, enrollment.ErrLateInviteNotFound)
}
