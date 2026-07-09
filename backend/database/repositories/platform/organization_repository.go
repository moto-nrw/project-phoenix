package platform

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/platform"
	"github.com/uptrace/bun"
)

const (
	tablePlatformOrganizations      = "platform.organizations"
	tablePlatformOrganizationsAlias = `platform.organizations AS "organization"`
)

// OrganizationRepository implements platform.OrganizationRepository.
type OrganizationRepository struct {
	*base.Repository[*platform.Organization]
	db *bun.DB
}

// NewOrganizationRepository creates a new OrganizationRepository.
func NewOrganizationRepository(db *bun.DB) platform.OrganizationRepository {
	return &OrganizationRepository{
		Repository: base.NewRepository[*platform.Organization](db, tablePlatformOrganizations, "Organization"),
		db:         db,
	}
}

func (r *OrganizationRepository) FindByID(ctx context.Context, id int64) (*platform.Organization, error) {
	organization := new(platform.Organization)
	err := base.GetDB(ctx, r.db).NewSelect().
		Model(organization).
		ModelTableExpr(tablePlatformOrganizationsAlias).
		Where(`"organization".id = ?`, id).
		Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, &modelBase.DatabaseError{Op: "find organization by id", Err: err}
	}
	return organization, nil
}

// findByIDWithLock returns an organization by ID while acquiring the given
// row-level lock. `lockClause` is passed verbatim to bun's `For(...)` builder
// (e.g. "SHARE", "UPDATE"). `op` labels the operation in DatabaseError on
// wrapping failure. Must be called within a transaction.
//
// Locking contract shared by FindByIDForShare and FindByIDForUpdate:
//
// Any service method that mutates a school in a way that depends on the parent
// organization NOT being deleted (CreateSchool, UpdateSchool when the org
// changes, RestoreSchool, UpdateOrganization) takes FOR SHARE on the parent
// org row. This pins the row for the lifetime of the transaction so
// SoftDeleteOrganization (which takes FOR UPDATE on the same row) cannot
// commit deleted_at between the IsDeleted check and the subsequent write.
// Without this, a school insert or update could land after the
// CountNonDeletedByOrganizationID check but before the organization's
// SoftDelete commits, leaving a live school under a tombstoned organization.
//
// Multiple FOR SHARE readers can hold the lock concurrently (schools under
// the same org can be mutated in parallel); FOR UPDATE blocks until all
// FOR SHARE holders release. Lock order is always org → school, so no
// deadlock.
func (r *OrganizationRepository) findByIDWithLock(ctx context.Context, id int64, lockClause, op string) (*platform.Organization, error) {
	organization := new(platform.Organization)
	err := base.GetDB(ctx, r.db).NewSelect().
		Model(organization).
		ModelTableExpr(tablePlatformOrganizationsAlias).
		Where(`"organization".id = ?`, id).
		For(lockClause).
		Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, &modelBase.DatabaseError{Op: op, Err: err}
	}
	return organization, nil
}

// FindByIDForShare returns an organization by ID while acquiring a FOR SHARE
// lock on the row. See findByIDWithLock for the locking contract.
func (r *OrganizationRepository) FindByIDForShare(ctx context.Context, id int64) (*platform.Organization, error) {
	return r.findByIDWithLock(ctx, id, "SHARE", "find organization by id for share")
}

// FindByIDForUpdate returns an organization by ID while acquiring a FOR
// UPDATE lock on the row. SoftDeleteOrganization uses this to serialize
// against concurrent school mutations that take FOR SHARE on the same row.
// See findByIDWithLock for the locking contract.
func (r *OrganizationRepository) FindByIDForUpdate(ctx context.Context, id int64) (*platform.Organization, error) {
	return r.findByIDWithLock(ctx, id, "UPDATE", "find organization by id for update")
}

func (r *OrganizationRepository) FindBySlug(ctx context.Context, slug string) (*platform.Organization, error) {
	organization := new(platform.Organization)
	err := base.GetDB(ctx, r.db).NewSelect().
		Model(organization).
		ModelTableExpr(tablePlatformOrganizationsAlias).
		Where(`"organization".slug = ?`, slug).
		Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, &modelBase.DatabaseError{Op: "find organization by slug", Err: err}
	}
	return organization, nil
}

func (r *OrganizationRepository) Update(ctx context.Context, organization *platform.Organization) error {
	if organization == nil {
		return fmt.Errorf("organization cannot be nil")
	}
	if err := organization.Validate(); err != nil {
		return err
	}
	result, err := base.GetDB(ctx, r.db).NewUpdate().
		Model(organization).
		ModelTableExpr(tablePlatformOrganizationsAlias).
		Column("name", "slug", "active", "settings").
		Where(`"organization".id = ?`, organization.ID).
		Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{Op: "update organization", Err: err}
	}
	return base.AssertRowsAffected(result, 1, "update organization")
}

// CountByIDs counts how many of the given IDs exist in the organizations table,
// excluding soft-deleted rows. This is used to enforce that new announcement
// targeting (create + newly-added IDs on update) cannot reference trashed
// organizations. Historical targets already on an announcement are allowed
// through by the service layer via a diff against the existing record.
func (r *OrganizationRepository) CountByIDs(ctx context.Context, ids []int64) (int, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	count, err := base.GetDB(ctx, r.db).NewSelect().
		Model((*platform.Organization)(nil)).
		ModelTableExpr(tablePlatformOrganizationsAlias).
		Where(`"organization".id IN (?)`, bun.List(ids)).
		Where(`"organization".deleted_at IS NULL`).
		Count(ctx)
	if err != nil {
		return 0, &modelBase.DatabaseError{Op: "count organizations by ids", Err: err}
	}
	return count, nil
}

func (r *OrganizationRepository) List(ctx context.Context) ([]*platform.Organization, error) {
	var organizations []*platform.Organization
	err := base.GetDB(ctx, r.db).NewSelect().
		Model(&organizations).
		ModelTableExpr(tablePlatformOrganizationsAlias).
		OrderExpr(`"organization".name ASC`).
		Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{Op: "list organizations", Err: err}
	}
	return organizations, nil
}

// SoftDelete sets deleted_at on an organization. Fails if the organization is already deleted.
func (r *OrganizationRepository) SoftDelete(ctx context.Context, id int64) error {
	result, err := base.GetDB(ctx, r.db).NewUpdate().
		ModelTableExpr(tablePlatformOrganizationsAlias).
		Set(`deleted_at = NOW()`).
		Where(`"organization".id = ?`, id).
		Where(`"organization".deleted_at IS NULL`).
		Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{Op: "soft delete organization", Err: err}
	}
	return base.AssertRowsAffected(result, 1, "soft delete organization")
}

// Restore clears deleted_at on a soft-deleted organization. Fails if the organization is not deleted.
func (r *OrganizationRepository) Restore(ctx context.Context, id int64) error {
	result, err := base.GetDB(ctx, r.db).NewUpdate().
		ModelTableExpr(tablePlatformOrganizationsAlias).
		Set(`deleted_at = NULL`).
		Where(`"organization".id = ?`, id).
		Where(`"organization".deleted_at IS NOT NULL`).
		Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{Op: "restore organization", Err: err}
	}
	return base.AssertRowsAffected(result, 1, "restore organization")
}
