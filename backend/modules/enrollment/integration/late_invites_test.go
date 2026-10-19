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
	require.NoError(t, testpkg.TenantCommand1(t, ctx, db, owner.InsertRequest, request))
	now := time.Now().UTC()
	invite := &enrollment.LateInvite{PhaseID: phase.ID, TokenHash: "late-invite-owner", GuardianEmail: request.GuardianEmail, ExpiresAt: now.Add(time.Hour), CreatedBy: creator.ID}
	require.NoError(t, testpkg.TenantCommand1(t, ctx, db, owner.InsertLateInvite, invite))
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
	restored, err := testpkg.TenantQuery4(t, ctx, db, owner.UsableLateInvite, invite.TokenHash, phase.ID, now, false)
	require.NoError(t, err)
	require.Nil(t, restored.UsedAt)
	_, err = testpkg.TenantQuery4(t, ctx, db, owner.UsableLateInvite, invite.TokenHash, phase.ID, invite.ExpiresAt, false)
	require.ErrorIs(t, err, enrollment.ErrLateInviteNotFound)
	require.NoError(t, testpkg.TenantCommand3(t, ctx, db, owner.MarkLateInviteUsed, invite.ID, request.ID, now))
	require.EqualError(t, testpkg.TenantCommand3(t, ctx, db, owner.MarkLateInviteUsed, invite.ID, request.ID, now), "late invite was already used")
	_, err = testpkg.TenantQuery4(t, ctx, db, owner.UsableLateInvite, invite.TokenHash, phase.ID, now, false)
	require.ErrorIs(t, err, enrollment.ErrLateInviteNotFound)
	consumed, err := testpkg.TenantQuery1(t, ctx, db, owner.LateInviteByUsedRequestID, request.ID)
	require.NoError(t, err)
	require.Equal(t, invite.ID, consumed.ID)
	t.Run("foreign tenant", func(t *testing.T) {
		testpkg.OwnTenant(t)
		foreignCtx := testpkg.Ctx(t)
		foreignPhase := testpkg.CreateTestEnrollmentPhase(t, db)
		foreignCreator := testpkg.CreateTestAccount(t, db, "foreign-invite-owner@example.test")
		crossTenant := &enrollment.LateInvite{PhaseID: foreignPhase.ID, TokenHash: "cross-tenant-used-request", GuardianEmail: "foreign@example.test", ExpiresAt: now.Add(time.Hour), CreatedBy: foreignCreator.ID, UsedRequestID: &request.ID, UsedAt: &now}
		require.ErrorIs(t, testpkg.TenantCommand1(t, foreignCtx, db, owner.InsertLateInvite, crossTenant), sql.ErrNoRows)

		_, err := testpkg.TenantQuery1(t, foreignCtx, db, owner.LateInviteByUsedRequestID, request.ID)
		require.ErrorIs(t, err, enrollment.ErrLateInviteNotFound)
		deleted, err := testpkg.TenantQuery1(t, foreignCtx, db, owner.DeleteLateInvitesByUsedRequestID, request.ID)
		require.NoError(t, err)
		require.Zero(t, deleted)
	})
	deleted, err := testpkg.TenantQuery1(t, ctx, db, owner.DeleteLateInvitesByUsedRequestID, request.ID)
	require.NoError(t, err)
	require.EqualValues(t, 1, deleted)
	_, err = testpkg.TenantQuery1(t, ctx, db, owner.LateInviteByUsedRequestID, request.ID)
	require.ErrorIs(t, err, enrollment.ErrLateInviteNotFound)
}
