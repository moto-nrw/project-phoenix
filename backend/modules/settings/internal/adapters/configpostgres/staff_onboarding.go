package config

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

const tableStaffOnboardings = "config.staff_onboardings"

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

// staffOnboardingAcrossSchools is one row of
// config.staff_onboarding_across_schools.
type staffOnboardingAcrossSchools struct {
	DoneSteps    []string   `bun:"done_steps,array"`
	SkippedSteps []string   `bun:"skipped_steps,array"`
	DismissedAt  *time.Time `bun:"dismissed_at"`
}

// staffOnboarding is one person's stored first-steps progress.
type staffOnboarding struct {
	TenantID     int64
	AccountID    int64
	DoneSteps    []string
	SkippedSteps []string
	DismissedAt  *time.Time
}

// StaffOnboardingState is the shape of one person's stored first-steps
// progress. School Setup names its own type of that shape and instantiates
// the repository with it (#4072).
type StaffOnboardingState interface {
	~struct {
		TenantID     int64
		AccountID    int64
		DoneSteps    []string
		SkippedSteps []string
		DismissedAt  *time.Time
	}
}

// StaffOnboardingRepository stores the first steps of care workers. The
// table belongs to Settings Platform, next to the school wizard's tables.
//
// Writes are tenant scoped by the runtime's tenant transaction and the RLS
// policy on the table. The read crosses the person's schools through a function
// that checks the person's access to the school of the transaction (#5659).
type StaffOnboardingRepository[S StaffOnboardingState] struct {
	runtime Runtime
}

// LockStaffOnboarding serializes cross-school read-modify-write updates for
// an account until the ambient tenant transaction ends (#5659).
func (r *StaffOnboardingRepository[S]) LockStaffOnboarding(ctx context.Context, accountID int64) error {
	if accountID <= 0 {
		return errors.New("account ID is required")
	}
	db, err := r.runtime.DB(ctx)
	if err != nil {
		return err
	}
	_, err = db.NewRaw(`SELECT pg_advisory_xact_lock(hashtextextended(?, 0))`, fmt.Sprintf("staff-onboarding:%d", accountID)).Exec(ctx)
	if err != nil {
		return fmt.Errorf("lock staff onboarding: %w", err)
	}
	return nil
}

// NewStaffOnboardingRepository creates a new StaffOnboardingRepository.
func NewStaffOnboardingRepository[S StaffOnboardingState](runtime Runtime) *StaffOnboardingRepository[S] {
	return &StaffOnboardingRepository[S]{runtime: runtime}
}

// StaffOnboardingOf returns the person's state across all their schools, or
// (nil, nil) if the person has not started anywhere (#5659). The returned state
// names no school: the caller stores it in the school of the request.
func (r *StaffOnboardingRepository[S]) StaffOnboardingOf(ctx context.Context, accountID int64) (*S, error) {
	if accountID <= 0 {
		return nil, errors.New("account ID is required")
	}
	db, err := r.runtime.DB(ctx)
	if err != nil {
		return nil, err
	}
	stored := new(staffOnboardingAcrossSchools)
	err = db.NewRaw(
		`SELECT done_steps, skipped_steps, dismissed_at FROM config.staff_onboarding_across_schools(?)`,
		accountID,
	).Scan(ctx, stored)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("find staff onboarding: %w", err)
	}
	state := S(staffOnboarding{
		AccountID:    accountID,
		DoneSteps:    stored.DoneSteps,
		SkippedSteps: stored.SkippedSteps,
		DismissedAt:  stored.DismissedAt,
	})
	return &state, nil
}

// StoreStaffOnboarding replaces the person's state wholesale.
func (r *StaffOnboardingRepository[S]) StoreStaffOnboarding(ctx context.Context, stored *S) error {
	if stored == nil {
		return errors.New("staff onboarding cannot be nil")
	}
	state := staffOnboarding(*stored)
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

	ambientDB2, databaseErr2 := r.runtime.DB(ctx)
	if databaseErr2 != nil {
		return databaseErr2
	}
	_, err := ambientDB2.NewInsert().
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
