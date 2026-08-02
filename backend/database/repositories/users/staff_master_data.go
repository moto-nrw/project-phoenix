package users

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/uptrace/bun"
)

// Stammdaten repositories (#1423). Three narrow repos instead of widening
// StaffRepository: the financial repo in particular must stay an isolated
// dependency so only the staff:financial service path can reach it.

// StaffMasterDataRepository implements users.StaffMasterDataRepository.
type StaffMasterDataRepository struct {
	*base.Repository[*users.StaffMasterData]
	db *bun.DB
}

// NewStaffMasterDataRepository creates the 1:1 master-data repository.
func NewStaffMasterDataRepository(db *bun.DB) users.StaffMasterDataRepository {
	repo := base.NewRepository[*users.StaffMasterData](db, "users.staff_master_data", "StaffMasterData")
	repo.TenantScoped = true
	return &StaffMasterDataRepository{Repository: repo, db: db}
}

// FindByStaffID returns the staff member's master-data row, or (nil, nil)
// when none exists yet — an empty Stammdaten tab is a normal state, not an
// error.
func (r *StaffMasterDataRepository) FindByStaffID(ctx context.Context, staffID int64) (*users.StaffMasterData, error) {
	data := new(users.StaffMasterData)
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(data).
		ModelTableExpr(`users.staff_master_data AS "staff_master_data"`).
		Where(`"staff_master_data".staff_id = ?`, staffID)

	query = base.WithTenantFilter(ctx, query, "staff_master_data")

	if err := query.Scan(ctx); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, &modelBase.DatabaseError{Op: "find staff master data by staff id", Err: err}
	}
	return data, nil
}

// StaffQualificationRepository implements users.StaffQualificationRepository.
type StaffQualificationRepository struct {
	*base.Repository[*users.StaffQualification]
	db *bun.DB
}

// NewStaffQualificationRepository creates the qualification repository.
func NewStaffQualificationRepository(db *bun.DB) users.StaffQualificationRepository {
	repo := base.NewRepository[*users.StaffQualification](db, "users.staff_qualifications", "StaffQualification")
	repo.TenantScoped = true
	return &StaffQualificationRepository{Repository: repo, db: db}
}

// ListByStaffID returns the staff member's qualifications, oldest first.
func (r *StaffQualificationRepository) ListByStaffID(ctx context.Context, staffID int64) ([]*users.StaffQualification, error) {
	var rows []*users.StaffQualification
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&rows).
		ModelTableExpr(`users.staff_qualifications AS "staff_qualification"`).
		Where(`"staff_qualification".staff_id = ?`, staffID).
		Order("id ASC")

	query = base.WithTenantFilter(ctx, query, "staff_qualification")

	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{Op: "list staff qualifications", Err: err}
	}
	return rows, nil
}

// ReplaceForStaff atomically replaces the qualification list of one staff
// member (delete + insert). The section edit modal submits the full list;
// diff-merging concurrent editors would silently interleave half-lists.
// Callers run this inside a tenant transaction; tenant scoping of the delete
// is enforced by RLS plus the staff_id ownership check in the service.
func (r *StaffQualificationRepository) ReplaceForStaff(ctx context.Context, staffID int64, qualifications []*users.StaffQualification) error {
	db := base.GetDB(ctx, r.db)

	if _, err := db.NewDelete().
		Model((*users.StaffQualification)(nil)).
		ModelTableExpr(`users.staff_qualifications AS "staff_qualification"`).
		Where(`"staff_qualification".staff_id = ?`, staffID).
		Exec(ctx); err != nil {
		return &modelBase.DatabaseError{Op: "delete staff qualifications", Err: err}
	}

	if len(qualifications) == 0 {
		return nil
	}

	for _, q := range qualifications {
		q.StaffID = staffID
		base.EnsureTenantID(ctx, q)
		if err := q.Validate(); err != nil {
			return fmt.Errorf("invalid staff qualification: %w", err)
		}
	}

	if _, err := db.NewInsert().
		Model(&qualifications).
		ModelTableExpr(`users.staff_qualifications`).
		Exec(ctx); err != nil {
		return &modelBase.DatabaseError{Op: "insert staff qualifications", Err: err}
	}
	return nil
}

// StaffFinancialDataRepository implements users.StaffFinancialDataRepository.
type StaffFinancialDataRepository struct {
	*base.Repository[*users.StaffFinancialData]
	db *bun.DB
}

// NewStaffFinancialDataRepository creates the 1:1 financial-data repository.
func NewStaffFinancialDataRepository(db *bun.DB) users.StaffFinancialDataRepository {
	repo := base.NewRepository[*users.StaffFinancialData](db, "users.staff_financial_data", "StaffFinancialData")
	repo.TenantScoped = true
	return &StaffFinancialDataRepository{Repository: repo, db: db}
}

// FindByStaffID returns the staff member's financial row, or (nil, nil) when
// none exists yet.
func (r *StaffFinancialDataRepository) FindByStaffID(ctx context.Context, staffID int64) (*users.StaffFinancialData, error) {
	data := new(users.StaffFinancialData)
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(data).
		ModelTableExpr(`users.staff_financial_data AS "staff_financial_data"`).
		Where(`"staff_financial_data".staff_id = ?`, staffID)

	query = base.WithTenantFilter(ctx, query, "staff_financial_data")

	if err := query.Scan(ctx); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, &modelBase.DatabaseError{Op: "find staff financial data by staff id", Err: err}
	}
	return data, nil
}
