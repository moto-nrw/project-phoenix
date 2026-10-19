package config

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

const (
	tableSchoolSetups = "config.school_setups"
	// The alias must match the row struct's name: bun qualifies the selected
	// columns with the model alias it derives from it.
	tableSchoolSetupsAlias     = `config.school_setups AS "school_setup_row"`
	tableSchoolSetupDismissals = "config.school_setup_dismissals"
)

// schoolSetupRow is one config.school_setups row (#2832, ADR 0043).
type schoolSetupRow struct {
	ID           int64      `bun:"id,pk,autoincrement"`
	TenantID     int64      `bun:"tenant_id,notnull"`
	SkippedSteps []string   `bun:"skipped_steps,array,notnull"`
	CompletedAt  *time.Time `bun:"completed_at"`
	UpdatedBy    *int64     `bun:"updated_by"`
	CreatedAt    time.Time  `bun:"created_at,notnull,default:now()"`
	UpdatedAt    time.Time  `bun:"updated_at,notnull,default:now()"`
}

// schoolSetup is one school's stored wizard state.
type schoolSetup struct {
	TenantID     int64
	SkippedSteps []string
	CompletedAt  *time.Time
	UpdatedBy    *int64
}

// SchoolSetupState is the shape of one school's stored wizard state. The
// wizard names its own type of that shape and instantiates the repository
// with it, so Settings Platform stores the state without importing School
// Setup (#4072).
type SchoolSetupState interface {
	~struct {
		TenantID     int64
		SkippedSteps []string
		CompletedAt  *time.Time
		UpdatedBy    *int64
	}
}

// SchoolSetupRepository stores the school setup wizard's state. Both tables
// belong to Settings Platform, which is why the wizard's storage lives here.
//
// Tenant scoping comes from the runtime's tenant transaction and the RLS
// policy on both tables; the account is the only key this package supplies
// itself.
type SchoolSetupRepository[S SchoolSetupState] struct {
	runtime Runtime
}

// NewSchoolSetupRepository creates a new SchoolSetupRepository.
func NewSchoolSetupRepository[S SchoolSetupState](runtime Runtime) *SchoolSetupRepository[S] {
	return &SchoolSetupRepository[S]{runtime: runtime}
}

// Find returns the school's wizard state, or (nil, nil) for a new school.
func (r *SchoolSetupRepository[S]) SetupOfSchool(ctx context.Context) (*S, error) {
	stored := new(schoolSetupRow)
	ambientDB1, databaseErr1 := r.runtime.DB(ctx)
	if databaseErr1 != nil {
		return *new(*S), databaseErr1
	}
	err := ambientDB1.NewSelect().
		Model(stored).
		ModelTableExpr(tableSchoolSetupsAlias).
		Limit(1).
		Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("find school setup: %w", err)
	}
	state := S(schoolSetup{
		TenantID:     stored.TenantID,
		SkippedSteps: stored.SkippedSteps,
		CompletedAt:  stored.CompletedAt,
		UpdatedBy:    stored.UpdatedBy,
	})
	return &state, nil
}

// Upsert replaces the school's wizard state wholesale.
func (r *SchoolSetupRepository[S]) StoreSetup(ctx context.Context, stored *S) error {
	if stored == nil {
		return errors.New("school setup cannot be nil")
	}
	state := schoolSetup(*stored)
	if state.TenantID <= 0 {
		return errors.New("tenant ID is required")
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
		Model(&schoolSetupRow{
			TenantID:     state.TenantID,
			SkippedSteps: skipped,
			CompletedAt:  state.CompletedAt,
			UpdatedBy:    state.UpdatedBy,
		}).
		ModelTableExpr(tableSchoolSetups).
		On("CONFLICT (tenant_id) DO UPDATE").
		Set("skipped_steps = EXCLUDED.skipped_steps").
		Set("completed_at = EXCLUDED.completed_at").
		Set("updated_by = EXCLUDED.updated_by").
		Set("updated_at = NOW()").
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("upsert school setup: %w", err)
	}
	return nil
}

// IsDismissed reports whether the account hid the wizard.
func (r *SchoolSetupRepository[S]) IsDismissed(ctx context.Context, accountID int64) (bool, error) {
	if accountID <= 0 {
		return false, errors.New("account ID is required")
	}
	ambientDB3, databaseErr3 := r.runtime.DB(ctx)
	if databaseErr3 != nil {
		return *new(bool), databaseErr3
	}
	exists, err := ambientDB3.NewSelect().
		TableExpr(tableSchoolSetupDismissals).
		Where("account_id = ?", accountID).
		Exists(ctx)
	if err != nil {
		return false, fmt.Errorf("find school setup dismissal: %w", err)
	}
	return exists, nil
}

// SetDismissed hides or shows the wizard for the account. Both directions are
// idempotent.
func (r *SchoolSetupRepository[S]) SetDismissed(ctx context.Context, tenantID, accountID int64, dismissed bool) error {
	if tenantID <= 0 || accountID <= 0 {
		return errors.New("tenant ID and account ID are required")
	}
	ambientDB4, databaseErr4 := r.runtime.DB(ctx)
	if databaseErr4 != nil {
		return databaseErr4
	}
	db := ambientDB4
	if dismissed {
		_, err := db.NewRaw(
			`INSERT INTO config.school_setup_dismissals (tenant_id, account_id)
			 VALUES (?, ?)
			 ON CONFLICT (tenant_id, account_id) DO NOTHING`,
			tenantID, accountID,
		).Exec(ctx)
		if err != nil {
			return fmt.Errorf("dismiss school setup: %w", err)
		}
		return nil
	}
	_, err := db.NewDelete().
		TableExpr(tableSchoolSetupDismissals).
		Where("account_id = ?", accountID).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("restore school setup: %w", err)
	}
	return nil
}
