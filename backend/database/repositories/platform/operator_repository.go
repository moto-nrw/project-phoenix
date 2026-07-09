package platform

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/platform"
	"github.com/uptrace/bun"
)

// Table and query constants
const (
	tablePlatformOperators      = "platform.operators"
	tablePlatformOperatorsAlias = `platform.operators AS "operator"`
)

// OperatorRepository implements platform.OperatorRepository interface
type OperatorRepository struct {
	*base.Repository[*platform.Operator]
	db *bun.DB
}

// NewOperatorRepository creates a new OperatorRepository
func NewOperatorRepository(db *bun.DB) platform.OperatorRepository {
	return &OperatorRepository{
		Repository: base.NewRepository[*platform.Operator](db, tablePlatformOperators, "Operator"),
		db:         db,
	}
}

// FindByID retrieves an operator by ID
func (r *OperatorRepository) FindByID(ctx context.Context, id int64) (*platform.Operator, error) {
	operator := new(platform.Operator)
	err := base.GetDB(ctx, r.db).NewSelect().
		Model(operator).
		ModelTableExpr(tablePlatformOperatorsAlias).
		Where(`"operator".id = ?`, id).
		Scan(ctx)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, &modelBase.DatabaseError{
			Op:  "find operator by id",
			Err: err,
		}
	}

	return operator, nil
}

// FindByIDForUpdate retrieves an operator by ID with a FOR UPDATE row lock.
// Must be called within a transaction to serialize concurrent access.
func (r *OperatorRepository) FindByIDForUpdate(ctx context.Context, id int64) (*platform.Operator, error) {
	operator := new(platform.Operator)
	err := base.GetDB(ctx, r.db).NewSelect().
		Model(operator).
		ModelTableExpr(tablePlatformOperatorsAlias).
		Where(`"operator".id = ?`, id).
		For("UPDATE").
		Scan(ctx)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, &modelBase.DatabaseError{
			Op:  "find operator by id for update",
			Err: err,
		}
	}

	return operator, nil
}

// FindByEmail retrieves an operator by email
func (r *OperatorRepository) FindByEmail(ctx context.Context, email string) (*platform.Operator, error) {
	operator := new(platform.Operator)
	err := base.GetDB(ctx, r.db).NewSelect().
		Model(operator).
		ModelTableExpr(tablePlatformOperatorsAlias).
		Where(`"operator".email = ?`, email).
		Scan(ctx)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, &modelBase.DatabaseError{
			Op:  "find operator by email",
			Err: err,
		}
	}

	return operator, nil
}

// Delete removes an operator by ID
func (r *OperatorRepository) Delete(ctx context.Context, id int64) error {
	return r.Repository.Delete(ctx, id)
}

// List retrieves all operators.
// Uses unqualified columns (no ModelTableExpr alias) because BUN's auto-generated
// SELECT column references derive from the model tag, not the alias. With the
// schema:platform tag, ModelTableExpr aliases cause "missing FROM-clause entry"
// errors on single-table SELECT queries. This is safe because there are no joins.
func (r *OperatorRepository) List(ctx context.Context) ([]*platform.Operator, error) {
	var operators []*platform.Operator
	err := base.GetDB(ctx, r.db).NewSelect().
		Model(&operators).
		Order("display_name ASC").
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "list operators",
			Err: err,
		}
	}

	return operators, nil
}

// IncrementMFAAttempts atomically bumps mfa_attempts and applies the
// lockout window once threshold is reached. Mirror of the tenant-side
// AccountRepository.IncrementMFAAttempts — see that doc-comment for the
// race-condition the SQL-level CAS closes. (#1430 review item #6)
func (r *OperatorRepository) IncrementMFAAttempts(ctx context.Context, id int64, threshold int, lockoutDuration time.Duration) (platform.OperatorMFAAttemptResult, error) {
	type incrementRow struct {
		MFAAttempts    int        `bun:"mfa_attempts"`
		MFALockedUntil *time.Time `bun:"mfa_locked_until"`
	}
	row := new(incrementRow)
	_, err := base.GetDB(ctx, r.db).NewUpdate().
		Model((*platform.Operator)(nil)).
		ModelTableExpr(tablePlatformOperators).
		Set("mfa_attempts = mfa_attempts + 1").
		Set(
			"mfa_locked_until = CASE WHEN mfa_attempts + 1 >= ? THEN now() + (? * interval '1 second') ELSE mfa_locked_until END",
			threshold, int64(lockoutDuration.Seconds()),
		).
		Where("id = ?", id).
		Returning("mfa_attempts, mfa_locked_until").
		Exec(ctx, row)
	if err != nil {
		return platform.OperatorMFAAttemptResult{}, &modelBase.DatabaseError{
			Op:  "increment operator mfa attempts",
			Err: err,
		}
	}
	return platform.OperatorMFAAttemptResult{
		Attempts:    row.MFAAttempts,
		LockedUntil: row.MFALockedUntil,
	}, nil
}

// ResetMFAAttempts atomically clears mfa_attempts and mfa_locked_until
// after a successful operator verify.
func (r *OperatorRepository) ResetMFAAttempts(ctx context.Context, id int64) error {
	_, err := base.GetDB(ctx, r.db).NewUpdate().
		Model((*platform.Operator)(nil)).
		ModelTableExpr(tablePlatformOperators).
		Set("mfa_attempts = 0").
		Set("mfa_locked_until = NULL").
		Where("id = ?", id).
		Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "reset operator mfa attempts",
			Err: err,
		}
	}
	return nil
}

// UpdateLastLogin updates the last login timestamp
func (r *OperatorRepository) UpdateLastLogin(ctx context.Context, id int64) error {
	now := time.Now()
	operator := &platform.Operator{Model: modelBase.Model{ID: id}, LastLogin: &now}
	_, err := r.UpdateColumns(ctx, operator, "last_login")
	return err
}
