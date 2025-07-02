package active

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/models/active"
	"github.com/uptrace/bun"
)

// ScheduledCheckoutRepository implements the repository for scheduled checkouts
type ScheduledCheckoutRepository struct {
	db *bun.DB
}

// NewScheduledCheckoutRepository creates a new scheduled checkout repository
func NewScheduledCheckoutRepository(db *bun.DB) active.ScheduledCheckoutRepository {
	return &ScheduledCheckoutRepository{db: db}
}

// Create creates a new scheduled checkout
func (r *ScheduledCheckoutRepository) Create(ctx context.Context, checkout *active.ScheduledCheckout) error {
	// Set timestamps if not already set
	now := time.Now()
	if checkout.CreatedAt.IsZero() {
		checkout.CreatedAt = now
	}
	if checkout.UpdatedAt.IsZero() {
		checkout.UpdatedAt = now
	}
	
	_, err := r.db.NewInsert().
		Model(checkout).
		ModelTableExpr(`active.scheduled_checkouts`).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to create scheduled checkout: %w", err)
	}
	return nil
}

// GetByID retrieves a scheduled checkout by ID
func (r *ScheduledCheckoutRepository) GetByID(ctx context.Context, id int64) (*active.ScheduledCheckout, error) {
	var checkout active.ScheduledCheckout
	err := r.db.NewSelect().
		Model(&checkout).
		ModelTableExpr(`active.scheduled_checkouts AS "scheduled_checkout"`).
		Where(`"scheduled_checkout".id = ?`, id).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get scheduled checkout by ID: %w", err)
	}
	return &checkout, nil
}

// GetPendingByStudentID gets the pending scheduled checkout for a student
func (r *ScheduledCheckoutRepository) GetPendingByStudentID(ctx context.Context, studentID int64) (*active.ScheduledCheckout, error) {
	var checkout active.ScheduledCheckout
	err := r.db.NewSelect().
		Model(&checkout).
		ModelTableExpr(`active.scheduled_checkouts AS "scheduled_checkout"`).
		Where(`"scheduled_checkout".student_id = ?`, studentID).
		Where(`"scheduled_checkout".status = ?`, active.ScheduledCheckoutStatusPending).
		Scan(ctx)
	if err != nil {
		if err.Error() == "sql: no rows in result set" {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get pending scheduled checkout: %w", err)
	}
	return &checkout, nil
}

// GetDueCheckouts retrieves all pending checkouts scheduled for before the given time
func (r *ScheduledCheckoutRepository) GetDueCheckouts(ctx context.Context, beforeTime time.Time) ([]*active.ScheduledCheckout, error) {
	var checkouts []*active.ScheduledCheckout
	err := r.db.NewSelect().
		Model(&checkouts).
		Where(`status = ?`, active.ScheduledCheckoutStatusPending).
		Where(`scheduled_for <= ?`, beforeTime).
		Order(`scheduled_for ASC`).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get due scheduled checkouts: %w", err)
	}
	return checkouts, nil
}

// Update updates a scheduled checkout
func (r *ScheduledCheckoutRepository) Update(ctx context.Context, checkout *active.ScheduledCheckout) error {
	checkout.UpdatedAt = time.Now()
	_, err := r.db.NewUpdate().
		Model(checkout).
		ModelTableExpr(`active.scheduled_checkouts AS "scheduled_checkout"`).
		WherePK().
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to update scheduled checkout: %w", err)
	}
	return nil
}

// Delete deletes a scheduled checkout
func (r *ScheduledCheckoutRepository) Delete(ctx context.Context, id int64) error {
	_, err := r.db.NewDelete().
		Model((*active.ScheduledCheckout)(nil)).
		ModelTableExpr(`active.scheduled_checkouts AS "scheduled_checkout"`).
		Where(`"scheduled_checkout".id = ?`, id).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to delete scheduled checkout: %w", err)
	}
	return nil
}

// ListByStudentID lists all scheduled checkouts for a student
func (r *ScheduledCheckoutRepository) ListByStudentID(ctx context.Context, studentID int64) ([]*active.ScheduledCheckout, error) {
	var checkouts []*active.ScheduledCheckout
	err := r.db.NewSelect().
		Model(&checkouts).
		ModelTableExpr(`active.scheduled_checkouts AS "scheduled_checkout"`).
		Where(`"scheduled_checkout".student_id = ?`, studentID).
		Order(`created_at DESC`).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list scheduled checkouts: %w", err)
	}
	return checkouts, nil
}

// CancelPendingByStudentID cancels any pending scheduled checkouts for a student
func (r *ScheduledCheckoutRepository) CancelPendingByStudentID(ctx context.Context, studentID int64, cancelledBy int64) error {
	now := time.Now()
	_, err := r.db.NewUpdate().
		Model((*active.ScheduledCheckout)(nil)).
		ModelTableExpr(`active.scheduled_checkouts AS "scheduled_checkout"`).
		Set(`status = ?`, active.ScheduledCheckoutStatusCancelled).
		Set(`cancelled_at = ?`, now).
		Set(`cancelled_by = ?`, cancelledBy).
		Set(`updated_at = ?`, now).
		Where(`"scheduled_checkout".student_id = ?`, studentID).
		Where(`"scheduled_checkout".status = ?`, active.ScheduledCheckoutStatusPending).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to cancel pending scheduled checkouts: %w", err)
	}
	return nil
}