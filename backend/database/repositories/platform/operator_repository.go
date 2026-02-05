package platform

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
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

// Create inserts a new operator
func (r *OperatorRepository) Create(ctx context.Context, operator *platform.Operator) error {
	if operator == nil {
		return fmt.Errorf("operator cannot be nil")
	}

	if err := operator.Validate(); err != nil {
		return err
	}

	return r.Repository.Create(ctx, operator)
}

// FindByID retrieves an operator by ID
func (r *OperatorRepository) FindByID(ctx context.Context, id int64) (*platform.Operator, error) {
	operator := new(platform.Operator)
	err := r.db.NewSelect().
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

// FindByEmail retrieves an operator by email
func (r *OperatorRepository) FindByEmail(ctx context.Context, email string) (*platform.Operator, error) {
	operator := new(platform.Operator)
	err := r.db.NewSelect().
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

// Update updates an operator
func (r *OperatorRepository) Update(ctx context.Context, operator *platform.Operator) error {
	if operator == nil {
		return fmt.Errorf("operator cannot be nil")
	}

	if err := operator.Validate(); err != nil {
		return err
	}

	return r.Repository.Update(ctx, operator)
}

// Delete removes an operator by ID
func (r *OperatorRepository) Delete(ctx context.Context, id int64) error {
	return r.Repository.Delete(ctx, id)
}

// List retrieves all operators
func (r *OperatorRepository) List(ctx context.Context) ([]*platform.Operator, error) {
	var operators []*platform.Operator
	err := r.db.NewSelect().
		Model(&operators).
		ModelTableExpr(tablePlatformOperatorsAlias).
		Order(`"operator".display_name ASC`).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "list operators",
			Err: err,
		}
	}

	return operators, nil
}

// UpdateLastLogin updates the last login timestamp
func (r *OperatorRepository) UpdateLastLogin(ctx context.Context, id int64) error {
	now := time.Now()
	_, err := r.db.NewUpdate().
		Model((*platform.Operator)(nil)).
		ModelTableExpr(tablePlatformOperators).
		Set("last_login = ?", now).
		Where("id = ?", id).
		Exec(ctx)

	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "update operator last login",
			Err: err,
		}
	}

	return nil
}
