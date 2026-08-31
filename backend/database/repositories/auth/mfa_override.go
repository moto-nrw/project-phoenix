package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/models/auth"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
)

const (
	mfaOverrideTable      = "auth.mfa_overrides"
	mfaOverrideTableAlias = `auth.mfa_overrides AS "mfa_override"`
)

// MFAOverrideRepository persists tenant-scoped + platform-wide MFA
// admin overrides. See auth.MFAOverride for the row shape and
// auth.MFAOverrideRepository for the interface contract.
type MFAOverrideRepository struct {
	*base.Repository[*auth.MFAOverride]
	db *bun.DB
}

// NewMFAOverrideRepository wires the BUN repository.
func NewMFAOverrideRepository(db *bun.DB) auth.MFAOverrideRepository {
	return &MFAOverrideRepository{
		Repository: base.NewRepository[*auth.MFAOverride](db, mfaOverrideTable, "MfaOverride"),
		db:         db,
	}
}

// FindGlobal returns the (account, tenant_id IS NULL) row when an
// operator has set a platform-wide override. sql.ErrNoRows maps to
// (nil, nil) — the resolver treats "no row" as a valid state, not an
// error.
func (r *MFAOverrideRepository) FindGlobal(ctx context.Context, accountID int64) (*auth.MFAOverride, error) {
	row := new(auth.MFAOverride)
	err := base.GetDB(ctx, r.db).NewSelect().
		Model(row).
		ModelTableExpr(mfaOverrideTableAlias).
		Where("account_id = ?", accountID).
		Where("tenant_id IS NULL").
		Limit(1).
		Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, &modelBase.DatabaseError{Op: "find global mfa override", Err: base.TranslateNotFound(err)}
	}
	return row, nil
}

// FindByAccountAndTenant returns the (account, tenant) row or
// (nil, nil) when no tenant-admin override exists for the pair.
func (r *MFAOverrideRepository) FindByAccountAndTenant(ctx context.Context, accountID, tenantID int64) (*auth.MFAOverride, error) {
	if tenantID == 0 {
		// A zero tenant id is the resolver's sentinel for "outside any
		// tenant" (login flow before tenant is locked in). It must not
		// be allowed to read the platform-wide row through this method
		// — that path goes through FindGlobal explicitly.
		return nil, nil
	}
	row := new(auth.MFAOverride)
	err := base.GetDB(ctx, r.db).NewSelect().
		Model(row).
		ModelTableExpr(mfaOverrideTableAlias).
		Where("account_id = ?", accountID).
		Where("tenant_id = ?", tenantID).
		Limit(1).
		Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, &modelBase.DatabaseError{Op: "find tenant mfa override", Err: base.TranslateNotFound(err)}
	}
	return row, nil
}

// UpsertGlobal writes or replaces the platform-wide row for an
// account. ON CONFLICT targets the partial unique index over
// (account_id) WHERE tenant_id IS NULL, so the upsert collapses to
// "set the row if it exists, otherwise insert it".
func (r *MFAOverrideRepository) UpsertGlobal(ctx context.Context, override *auth.MFAOverride) error {
	if override == nil {
		return fmt.Errorf("mfa override cannot be nil")
	}
	if override.AccountID == 0 {
		return fmt.Errorf("account_id is required")
	}
	if override.TenantID != nil {
		return fmt.Errorf("global override must have tenant_id == nil")
	}
	if override.Override != auth.MFAAdminOverrideForceOff && override.Override != auth.MFAAdminOverrideForceOn {
		return fmt.Errorf("override must be force_off or force_on, got %q", override.Override)
	}
	if override.SetByType != auth.MFAOverrideSetByTypeOperator {
		return fmt.Errorf("global override must be set by operator")
	}
	// ModelTableExpr forces the schema-qualified name on INSERT.
	// Without it BUN falls back to bare "mfa_overrides" on the conflict
	// target, which the phoenix_admin role can't resolve because its
	// search_path is ("$user", public). (#1430 review round 2 follow-up.)
	_, err := base.GetDB(ctx, r.db).NewInsert().
		Model(override).
		ModelTableExpr(mfaOverrideTable).
		On("CONFLICT (account_id) WHERE tenant_id IS NULL DO UPDATE").
		Set("override = EXCLUDED.override").
		Set("set_by = EXCLUDED.set_by").
		Set("set_by_type = EXCLUDED.set_by_type").
		Set("reason = EXCLUDED.reason").
		Set("updated_at = CURRENT_TIMESTAMP").
		Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{Op: "upsert global mfa override", Err: base.TranslateNotFound(err)}
	}
	return nil
}

// UpsertTenant writes or replaces the tenant-scoped row. ON CONFLICT
// targets the partial unique index over (account_id, tenant_id) WHERE
// tenant_id IS NOT NULL.
func (r *MFAOverrideRepository) UpsertTenant(ctx context.Context, override *auth.MFAOverride) error {
	if override == nil {
		return fmt.Errorf("mfa override cannot be nil")
	}
	if override.AccountID == 0 {
		return fmt.Errorf("account_id is required")
	}
	if override.TenantID == nil || *override.TenantID == 0 {
		return fmt.Errorf("tenant override must have a non-zero tenant_id")
	}
	if override.Override != auth.MFAAdminOverrideForceOff && override.Override != auth.MFAAdminOverrideForceOn {
		return fmt.Errorf("override must be force_off or force_on, got %q", override.Override)
	}
	_, err := base.GetDB(ctx, r.db).NewInsert().
		Model(override).
		ModelTableExpr(mfaOverrideTable).
		On("CONFLICT (account_id, tenant_id) WHERE tenant_id IS NOT NULL DO UPDATE").
		Set("override = EXCLUDED.override").
		Set("set_by = EXCLUDED.set_by").
		Set("set_by_type = EXCLUDED.set_by_type").
		Set("reason = EXCLUDED.reason").
		Set("updated_at = CURRENT_TIMESTAMP").
		Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{Op: "upsert tenant mfa override", Err: base.TranslateNotFound(err)}
	}
	return nil
}

// DeleteGlobal removes the platform-wide row if one exists. Returning
// "no rows" silently is intentional — callers use it as an idempotent
// "ensure this is gone" step.
func (r *MFAOverrideRepository) DeleteGlobal(ctx context.Context, accountID int64) error {
	_, err := base.GetDB(ctx, r.db).NewDelete().
		Model((*auth.MFAOverride)(nil)).
		ModelTableExpr(mfaOverrideTable).
		Where("account_id = ?", accountID).
		Where("tenant_id IS NULL").
		Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{Op: "delete global mfa override", Err: base.TranslateNotFound(err)}
	}
	return nil
}

// DeleteTenant removes the (account, tenant) row if it exists.
func (r *MFAOverrideRepository) DeleteTenant(ctx context.Context, accountID, tenantID int64) error {
	if tenantID == 0 {
		return fmt.Errorf("tenant_id is required")
	}
	_, err := base.GetDB(ctx, r.db).NewDelete().
		Model((*auth.MFAOverride)(nil)).
		ModelTableExpr(mfaOverrideTable).
		Where("account_id = ?", accountID).
		Where("tenant_id = ?", tenantID).
		Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{Op: "delete tenant mfa override", Err: base.TranslateNotFound(err)}
	}
	return nil
}

// ListByAccount returns every override row that touches the account
// — used by the operator UI's diagnostic surface and by the test
// fixtures that need to assert a clean state.
func (r *MFAOverrideRepository) ListByAccount(ctx context.Context, accountID int64) ([]*auth.MFAOverride, error) {
	var rows []*auth.MFAOverride
	err := base.GetDB(ctx, r.db).NewSelect().
		Model(&rows).
		ModelTableExpr(mfaOverrideTableAlias).
		Where("account_id = ?", accountID).
		// OrderExpr (raw SQL) — BUN's Order() parses "NULLS FIRST"
		// as a sort direction and silently drops the clause, which
		// would let the tenant row sort ahead of the global row.
		OrderExpr("tenant_id IS NULL DESC, tenant_id ASC").
		Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{Op: "list mfa overrides by account", Err: base.TranslateNotFound(err)}
	}
	return rows, nil
}

// DeleteAllByAccount wipes every row for the account in a single
// statement. Used by the AdminDisable cascade so a reset clears the
// override slate alongside trusted devices + credential.
func (r *MFAOverrideRepository) DeleteAllByAccount(ctx context.Context, accountID int64) (int64, error) {
	res, err := base.GetDB(ctx, r.db).NewDelete().
		Model((*auth.MFAOverride)(nil)).
		ModelTableExpr(mfaOverrideTable).
		Where("account_id = ?", accountID).
		Exec(ctx)
	if err != nil {
		return 0, &modelBase.DatabaseError{Op: "delete all mfa overrides for account", Err: base.TranslateNotFound(err)}
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, &modelBase.DatabaseError{Op: "rows affected for delete all mfa overrides", Err: base.TranslateNotFound(err)}
	}
	return n, nil
}
