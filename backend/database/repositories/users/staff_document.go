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

func (r *StaffDocumentRepository) ListPendingFileCleanupByStaffID(ctx context.Context, staffID int64) ([]*users.StaffDocument, error) {
	var documents []*users.StaffDocument
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&documents).
		ModelTableExpr(`users.staff_documents AS "staff_document"`).
		Where(`"staff_document".staff_id = ?`, staffID).
		Where(`"staff_document".file_deleted_at IS NULL`).
		WhereAllWithDeleted()

	query = base.WithTenantFilter(ctx, query, "staff_document")
	if err := query.Scan(ctx, &documents); err != nil {
		return nil, &modelBase.DatabaseError{Op: "list pending staff document cleanup", Err: err}
	}
	return documents, nil
}

func (r *StaffDocumentRepository) ListDeletedPendingFileCleanupByStaffID(ctx context.Context, staffID int64, categories []string) ([]*users.StaffDocument, error) {
	if len(categories) == 0 {
		return []*users.StaffDocument{}, nil
	}
	var documents []*users.StaffDocument
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&documents).
		ModelTableExpr(`users.staff_documents AS "staff_document"`).
		Where(`"staff_document".staff_id = ?`, staffID).
		Where(`"staff_document".category IN (?)`, bun.List(categories)).
		Where(`"staff_document".deleted_at IS NOT NULL`).
		Where(`"staff_document".file_deleted_at IS NULL`).
		WhereAllWithDeleted()

	query = base.WithTenantFilter(ctx, query, "staff_document")
	if err := query.Scan(ctx, &documents); err != nil {
		return nil, &modelBase.DatabaseError{Op: "list pending deleted staff document cleanup", Err: err}
	}
	return documents, nil
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

func (r *StaffDocumentRepository) MarkFileDeleted(ctx context.Context, documentID int64) error {
	now := time.Now()
	query := base.GetDB(ctx, r.db).NewUpdate().
		Model((*users.StaffDocument)(nil)).
		ModelTableExpr(`users.staff_documents AS "staff_document"`).
		Set("file_deleted_at = ?", now).
		Where(`"staff_document".id = ?`, documentID).
		Where(`"staff_document".file_deleted_at IS NULL`)
	query = query.WhereAllWithDeleted()
	query = base.WithTenantFilter(ctx, query, "staff_document")
	if _, err := query.Exec(ctx); err != nil {
		return &modelBase.DatabaseError{Op: "mark staff document file deleted", Err: err}
	}
	return nil
}

func (r *StaffDocumentRepository) QueueFileCleanup(ctx context.Context, cleanup *users.StaffDocumentFileCleanup) error {
	base.EnsureTenantID(ctx, cleanup)
	_, err := base.GetDB(ctx, r.db).NewInsert().
		Model(cleanup).
		ModelTableExpr(`users.staff_document_file_cleanup AS "staff_document_file_cleanup"`).
		On(`CONFLICT (tenant_id, filename_stored) DO NOTHING`).
		Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{Op: "queue staff document file cleanup", Err: err}
	}
	return nil
}

func (r *StaffDocumentRepository) ListQueuedFileCleanupByStaffID(ctx context.Context, staffID int64) ([]*users.StaffDocumentFileCleanup, error) {
	return r.listQueuedFileCleanups(ctx, `"staff_document_file_cleanup".staff_id = ?`, staffID)
}

func (r *StaffDocumentRepository) ListQueuedFileCleanups(ctx context.Context) ([]*users.StaffDocumentFileCleanup, error) {
	return r.listQueuedFileCleanups(ctx, "", nil)
}

func (r *StaffDocumentRepository) listQueuedFileCleanups(ctx context.Context, where string, arg any) ([]*users.StaffDocumentFileCleanup, error) {
	var cleanups []*users.StaffDocumentFileCleanup
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&cleanups).
		ModelTableExpr(`users.staff_document_file_cleanup AS "staff_document_file_cleanup"`).
		Where(`"staff_document_file_cleanup".cleaned_at IS NULL`)
	if where != "" {
		query = query.Where(where, arg)
	}
	query = base.WithTenantFilter(ctx, query, "staff_document_file_cleanup")
	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{Op: "list queued staff document file cleanup", Err: err}
	}
	return cleanups, nil
}

func (r *StaffDocumentRepository) MarkQueuedFileCleanupComplete(ctx context.Context, cleanupID int64) error {
	now := time.Now()
	query := base.GetDB(ctx, r.db).NewUpdate().
		Model((*users.StaffDocumentFileCleanup)(nil)).
		ModelTableExpr(`users.staff_document_file_cleanup AS "staff_document_file_cleanup"`).
		Set("cleaned_at = ?", now).
		Where(`"staff_document_file_cleanup".id = ?`, cleanupID).
		Where(`"staff_document_file_cleanup".cleaned_at IS NULL`)
	query = base.WithTenantFilter(ctx, query, "staff_document_file_cleanup")
	if _, err := query.Exec(ctx); err != nil {
		return &modelBase.DatabaseError{Op: "mark queued staff document file cleanup complete", Err: err}
	}
	return nil
}
