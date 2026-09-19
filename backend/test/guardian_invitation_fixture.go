package test

import (
	"context"
	"testing"
	"time"

	authModels "github.com/moto-nrw/project-phoenix/modules/identityaccess/legacy/authmodels"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// auth.guardian_invitations belongs to Identity & Access (#2722), which
// serves it through its own capability. Tests that need to plant a row or
// read one back use the fixtures here instead of a repository.

// InsertTestGuardianInvitation writes the invitation and fills in the
// identifiers the database assigns. Every field the caller left unset keeps
// the table's default.
func InsertTestGuardianInvitation(t *testing.T, db *bun.DB, invitation *authModels.GuardianInvitation) *authModels.GuardianInvitation {
	t.Helper()
	require.NotNil(t, invitation)
	if invitation.TenantID == 0 {
		invitation.SetTenantID(Tenant(t))
	}
	if invitation.ExpiresAt.IsZero() {
		invitation.ExpiresAt = time.Now().Add(48 * time.Hour)
	}
	if invitation.ApprovalStatus == "" {
		invitation.ApprovalStatus = authModels.GuardianInvitationApprovalNotRequired
	}
	_, err := db.NewInsert().
		Model(invitation).
		ModelTableExpr(`auth.guardian_invitations AS "guardian_invitation"`).
		Returning("*").
		Exec(context.Background())
	require.NoError(t, err, "insert guardian invitation fixture")
	t.Cleanup(func() {
		_, _ = db.NewDelete().
			TableExpr("auth.guardian_invitations").
			Where("id = ?", invitation.ID).
			Exec(context.Background())
	})
	return invitation
}

// GuardianInvitationByID reads the stored row so a test can assert what a
// flow wrote. It reads administratively, so a row of any school is visible.
func GuardianInvitationByID(t *testing.T, db *bun.DB, id int64) *authModels.GuardianInvitation {
	t.Helper()
	invitation := new(authModels.GuardianInvitation)
	err := db.NewSelect().
		Model(invitation).
		ModelTableExpr(`auth.guardian_invitations AS "guardian_invitation"`).
		Where(`"guardian_invitation".id = ?`, id).
		Scan(context.Background())
	require.NoError(t, err, "read guardian invitation %d", id)
	return invitation
}

// GuardianInvitationsByProfile reads the contact's invitations, newest first.
func GuardianInvitationsByProfile(t *testing.T, db *bun.DB, guardianProfileID int64) []*authModels.GuardianInvitation {
	t.Helper()
	var invitations []*authModels.GuardianInvitation
	err := db.NewSelect().
		Model(&invitations).
		ModelTableExpr(`auth.guardian_invitations AS "guardian_invitation"`).
		Where(`"guardian_invitation".guardian_profile_id = ?`, guardianProfileID).
		OrderExpr(`"guardian_invitation".created_at DESC`).
		Scan(context.Background())
	require.NoError(t, err, "read guardian invitations of profile %d", guardianProfileID)
	return invitations
}
