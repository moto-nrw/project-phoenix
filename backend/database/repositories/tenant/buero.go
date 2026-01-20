package tenant

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/tenant"
	"github.com/uptrace/bun"
)

// Table name constants (S1192 - avoid duplicate string literals)
const (
	tableTenantBuero      = "tenant.buero"
	tableExprBueroAsBuero = `tenant.buero AS "buero"`
)

// BueroRepository implements tenant.BueroRepository interface
type BueroRepository struct {
	*base.Repository[*tenant.Buero]
	db *bun.DB
}

// NewBueroRepository creates a new BueroRepository
func NewBueroRepository(db *bun.DB) tenant.BueroRepository {
	return &BueroRepository{
		Repository: base.NewRepository[*tenant.Buero](db, tableTenantBuero, "Buero"),
		db:         db,
	}
}

// Create inserts a new buero
func (r *BueroRepository) Create(ctx context.Context, buero *tenant.Buero) error {
	if buero == nil {
		return fmt.Errorf("buero cannot be nil")
	}
	if err := buero.Validate(); err != nil {
		return err
	}

	_, err := r.db.NewInsert().
		Model(buero).
		ModelTableExpr(tableTenantBuero).
		Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{Op: "create", Err: err}
	}
	return nil
}

// FindByID retrieves a buero by ID
func (r *BueroRepository) FindByID(ctx context.Context, id string) (*tenant.Buero, error) {
	buero := new(tenant.Buero)
	err := r.db.NewSelect().
		Model(buero).
		ModelTableExpr(tableExprBueroAsBuero).
		Where(`"buero".id = ?`, id).
		Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{Op: "find by id", Err: err}
	}
	return buero, nil
}

// FindByTraegerID retrieves all bueros for a traeger
func (r *BueroRepository) FindByTraegerID(ctx context.Context, traegerID string) ([]*tenant.Buero, error) {
	var bueros []*tenant.Buero
	err := r.db.NewSelect().
		Model(&bueros).
		ModelTableExpr(tableExprBueroAsBuero).
		Where(`"buero".traeger_id = ?`, traegerID).
		Order(`"buero".name ASC`).
		Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{Op: "find by traeger id", Err: err}
	}
	return bueros, nil
}

// Update updates an existing buero
func (r *BueroRepository) Update(ctx context.Context, buero *tenant.Buero) error {
	if buero == nil {
		return fmt.Errorf("buero cannot be nil")
	}
	if err := buero.Validate(); err != nil {
		return err
	}

	_, err := r.db.NewUpdate().
		Model(buero).
		ModelTableExpr(tableExprBueroAsBuero).
		Where(`"buero".id = ?`, buero.ID).
		Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{Op: "update", Err: err}
	}
	return nil
}

// Delete removes a buero
func (r *BueroRepository) Delete(ctx context.Context, id string) error {
	_, err := r.db.NewDelete().
		Model((*tenant.Buero)(nil)).
		ModelTableExpr(tableExprBueroAsBuero).
		Where(`"buero".id = ?`, id).
		Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{Op: "delete", Err: err}
	}
	return nil
}

// List retrieves all bueros
func (r *BueroRepository) List(ctx context.Context) ([]*tenant.Buero, error) {
	var bueros []*tenant.Buero
	err := r.db.NewSelect().
		Model(&bueros).
		ModelTableExpr(tableExprBueroAsBuero).
		Order(`"buero".name ASC`).
		Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{Op: "list", Err: err}
	}
	return bueros, nil
}
