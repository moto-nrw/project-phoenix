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
	tableSchoolSetups          = "config.school_setups"
	tableSchoolSetupsAlias     = `config.school_setups AS "school_setup"`
	tableSchoolSetupDismissals = "config.school_setup_dismissals"
)

// schoolSetupRow is one config.school_setups row (#2832, ADR 0040).
type schoolSetupRow struct {
	ID                int64      `bun:"id,pk,autoincrement"`
	TenantID          int64      `bun:"tenant_id,notnull"`
	ParentAppUsed     *bool      `bun:"parent_app_used"`
	SkippedSteps      []string   `bun:"skipped_steps,array,notnull"`
	BasicsConfirmedAt *time.Time `bun:"basics_confirmed_at"`
	CompletedAt       *time.Time `bun:"completed_at"`
	UpdatedBy         *int64     `bun:"updated_by"`
	CreatedAt         time.Time  `bun:"created_at,notnull,default:now()"`
	UpdatedAt         time.Time  `bun:"updated_at,notnull,default:now()"`
}

// SchoolSetupRepository implements schoolsetup.Store. Both tables belong to
// Settings Platform, which is why the wizard's storage lives here.
//
// Tenant scoping comes from the runtime's tenant transaction and the RLS
// policy on both tables; the account is the only key this package supplies
// itself.
type SchoolSetupRepository struct {
	runtime Runtime
}

// NewSchoolSetupRepository creates a new SchoolSetupRepository.
func NewSchoolSetupRepository(runtime Runtime) schoolsetup.Store {
	return &SchoolSetupRepository{runtime: runtime}
}

// Find returns the school's wizard state, or (nil, nil) for a new school.
func (r *SchoolSetupRepository) Find(ctx context.Context) (*schoolsetup.State, error) {
	stored := new(schoolSetupRow)
	err := r.runtime.DB(ctx).NewSelect().
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
	return &schoolsetup.State{
		TenantID:          stored.TenantID,
		ParentAppUsed:     stored.ParentAppUsed,
		SkippedSteps:      stored.SkippedSteps,
		BasicsConfirmedAt: stored.BasicsConfirmedAt,
		CompletedAt:       stored.CompletedAt,
		UpdatedBy:         stored.UpdatedBy,
	}, nil
}

// Upsert replaces the school's wizard state wholesale.
func (r *SchoolSetupRepository) Upsert(ctx context.Context, state *schoolsetup.State) error {
	if state == nil {
		return errors.New("school setup cannot be nil")
	}
	if state.TenantID <= 0 {
		return errors.New("tenant ID is required")
	}
	skipped := state.SkippedSteps
	if skipped == nil {
		skipped = []string{}
	}

	_, err := r.runtime.DB(ctx).NewInsert().
		Model(&schoolSetupRow{
			TenantID:          state.TenantID,
			ParentAppUsed:     state.ParentAppUsed,
			SkippedSteps:      skipped,
			BasicsConfirmedAt: state.BasicsConfirmedAt,
			CompletedAt:       state.CompletedAt,
			UpdatedBy:         state.UpdatedBy,
		}).
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
		return false, errors.New("account ID is required")
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
		return errors.New("tenant ID and account ID are required")
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
