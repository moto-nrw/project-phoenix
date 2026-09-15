package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/models/auth"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
)

const (
	errMsgRowsAffected       = "failed to get rows affected: %w"
	errMsgInvitationNotFound = "guardian invitation not found"
)

// GuardianInvitationRepository implements the auth.GuardianInvitationRepository interface
type GuardianInvitationRepository struct {
	updater *base.Repository[*auth.GuardianInvitation]
	db      *bun.DB
}

// NewGuardianInvitationRepository creates a new GuardianInvitationRepository instance
func NewGuardianInvitationRepository(db *bun.DB) auth.GuardianInvitationRepository {
	repo := base.NewRepository[*auth.GuardianInvitation](db, "auth.guardian_invitations", "GuardianInvitation")
	repo.TenantScoped = true
	return &GuardianInvitationRepository{updater: repo, db: db}
}

// Create inserts a new guardian invitation
func (r *GuardianInvitationRepository) Create(ctx context.Context, invitation *auth.GuardianInvitation) error {
	if err := invitation.Validate(); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	base.EnsureTenantID(ctx, invitation)

	_, err := base.GetDB(ctx, r.db).NewInsert().
		Model(invitation).
		ModelTableExpr(`auth.guardian_invitations AS "guardian_invitation"`).
		Exec(ctx)

	if err != nil {
		return fmt.Errorf("failed to create guardian invitation: %w", err)
	}

	return nil
}

// Update updates an existing guardian invitation
func (r *GuardianInvitationRepository) Update(ctx context.Context, invitation *auth.GuardianInvitation) error {
	if err := invitation.Validate(); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	result, err := base.GetDB(ctx, r.db).NewUpdate().
		Model(invitation).
		ModelTableExpr(`auth.guardian_invitations AS "guardian_invitation"`).
		Where(`"guardian_invitation".id = ?`, invitation.ID).
		Exec(ctx)

	if err != nil {
		return fmt.Errorf("failed to update guardian invitation: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf(errMsgRowsAffected, err)
	}

	if rowsAffected == 0 {
		return errors.New(errMsgInvitationNotFound)
	}

	return nil
}

// FindByID retrieves a guardian invitation by ID
func (r *GuardianInvitationRepository) FindByID(ctx context.Context, id int64) (*auth.GuardianInvitation, error) {
	invitation := new(auth.GuardianInvitation)

	err := base.GetDB(ctx, r.db).NewSelect().
		Model(invitation).
		ModelTableExpr(`auth.guardian_invitations AS "guardian_invitation"`).
		Where(`"guardian_invitation".id = ?`, id).
		Scan(ctx)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// Wrap sql.ErrNoRows so service-layer not-found checks
			// (errors.Is / isNotFoundError) can map this to a 404.
			return nil, fmt.Errorf("%s: %w", errMsgInvitationNotFound, sql.ErrNoRows)
		}
		return nil, fmt.Errorf("failed to find guardian invitation: %w", err)
	}

	return invitation, nil
}

// FindByToken retrieves a guardian invitation by token
func (r *GuardianInvitationRepository) FindByToken(ctx context.Context, token string) (*auth.GuardianInvitation, error) {
	invitation := new(auth.GuardianInvitation)

	err := base.GetDB(ctx, r.db).NewSelect().
		Model(invitation).
		ModelTableExpr(`auth.guardian_invitations AS "guardian_invitation"`).
		Where(`"guardian_invitation".token = ?`, token).
		Scan(ctx)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// Wrap sql.ErrNoRows so service-layer not-found checks
			// (errors.Is / isNotFoundError) can map this to a 404.
			return nil, fmt.Errorf("%s: %w", errMsgInvitationNotFound, sql.ErrNoRows)
		}
		return nil, fmt.Errorf("failed to find guardian invitation by token: %w", err)
	}

	return invitation, nil
}

// FindByGuardianProfileID retrieves invitations for a guardian profile
func (r *GuardianInvitationRepository) FindByGuardianProfileID(ctx context.Context, guardianProfileID int64) ([]*auth.GuardianInvitation, error) {
	var invitations []*auth.GuardianInvitation

	err := base.GetDB(ctx, r.db).NewSelect().
		Model(&invitations).
		ModelTableExpr(`auth.guardian_invitations AS "guardian_invitation"`).
		Where(`"guardian_invitation".guardian_profile_id = ?`, guardianProfileID).
		OrderExpr(`"guardian_invitation".created_at DESC`).
		Scan(ctx)

	if err != nil {
		return nil, fmt.Errorf("failed to find invitations by guardian profile ID: %w", err)
	}

	return invitations, nil
}

// FindPending retrieves all consumable pending (not accepted, not expired)
// invitations. Parent-initiated approval-queue rows are excluded: a 'pending'
// row has no usable token until staff approval and never had an email sent, and
// a 'rejected' row must not resurface as a live invite. Only 'not_required'
// (staff invites + parent direct mode) and 'approved' (staff approved; email
// dispatched) statuses back a real, acceptable invitation.
func (r *GuardianInvitationRepository) FindPending(ctx context.Context) ([]*auth.GuardianInvitation, error) {
	var invitations []*auth.GuardianInvitation

	err := base.GetDB(ctx, r.db).NewSelect().
		Model(&invitations).
		ModelTableExpr(`auth.guardian_invitations AS "guardian_invitation"`).
		Where(`"guardian_invitation".accepted_at IS NULL`).
		Where(`"guardian_invitation".expires_at > ?`, time.Now()).
		Where(`"guardian_invitation".approval_status IN (?)`, bun.List([]string{
			auth.GuardianInvitationApprovalNotRequired,
			auth.GuardianInvitationApprovalApproved,
		})).
		OrderExpr(`"guardian_invitation".created_at DESC`).
		Scan(ctx)

	if err != nil {
		return nil, fmt.Errorf("failed to find pending invitations: %w", err)
	}

	return invitations, nil
}

// FindPendingApproval retrieves parent-initiated invitations awaiting staff
// approval (approval_status = 'pending'), newest first. Backs the staff
// approval queue. Expired requests are excluded so stale rows that cleanup has
// not yet removed cannot be approved into a live child link. Tenant isolation
// is enforced by RLS on the ambient tenant transaction.
func (r *GuardianInvitationRepository) FindPendingApproval(ctx context.Context) ([]*auth.GuardianInvitation, error) {
	var invitations []*auth.GuardianInvitation

	err := base.GetDB(ctx, r.db).NewSelect().
		Model(&invitations).
		ModelTableExpr(`auth.guardian_invitations AS "guardian_invitation"`).
		Where(`"guardian_invitation".approval_status = ?`, auth.GuardianInvitationApprovalPending).
		Where(`"guardian_invitation".accepted_at IS NULL`).
		Where(`"guardian_invitation".expires_at > ?`, time.Now()).
		OrderExpr(`"guardian_invitation".created_at DESC`).
		Scan(ctx)

	if err != nil {
		return nil, fmt.Errorf("failed to find invitations pending approval: %w", err)
	}

	return invitations, nil
}

// FindOpenByGuardianProfileIDs retrieves every consumable or approval-pending
// invitation for the requested profiles in one tenant-scoped read.
func (r *GuardianInvitationRepository) FindOpenByGuardianProfileIDs(ctx context.Context, profileIDs []int64) ([]*auth.GuardianInvitation, error) {
	if len(profileIDs) == 0 {
		return []*auth.GuardianInvitation{}, nil
	}
	var invitations []*auth.GuardianInvitation
	err := base.GetDB(ctx, r.db).NewSelect().
		Model(&invitations).
		ModelTableExpr(`auth.guardian_invitations AS "guardian_invitation"`).
		Where(`"guardian_invitation".guardian_profile_id IN (?)`, bun.List(profileIDs)).
		Where(`"guardian_invitation".accepted_at IS NULL`).
		Where(`"guardian_invitation".expires_at > ?`, time.Now()).
		Where(`"guardian_invitation".approval_status IN (?)`, bun.List([]string{
			auth.GuardianInvitationApprovalNotRequired,
			auth.GuardianInvitationApprovalApproved,
			auth.GuardianInvitationApprovalPending,
		})).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to find open invitations by guardian profiles: %w", err)
	}
	return invitations, nil
}

// MarkAsAccepted marks an invitation as accepted
func (r *GuardianInvitationRepository) MarkAsAccepted(ctx context.Context, id int64) error {
	now := time.Now()

	result, err := base.GetDB(ctx, r.db).NewUpdate().
		Model((*auth.GuardianInvitation)(nil)).
		ModelTableExpr(`auth.guardian_invitations AS "guardian_invitation"`).
		Set("accepted_at = ?", now).
		Where(`"guardian_invitation".id = ?`, id).
		Exec(ctx)

	if err != nil {
		return fmt.Errorf("failed to mark invitation as accepted: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf(errMsgRowsAffected, err)
	}

	if rowsAffected == 0 {
		return errors.New(errMsgInvitationNotFound)
	}

	return nil
}

// UpdateEmailStatus updates the email delivery status
func (r *GuardianInvitationRepository) UpdateEmailStatus(ctx context.Context, id int64, sentAt *time.Time, emailError *string, retryCount int) error {
	invitation := &auth.GuardianInvitation{
		Model:           modelBase.Model{ID: id},
		EmailSentAt:     sentAt,
		EmailError:      emailError,
		EmailRetryCount: retryCount,
	}
	updated, err := r.updater.UpdateColumns(ctx, invitation, "email_sent_at", "email_error", "email_retry_count")
	if err != nil {
		if cause, ok := base.RowsAffectedCause(err); ok {
			return fmt.Errorf(errMsgRowsAffected, cause)
		}
		return fmt.Errorf("failed to update email status: %w", base.DatabaseErrorCause(err))
	}
	if updated == 0 {
		return errors.New(errMsgInvitationNotFound)
	}
	return nil
}

// DeleteExpired deletes expired invitations
func (r *GuardianInvitationRepository) DeleteExpired(ctx context.Context) (int, error) {
	result, err := base.GetDB(ctx, r.db).NewDelete().
		Model((*auth.GuardianInvitation)(nil)).
		ModelTableExpr(`auth.guardian_invitations AS "guardian_invitation"`).
		Where(`"guardian_invitation".accepted_at IS NULL`).
		Where(`"guardian_invitation".expires_at <= ?`, time.Now()).
		Exec(ctx)

	if err != nil {
		return 0, fmt.Errorf("failed to delete expired invitations: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf(errMsgRowsAffected, err)
	}

	return int(rowsAffected), nil
}
