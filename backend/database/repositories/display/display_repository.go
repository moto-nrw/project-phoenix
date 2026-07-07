package display

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/models/display"
	"github.com/uptrace/bun"
)

const displayTableExpr = `display.displays AS "display"`

// DisplayRepository implements display.Repository.
type DisplayRepository struct {
	*base.Repository[*display.Display]
	db *bun.DB
}

// NewDisplayRepository creates a new DisplayRepository.
func NewDisplayRepository(db *bun.DB) display.Repository {
	repo := base.NewRepository[*display.Display](db, "display.displays", "Display")
	repo.TenantScoped = true
	return &DisplayRepository{
		Repository: repo,
		db:         db,
	}
}

// FindByTokenHash looks up a display by its SHA-256 token hash WITHOUT a
// tenant predicate. CONTRACT: callers MUST run this inside tenant.WithAdminTx
// (BYPASSRLS) — the token is the only auth signal and the tenant is not known
// yet. The returned TenantID then scopes all downstream data queries via
// tenant.WithTenantTx. Never call this from a tenant-scoped transaction.
func (r *DisplayRepository) FindByTokenHash(ctx context.Context, tokenHash string) (*display.Display, error) {
	d := new(display.Display)
	err := base.GetDB(ctx, r.db).NewSelect().
		Model(d).
		ModelTableExpr(displayTableExpr).
		Where(`"display".token_hash = ?`, tokenHash).
		Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, display.ErrNotFound
		}
		return nil, fmt.Errorf("failed to find display by token hash: %w", err)
	}
	return d, nil
}
