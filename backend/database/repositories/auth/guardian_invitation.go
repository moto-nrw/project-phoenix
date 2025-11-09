package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/models/auth"
	"github.com/uptrace/bun"
)

// GuardianInvitationRepository implements the auth.GuardianInvitationRepository interface
type GuardianInvitationRepository struct {
	db *bun.DB
}

// NewGuardianInvitationRepository creates a new GuardianInvitationRepository instance
func NewGuardianInvitationRepository(db *bun.DB) auth.GuardianInvitationRepository {
	return &GuardianInvitationRepository{db: db}
}

// WithTx returns a new repository with the given transaction
func (r *GuardianInvitationRepository) WithTx(tx bun.Tx) interface{} {
	return &GuardianInvitationRepository{db: tx.(*bun.DB)}
}

// Create inserts a new guardian invitation
func (r *GuardianInvitationRepository) Create(ctx context.Context, invitation *auth.GuardianInvitation) error {
	if err := invitation.Validate(); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	_, err := r.db.NewInsert().
		Model(invitation).
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

	result, err := r.db.NewUpdate().
		Model(invitation).
		WherePK().
		Exec(ctx)

	if err != nil {
		return fmt.Errorf("failed to update guardian invitation: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("guardian invitation not found")
	}

	return nil
}

// FindByID retrieves a guardian invitation by ID
func (r *GuardianInvitationRepository) FindByID(ctx context.Context, id int64) (*auth.GuardianInvitation, error) {
	invitation := new(auth.GuardianInvitation)

	err := r.db.NewSelect().
		Model(invitation).
		Where("id = ?", id).
		Scan(ctx)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("guardian invitation not found")
		}
		return nil, fmt.Errorf("failed to find guardian invitation: %w", err)
	}

	return invitation, nil
}

// FindByToken retrieves a guardian invitation by token
func (r *GuardianInvitationRepository) FindByToken(ctx context.Context, token string) (*auth.GuardianInvitation, error) {
	invitation := new(auth.GuardianInvitation)

	err := r.db.NewSelect().
		Model(invitation).
		Where("token = ?", token).
		Scan(ctx)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("guardian invitation not found")
		}
		return nil, fmt.Errorf("failed to find guardian invitation by token: %w", err)
	}

	return invitation, nil
}

// FindByGuardianProfileID retrieves invitations for a guardian profile
func (r *GuardianInvitationRepository) FindByGuardianProfileID(ctx context.Context, guardianProfileID int64) ([]*auth.GuardianInvitation, error) {
	var invitations []*auth.GuardianInvitation

	err := r.db.NewSelect().
		Model(&invitations).
		Where("guardian_profile_id = ?", guardianProfileID).
		Order("created_at DESC").
		Scan(ctx)

	if err != nil {
		return nil, fmt.Errorf("failed to find invitations by guardian profile ID: %w", err)
	}

	return invitations, nil
}

// FindPending retrieves all pending (not accepted, not expired) invitations
func (r *GuardianInvitationRepository) FindPending(ctx context.Context) ([]*auth.GuardianInvitation, error) {
	var invitations []*auth.GuardianInvitation

	err := r.db.NewSelect().
		Model(&invitations).
		Where("accepted_at IS NULL").
		Where("expires_at > ?", time.Now()).
		Order("created_at DESC").
		Scan(ctx)

	if err != nil {
		return nil, fmt.Errorf("failed to find pending invitations: %w", err)
	}

	return invitations, nil
}

// FindExpired retrieves all expired invitations
func (r *GuardianInvitationRepository) FindExpired(ctx context.Context) ([]*auth.GuardianInvitation, error) {
	var invitations []*auth.GuardianInvitation

	err := r.db.NewSelect().
		Model(&invitations).
		Where("accepted_at IS NULL").
		Where("expires_at <= ?", time.Now()).
		Order("expires_at DESC").
		Scan(ctx)

	if err != nil {
		return nil, fmt.Errorf("failed to find expired invitations: %w", err)
	}

	return invitations, nil
}

// MarkAsAccepted marks an invitation as accepted
func (r *GuardianInvitationRepository) MarkAsAccepted(ctx context.Context, id int64) error {
	now := time.Now()

	result, err := r.db.NewUpdate().
		Model((*auth.GuardianInvitation)(nil)).
		Set("accepted_at = ?", now).
		Where("id = ?", id).
		Exec(ctx)

	if err != nil {
		return fmt.Errorf("failed to mark invitation as accepted: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("guardian invitation not found")
	}

	return nil
}

// UpdateEmailStatus updates the email delivery status
func (r *GuardianInvitationRepository) UpdateEmailStatus(ctx context.Context, id int64, sentAt *time.Time, emailError *string, retryCount int) error {
	result, err := r.db.NewUpdate().
		Model((*auth.GuardianInvitation)(nil)).
		Set("email_sent_at = ?", sentAt).
		Set("email_error = ?", emailError).
		Set("email_retry_count = ?", retryCount).
		Where("id = ?", id).
		Exec(ctx)

	if err != nil {
		return fmt.Errorf("failed to update email status: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("guardian invitation not found")
	}

	return nil
}

// DeleteExpired deletes expired invitations
func (r *GuardianInvitationRepository) DeleteExpired(ctx context.Context) (int, error) {
	result, err := r.db.NewDelete().
		Model((*auth.GuardianInvitation)(nil)).
		Where("accepted_at IS NULL").
		Where("expires_at <= ?", time.Now()).
		Exec(ctx)

	if err != nil {
		return 0, fmt.Errorf("failed to delete expired invitations: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to get rows affected: %w", err)
	}

	return int(rowsAffected), nil
}

// Count returns the total number of guardian invitations
func (r *GuardianInvitationRepository) Count(ctx context.Context) (int, error) {
	count, err := r.db.NewSelect().
		Model((*auth.GuardianInvitation)(nil)).
		Count(ctx)

	if err != nil {
		return 0, fmt.Errorf("failed to count guardian invitations: %w", err)
	}

	return count, nil
}
