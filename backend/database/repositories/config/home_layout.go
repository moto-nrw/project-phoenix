package config

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/models/config"
)

const (
	tableHomeLayouts          = "config.home_layouts"
	tableHomeLayoutsAlias     = `config.home_layouts AS "home_layout"`
	tableHomeBlockPolicies    = "config.home_block_policies"
	tableHomeBlockPolicyAlias = `config.home_block_policies AS "home_block_policy_set"`
)

// HomeLayoutRepository implements config.HomeLayoutRepository.
//
// Both stores hold one row that is always read and written whole, so every
// method here is a single-row lookup or a wholesale upsert. Tenant scoping
// comes from the runtime's tenant transaction and the RLS policy on the
// tables; the account is the only key this package supplies itself.
type HomeLayoutRepository struct {
	runtime Runtime
}

// NewHomeLayoutRepository creates a new HomeLayoutRepository.
func NewHomeLayoutRepository(runtime Runtime) config.HomeLayoutRepository {
	return &HomeLayoutRepository{runtime: runtime}
}

// FindByAccount returns the account's stored deviations, or (nil, nil) when the
// account has never customized its start page.
func (r *HomeLayoutRepository) FindByAccount(ctx context.Context, accountID int64) (*config.HomeLayout, error) {
	if accountID <= 0 {
		return nil, fmt.Errorf("account ID is required")
	}

	layout := new(config.HomeLayout)
	err := r.runtime.DB(ctx).NewSelect().
		Model(layout).
		ModelTableExpr(tableHomeLayoutsAlias).
		Where(`"home_layout".account_id = ?`, accountID).
		Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("find home layout by account: %w", err)
	}
	return layout, nil
}

// UpsertForAccount replaces the account's deviations wholesale.
func (r *HomeLayoutRepository) UpsertForAccount(ctx context.Context, layout *config.HomeLayout) error {
	if layout == nil {
		return fmt.Errorf("home layout cannot be nil")
	}
	if err := layout.Validate(); err != nil {
		return err
	}
	if layout.Overrides == nil {
		layout.Overrides = map[string]bool{}
	}

	_, err := r.runtime.DB(ctx).NewInsert().
		Model(layout).
		ModelTableExpr(tableHomeLayouts).
		On("CONFLICT (tenant_id, account_id) DO UPDATE").
		Set("overrides = EXCLUDED.overrides").
		Set("updated_at = NOW()").
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("upsert home layout: %w", err)
	}
	return nil
}

// DeleteForAccount removes the stored deviations. Deleting nothing is success:
// an account without a row already sees the recommended start page.
func (r *HomeLayoutRepository) DeleteForAccount(ctx context.Context, accountID int64) error {
	if accountID <= 0 {
		return fmt.Errorf("account ID is required")
	}

	_, err := r.runtime.DB(ctx).NewDelete().
		Model((*config.HomeLayout)(nil)).
		ModelTableExpr(tableHomeLayouts).
		Where("account_id = ?", accountID).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("delete home layout: %w", err)
	}
	return nil
}

// FindPolicies returns the tenant's prescription, or (nil, nil) when the school
// has never prescribed anything.
func (r *HomeLayoutRepository) FindPolicies(ctx context.Context) (*config.HomeBlockPolicySet, error) {
	policies := new(config.HomeBlockPolicySet)
	err := r.runtime.DB(ctx).NewSelect().
		Model(policies).
		ModelTableExpr(tableHomeBlockPolicyAlias).
		Limit(1).
		Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("find home block policies: %w", err)
	}
	return policies, nil
}

// UpsertPolicies replaces the tenant's prescription wholesale.
func (r *HomeLayoutRepository) UpsertPolicies(ctx context.Context, policies *config.HomeBlockPolicySet) error {
	if policies == nil {
		return fmt.Errorf("home block policies cannot be nil")
	}
	if err := policies.Validate(); err != nil {
		return err
	}
	if policies.Policies == nil {
		policies.Policies = map[string]config.BlockPolicy{}
	}

	_, err := r.runtime.DB(ctx).NewInsert().
		Model(policies).
		ModelTableExpr(tableHomeBlockPolicies).
		On("CONFLICT (tenant_id) DO UPDATE").
		Set("policies = EXCLUDED.policies").
		Set("updated_by = EXCLUDED.updated_by").
		Set("updated_at = NOW()").
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("upsert home block policies: %w", err)
	}
	return nil
}
