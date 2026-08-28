package users

import (
	"context"
	"database/sql"
	"errors"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/uptrace/bun"
)

// GuardianFinancialDataRepository implements
// users.GuardianFinancialDataRepository (#2608). A narrow repository of its
// own rather than a widening of GuardianProfileRepository: the bank row must
// stay an isolated dependency so only the guardians:financial service path can
// reach it.
type GuardianFinancialDataRepository struct {
	*base.Repository[*users.GuardianFinancialData]
	db *bun.DB
}

// NewGuardianFinancialDataRepository creates the 1:1 guardian bank repository.
func NewGuardianFinancialDataRepository(db *bun.DB) users.GuardianFinancialDataRepository {
	repo := base.NewRepository[*users.GuardianFinancialData](db, "users.guardian_financial_data", "GuardianFinancialData")
	repo.TenantScoped = true
	return &GuardianFinancialDataRepository{Repository: repo, db: db}
}

// FindByGuardianProfileID returns the guardian's bank row, or (nil, nil) when
// none exists yet — a guardian without bank details is a normal state, not an
// error.
func (r *GuardianFinancialDataRepository) FindByGuardianProfileID(ctx context.Context, guardianProfileID int64) (*users.GuardianFinancialData, error) {
	data := new(users.GuardianFinancialData)
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(data).
		ModelTableExpr(`users.guardian_financial_data AS "guardian_financial_data"`).
		Where(`"guardian_financial_data".guardian_profile_id = ?`, guardianProfileID)

	query = base.WithTenantFilter(ctx, query, "guardian_financial_data")

	if err := query.Scan(ctx); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, &modelBase.DatabaseError{Op: "find guardian financial data by guardian profile id", Err: err}
	}
	return data, nil
}

// ListByGuardianProfileIDs loads several guardians' bank rows in one query,
// keyed by guardian profile ID. Guardians without a row are simply absent from
// the map.
func (r *GuardianFinancialDataRepository) ListByGuardianProfileIDs(ctx context.Context, guardianProfileIDs []int64) (map[int64]*users.GuardianFinancialData, error) {
	out := make(map[int64]*users.GuardianFinancialData, len(guardianProfileIDs))
	if len(guardianProfileIDs) == 0 {
		return out, nil
	}

	var rows []*users.GuardianFinancialData
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&rows).
		ModelTableExpr(`users.guardian_financial_data AS "guardian_financial_data"`).
		Where(`"guardian_financial_data".guardian_profile_id IN (?)`, bun.List(guardianProfileIDs))

	query = base.WithTenantFilter(ctx, query, "guardian_financial_data")

	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{Op: "list guardian financial data by guardian profile ids", Err: err}
	}

	for _, row := range rows {
		out[row.GuardianProfileID] = row
	}
	return out, nil
}
