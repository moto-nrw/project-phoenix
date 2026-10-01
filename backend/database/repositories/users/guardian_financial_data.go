package users

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/uptrace/bun"
)

// GuardianFinancialDataRepository implements
// users.GuardianFinancialDataRepository (#2608). A narrow repository of its
// own rather than a widening of GuardianProfileRepository: the bank row must
// stay an isolated dependency so only the guardians:financial service path can
// reach it.
type GuardianFinancialDataRepository struct {
	runtime Runtime
}

// NewGuardianFinancialDataRepository creates the 1:1 guardian bank repository.
func NewGuardianFinancialDataRepository(runtime Runtime) users.GuardianFinancialDataRepository {
	return &GuardianFinancialDataRepository{runtime: requireRuntime(runtime)}
}

// FindByGuardianProfileID returns the guardian's bank row, or (nil, nil) when
// none exists yet — a guardian without bank details is a normal state, not an
// error.
func (r *GuardianFinancialDataRepository) FindByGuardianProfileID(ctx context.Context, guardianProfileID int64) (*users.GuardianFinancialData, error) {
	data := new(users.GuardianFinancialData)
	query := r.runtime.DB(ctx).NewSelect().
		Model(data).
		ModelTableExpr(`users.guardian_financial_data AS "guardian_financial_data"`).
		Where(`"guardian_financial_data".guardian_profile_id = ?`, guardianProfileID)

	query = withTenantFilter(ctx, r.runtime, query, "guardian_financial_data")

	if err := query.Scan(ctx); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, &users.DatabaseError{Op: "find guardian financial data by guardian profile id", Err: translateNotFound(err)}
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
	query := r.runtime.DB(ctx).NewSelect().
		Model(&rows).
		ModelTableExpr(`users.guardian_financial_data AS "guardian_financial_data"`).
		Where(`"guardian_financial_data".guardian_profile_id IN (?)`, bun.List(guardianProfileIDs))

	query = withTenantFilter(ctx, r.runtime, query, "guardian_financial_data")

	if err := query.Scan(ctx); err != nil {
		return nil, &users.DatabaseError{Op: "list guardian financial data by guardian profile ids", Err: translateNotFound(err)}
	}

	for _, row := range rows {
		out[row.GuardianProfileID] = row
	}
	return out, nil
}

// Create inserts a guardian's bank row, stamping the context's tenant when the
// row has none yet.
func (r *GuardianFinancialDataRepository) Create(ctx context.Context, data *users.GuardianFinancialData) error {
	if data == nil {
		return fmt.Errorf("%s cannot be nil or zero value", "GuardianFinancialData")
	}
	if err := data.Validate(); err != nil {
		return err
	}
	ensureTenantID(ctx, r.runtime, data)
	if _, err := r.runtime.DB(ctx).NewInsert().Model(data).ModelTableExpr(`users.guardian_financial_data`).Exec(ctx); err != nil {
		return &users.DatabaseError{Op: "create", Err: err}
	}
	return nil
}

// Update writes a guardian's bank row.
func (r *GuardianFinancialDataRepository) Update(ctx context.Context, data *users.GuardianFinancialData) error {
	if data == nil {
		return fmt.Errorf("%s cannot be nil or zero value", "GuardianFinancialData")
	}
	if err := data.Validate(); err != nil {
		return err
	}
	query := r.runtime.DB(ctx).NewUpdate().
		Model(data).
		ModelTableExpr(`users.guardian_financial_data AS "guardian_financial_data"`).
		WherePK()
	query = withTenantFilter(ctx, r.runtime, query, "guardian_financial_data")
	result, err := query.Exec(ctx)
	if err != nil {
		return &users.DatabaseError{Op: "update", Err: err}
	}
	return assertRowsAffected(result, 1, "update GuardianFinancialData")
}
