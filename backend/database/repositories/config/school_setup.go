package config

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/models/config"
)

const (
	tableSchoolSetups          = "config.school_setups"
	tableSchoolSetupsAlias     = `config.school_setups AS "school_setup"`
	tableSchoolSetupDismissals = "config.school_setup_dismissals"
)

// SchoolSetupRepository implements config.SchoolSetupRepository.
//
// Tenant scoping comes from the runtime's tenant transaction and the RLS
// policy on both tables; the account is the only key this package supplies
// itself.
type SchoolSetupRepository struct {
	runtime Runtime
}

// NewSchoolSetupRepository creates a new SchoolSetupRepository.
func NewSchoolSetupRepository(runtime Runtime) config.SchoolSetupRepository {
	return &SchoolSetupRepository{runtime: runtime}
}

// Find returns the school's wizard state, or (nil, nil) for a new school.
func (r *SchoolSetupRepository) Find(ctx context.Context) (*config.SchoolSetup, error) {
	setup := new(config.SchoolSetup)
	err := r.runtime.DB(ctx).NewSelect().
		Model(setup).
		ModelTableExpr(tableSchoolSetupsAlias).
		Limit(1).
		Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("find school setup: %w", err)
	}
	return setup, nil
}

// Upsert replaces the school's wizard state wholesale.
func (r *SchoolSetupRepository) Upsert(ctx context.Context, setup *config.SchoolSetup) error {
	if setup == nil {
		return fmt.Errorf("school setup cannot be nil")
	}
	if setup.TenantID <= 0 {
		return fmt.Errorf("tenant ID is required")
	}
	if setup.SkippedSteps == nil {
		setup.SkippedSteps = []string{}
	}

	_, err := r.runtime.DB(ctx).NewInsert().
		Model(setup).
		ModelTableExpr(tableSchoolSetups).
		On("CONFLICT (tenant_id) DO UPDATE").
		Set("parent_app_used = EXCLUDED.parent_app_used").
		Set("skipped_steps = EXCLUDED.skipped_steps").
		Set("basics_confirmed_at = EXCLUDED.basics_confirmed_at").
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
func (r *SchoolSetupRepository) IsDismissed(ctx context.Context, accountID int64) (bool, error) {
	if accountID <= 0 {
		return false, fmt.Errorf("account ID is required")
	}
	exists, err := r.runtime.DB(ctx).NewSelect().
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
func (r *SchoolSetupRepository) SetDismissed(ctx context.Context, tenantID, accountID int64, dismissed bool) error {
	if tenantID <= 0 || accountID <= 0 {
		return fmt.Errorf("tenant ID and account ID are required")
	}
	db := r.runtime.DB(ctx)
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
