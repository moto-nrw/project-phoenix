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
	tableTenantTraeger        = "tenant.traeger"
	tableExprTraegerAsTraeger = `tenant.traeger AS "traeger"`
)

// TraegerRepository implements tenant.TraegerRepository interface
type TraegerRepository struct {
	*base.Repository[*tenant.Traeger]
	db *bun.DB
}

// NewTraegerRepository creates a new TraegerRepository
func NewTraegerRepository(db *bun.DB) tenant.TraegerRepository {
	return &TraegerRepository{
		Repository: base.NewRepository[*tenant.Traeger](db, tableTenantTraeger, "Traeger"),
		db:         db,
	}
}

// Create inserts a new traeger
func (r *TraegerRepository) Create(ctx context.Context, traeger *tenant.Traeger) error {
	if traeger == nil {
		return fmt.Errorf("traeger cannot be nil")
	}
	if err := traeger.Validate(); err != nil {
		return err
	}

	_, err := r.db.NewInsert().
		Model(traeger).
		ModelTableExpr(tableTenantTraeger).
		ExcludeColumn("id").
		Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{Op: "create", Err: err}
	}
	return nil
}

// FindByID retrieves a traeger by ID
func (r *TraegerRepository) FindByID(ctx context.Context, id string) (*tenant.Traeger, error) {
	traeger := new(tenant.Traeger)
	err := r.db.NewSelect().
		Model(traeger).
		ModelTableExpr(tableExprTraegerAsTraeger).
		Where(`"traeger".id = ?`, id).
		Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{Op: "find by id", Err: err}
	}
	return traeger, nil
}

// FindByName retrieves a traeger by name
func (r *TraegerRepository) FindByName(ctx context.Context, name string) (*tenant.Traeger, error) {
	traeger := new(tenant.Traeger)
	err := r.db.NewSelect().
		Model(traeger).
		ModelTableExpr(tableExprTraegerAsTraeger).
		Where(`LOWER("traeger".name) = LOWER(?)`, name).
		Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{Op: "find by name", Err: err}
	}
	return traeger, nil
}

// Update updates an existing traeger
func (r *TraegerRepository) Update(ctx context.Context, traeger *tenant.Traeger) error {
	if traeger == nil {
		return fmt.Errorf("traeger cannot be nil")
	}
	if err := traeger.Validate(); err != nil {
		return err
	}

	_, err := r.db.NewUpdate().
		Model(traeger).
		ModelTableExpr(tableExprTraegerAsTraeger).
		Where(`"traeger".id = ?`, traeger.ID).
		Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{Op: "update", Err: err}
	}
	return nil
}

// Delete removes a traeger
func (r *TraegerRepository) Delete(ctx context.Context, id string) error {
	_, err := r.db.NewDelete().
		Model((*tenant.Traeger)(nil)).
		ModelTableExpr(tableExprTraegerAsTraeger).
		Where(`"traeger".id = ?`, id).
		Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{Op: "delete", Err: err}
	}
	return nil
}

// List retrieves all traegers
func (r *TraegerRepository) List(ctx context.Context) ([]*tenant.Traeger, error) {
	var traegers []*tenant.Traeger
	err := r.db.NewSelect().
		Model(&traegers).
		ModelTableExpr(tableTenantTraeger).
		Order(`name ASC`).
		Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{Op: "list", Err: err}
	}
	return traegers, nil
}

// FindWithBueros retrieves a traeger with all bueros loaded
func (r *TraegerRepository) FindWithBueros(ctx context.Context, id string) (*tenant.Traeger, error) {
	traeger := new(tenant.Traeger)
	err := r.db.NewSelect().
		Model(traeger).
		ModelTableExpr(tableExprTraegerAsTraeger).
		Relation("Bueros", func(q *bun.SelectQuery) *bun.SelectQuery {
			return q.ModelTableExpr(`tenant.buero AS "buero"`)
		}).
		Where(`"traeger".id = ?`, id).
		Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{Op: "find with bueros", Err: err}
	}
	return traeger, nil
}
