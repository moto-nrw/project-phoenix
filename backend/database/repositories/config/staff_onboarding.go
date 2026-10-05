package config

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/schoolsetup"
)

const (
	tableStaffOnboardings = "config.staff_onboardings"
	// The alias must match the row struct's name: bun qualifies the selected
	// columns with the model alias it derives from it.
	tableStaffOnboardingsAlias = `config.staff_onboardings AS "staff_onboarding_row"`
)

// staffOnboardingRow is one config.staff_onboardings row (#3748).
type staffOnboardingRow struct {
	ID           int64      `bun:"id,pk,autoincrement"`
	TenantID     int64      `bun:"tenant_id,notnull"`
	AccountID    int64      `bun:"account_id,notnull"`
	DoneSteps    []string   `bun:"done_steps,array,notnull"`
	SkippedSteps []string   `bun:"skipped_steps,array,notnull"`
	DismissedAt  *time.Time `bun:"dismissed_at"`
	CreatedAt    time.Time  `bun:"created_at,notnull,default:now()"`
	UpdatedAt    time.Time  `bun:"updated_at,notnull,default:now()"`
}

// StaffOnboardingRepository implements schoolsetup.StaffStore. The table
// belongs to Settings Platform, next to the school wizard's tables.
//
// Tenant scoping comes from the runtime's tenant transaction and the RLS
// policy on the table; the account is the only key this package supplies
// itself.
type StaffOnboardingRepository struct {
	runtime Runtime
}

// NewStaffOnboardingRepository creates a new StaffOnboardingRepository.
func NewStaffOnboardingRepository(runtime Runtime) schoolsetup.StaffStore {
	return &StaffOnboardingRepository{runtime: runtime}
}

// StaffOnboardingOf returns the person's state, or (nil, nil) if the person
// has not started.
func (r *StaffOnboardingRepository) StaffOnboardingOf(ctx context.Context, accountID int64) (*schoolsetup.StaffState, error) {
	if accountID <= 0 {
		return nil, errors.New("account ID is required")
	}
	stored := new(staffOnboardingRow)
	err := r.runtime.DB(ctx).NewSelect().
		Model(stored).
		ModelTableExpr(tableStaffOnboardingsAlias).
		Where(`"staff_onboarding_row".account_id = ?`, accountID).
		Limit(1).
		Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("find staff onboarding: %w", err)
	}
	return &schoolsetup.StaffState{
		TenantID:     stored.TenantID,
		AccountID:    stored.AccountID,
		DoneSteps:    stored.DoneSteps,
		SkippedSteps: stored.SkippedSteps,
		DismissedAt:  stored.DismissedAt,
	}, nil
}

// StoreStaffOnboarding replaces the person's state wholesale.
func (r *StaffOnboardingRepository) StoreStaffOnboarding(ctx context.Context, state *schoolsetup.StaffState) error {
	if state == nil {
		return errors.New("staff onboarding cannot be nil")
	}
	if state.TenantID <= 0 || state.AccountID <= 0 {
		return errors.New("tenant ID and account ID are required")
	}
	done := state.DoneSteps
	if done == nil {
		done = []string{}
	}
	skipped := state.SkippedSteps
	if skipped == nil {
		skipped = []string{}
	}

	_, err := r.runtime.DB(ctx).NewInsert().
		Model(&staffOnboardingRow{
			TenantID:     state.TenantID,
			AccountID:    state.AccountID,
			DoneSteps:    done,
			SkippedSteps: skipped,
			DismissedAt:  state.DismissedAt,
		}).
		ModelTableExpr(tableStaffOnboardings).
		On("CONFLICT (tenant_id, account_id) DO UPDATE").
		Set("done_steps = EXCLUDED.done_steps").
		Set("skipped_steps = EXCLUDED.skipped_steps").
		Set("dismissed_at = EXCLUDED.dismissed_at").
		Set("updated_at = NOW()").
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("upsert staff onboarding: %w", err)
	}
	return nil
}
