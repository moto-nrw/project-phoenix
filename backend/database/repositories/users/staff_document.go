package users

import (
	"context"
	"database/sql"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/uptrace/bun"
)

// StaffDocumentRepository implements users.StaffDocumentRepository (#1424).
// The soft_delete tag on the model keeps deleted rows out of every select
// automatically; SoftDelete is a domain operation because deleted_by must be
// stamped in the same write (phoenix_tenant has no DELETE grant).
type StaffDocumentRepository struct {
	*base.Repository[*users.StaffDocument]
	db *bun.DB
}

// NewStaffDocumentRepository creates the staff document metadata repository.
func NewStaffDocumentRepository(db *bun.DB) users.StaffDocumentRepository {
	repo := base.NewRepository[*users.StaffDocument](db, "users.staff_documents", "StaffDocument")
	repo.TenantScoped = true
	return &StaffDocumentRepository{Repository: repo, db: db}
}

// FindForStaff loads one non-deleted document belonging to the given staff
// member. sql.ErrNoRows propagates (wrapped) — a missing or foreign document
// is a 404, not a normal state.
func (r *StaffDocumentRepository) FindForStaff(ctx context.Context, staffID, documentID int64) (*users.StaffDocument, error) {
	return r.findForStaff(ctx, staffID, documentID, false)
}

func (r *StaffDocumentRepository) FindForStaffIncludingDeleted(ctx context.Context, staffID, documentID int64) (*users.StaffDocument, error) {
	return r.findForStaff(ctx, staffID, documentID, true)
}

func (r *StaffDocumentRepository) findForStaff(ctx context.Context, staffID, documentID int64, includeDeleted bool) (*users.StaffDocument, error) {
	doc := new(users.StaffDocument)
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(doc).
		ModelTableExpr(`users.staff_documents AS "staff_document"`).
		Where(`"staff_document".id = ?`, documentID).
		Where(`"staff_document".staff_id = ?`, staffID)
	if includeDeleted {
		query = query.WhereAllWithDeleted()
	}

	query = base.WithTenantFilter(ctx, query, "staff_document")

	if err := query.Scan(ctx); err != nil {
		// sql.ErrNoRows stays wrapped so modelBase.IsNoRows classifies the
		// missing (or foreign) document as a 404.
		return nil, &modelBase.DatabaseError{Op: "find staff document", Err: err}
	}
	return doc, nil
}

// ListByStaffID returns the staff member's non-deleted documents, newest
// first, restricted to the given categories. The category list is the
// caller's visibility set — empty means the caller may see no category, so
// the result is empty without touching the database.
func (r *StaffDocumentRepository) ListByStaffID(ctx context.Context, staffID int64, categories []string) ([]*users.StaffDocument, error) {
	if len(categories) == 0 {
		return []*users.StaffDocument{}, nil
	}

	var rows []*users.StaffDocument
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&rows).
		ModelTableExpr(`users.staff_documents AS "staff_document"`).
		Where(`"staff_document".staff_id = ?`, staffID).
		Where(`"staff_document".category IN (?)`, bun.List(categories)).
		Order("created_at DESC", "id DESC")

	query = base.WithTenantFilter(ctx, query, "staff_document")

	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{Op: "list staff documents", Err: err}
	}
	return rows, nil
}

// ListStoredByStaffID returns all filenames still recorded for a staff
// member, including soft-deleted rows. The offboarding retry path needs these
// durable references after the staff row itself has been soft-deleted.
func (r *StaffDocumentRepository) ListStoredByStaffID(ctx context.Context, staffID int64) ([]string, error) {
	var filenames []string
	query := base.GetDB(ctx, r.db).NewSelect().
		Model((*users.StaffDocument)(nil)).
		ModelTableExpr(`users.staff_documents AS "staff_document"`).
		ColumnExpr(`"staff_document".filename_stored`).
		Where(`"staff_document".staff_id = ?`, staffID).
		WhereAllWithDeleted()

	query = base.WithTenantFilter(ctx, query, "staff_document")
	if err := query.Scan(ctx, &filenames); err != nil {
		return nil, &modelBase.DatabaseError{Op: "list stored staff documents", Err: err}
	}
	return filenames, nil
}

func (r *StaffDocumentRepository) ListDeletedStoredByStaffID(ctx context.Context, staffID int64, categories []string) ([]string, error) {
	if len(categories) == 0 {
		return []string{}, nil
	}
	var filenames []string
	query := base.GetDB(ctx, r.db).NewSelect().
		Model((*users.StaffDocument)(nil)).
		ModelTableExpr(`users.staff_documents AS "staff_document"`).
		ColumnExpr(`"staff_document".filename_stored`).
		Where(`"staff_document".staff_id = ?`, staffID).
		Where(`"staff_document".category IN (?)`, bun.List(categories)).
		Where(`"staff_document".deleted_at IS NOT NULL`).
		WhereAllWithDeleted()

	query = base.WithTenantFilter(ctx, query, "staff_document")
	if err := query.Scan(ctx, &filenames); err != nil {
		return nil, &modelBase.DatabaseError{Op: "list deleted stored staff documents", Err: err}
	}
	return filenames, nil
}

// SoftDelete stamps deleted_at and deleted_by in one update. The model's
// soft_delete tag then hides the row from every subsequent select.
func (r *StaffDocumentRepository) SoftDelete(ctx context.Context, doc *users.StaffDocument, deletedBy int64) error {
	now := time.Now()
	res, err := base.GetDB(ctx, r.db).NewUpdate().
		Model(doc).
		ModelTableExpr(`users.staff_documents AS "staff_document"`).
		Set("deleted_at = ?", now).
		Set("deleted_by = ?", deletedBy).
		Where(`"staff_document".id = ?`, doc.ID).
		Where(`"staff_document".deleted_at IS NULL`).
		Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{Op: "soft delete staff document", Err: err}
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return &modelBase.DatabaseError{Op: "soft delete staff document", Err: err}
	}
	if affected == 0 {
		return &modelBase.DatabaseError{Op: "soft delete staff document", Err: sql.ErrNoRows}
	}
	doc.DeletedAt = &now
	doc.DeletedBy = &deletedBy
	return nil
}
