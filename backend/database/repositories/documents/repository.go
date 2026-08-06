// Package documents implements the storage-agnostic metadata layer every
// document feature shares: the rows, their soft delete, and the durable
// cleanup intents that make an interrupted upload recoverable.
//
// It is generic over the concrete document type rather than polymorphic over
// a single table. Each owning domain keeps its own table with a real
// composite foreign key (tenant_id, owner_id), so the database still refuses
// a document that points at nothing, while the ~300 lines of query plumbing
// exist exactly once.
package documents

import (
	"context"
	"database/sql"
	"fmt"
	"reflect"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/documents"
	"github.com/uptrace/bun"
)

// Config describes one domain's document tables. Every identifier here ends
// up in SQL, so all values are compile-time constants owned by the domain's
// repository constructor, never request input.
type Config struct {
	// Table is the schema-qualified document table ("users.student_documents").
	Table string
	// Alias is the SQL alias used for that table ("student_document").
	Alias string
	// OwnerColumn names the owning entity's FK column ("student_id").
	OwnerColumn string
	// OwnerTable and OwnerAlias support the "owner was deleted" cleanup
	// sweep. OwnerTable empty disables that sweep for domains whose owner
	// cannot be soft-deleted.
	OwnerTable string
	OwnerAlias string
	// CleanupTable and CleanupAlias name the durable cleanup-intent table.
	CleanupTable string
	CleanupAlias string
}

// Repository is the generic document metadata repository.
type Repository[T documents.Entity, C documents.CleanupEntity] struct {
	db  *bun.DB
	cfg Config
}

// NewRepository wires a document repository for one domain.
func NewRepository[T documents.Entity, C documents.CleanupEntity](db *bun.DB, cfg Config) *Repository[T, C] {
	return &Repository[T, C]{db: db, cfg: cfg}
}

func (r *Repository[T, C]) tableExpr() string {
	return fmt.Sprintf(`%s AS "%s"`, r.cfg.Table, r.cfg.Alias)
}

func (r *Repository[T, C]) cleanupTableExpr() string {
	return fmt.Sprintf(`%s AS "%s"`, r.cfg.CleanupTable, r.cfg.CleanupAlias)
}

func (r *Repository[T, C]) col(column string) string {
	return fmt.Sprintf(`"%s".%s`, r.cfg.Alias, column)
}

func (r *Repository[T, C]) cleanupCol(column string) string {
	return fmt.Sprintf(`"%s".%s`, r.cfg.CleanupAlias, column)
}

// newEntity allocates a concrete instance of the pointer type T so bun has
// something to scan into.
func newEntity[E any]() E {
	entityType := reflect.TypeFor[E]()
	if entityType.Kind() == reflect.Pointer {
		entityType = entityType.Elem()
	}
	return reflect.New(entityType).Interface().(E)
}

// Create persists a document and hydrates database-generated values (ID and
// timestamps) used in the upload response.
func (r *Repository[T, C]) Create(ctx context.Context, document T) error {
	if err := document.Validate(); err != nil {
		return err
	}
	base.EnsureTenantID(ctx, document)
	if _, err := base.GetDB(ctx, r.db).NewInsert().
		Model(document).
		ModelTableExpr(r.tableExpr()).
		Returning("*").
		Exec(ctx); err != nil {
		return &modelBase.DatabaseError{Op: "create " + r.cfg.Alias, Err: err}
	}
	return nil
}

// FindForOwner loads one non-deleted document and enforces that it belongs to
// the given owner — the URL names both IDs and they must agree.
func (r *Repository[T, C]) FindForOwner(ctx context.Context, ownerID, documentID int64) (T, error) {
	return r.findForOwner(ctx, ownerID, documentID, false)
}

// FindForOwnerIncludingDeleted loads a document by the owner/document URL pair
// even after its metadata was soft-deleted, so filesystem cleanup can be
// retried without exposing unrelated records.
func (r *Repository[T, C]) FindForOwnerIncludingDeleted(ctx context.Context, ownerID, documentID int64) (T, error) {
	return r.findForOwner(ctx, ownerID, documentID, true)
}

func (r *Repository[T, C]) findForOwner(ctx context.Context, ownerID, documentID int64, includeDeleted bool) (T, error) {
	var zero T
	document := newEntity[T]()
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(document).
		ModelTableExpr(r.tableExpr()).
		Where(r.col("id")+" = ?", documentID).
		Where(r.col(r.cfg.OwnerColumn)+" = ?", ownerID)
	if includeDeleted {
		query = query.WhereAllWithDeleted()
	}
	query = base.WithTenantFilter(ctx, query, r.cfg.Alias)

	if err := query.Scan(ctx); err != nil {
		// sql.ErrNoRows stays wrapped so modelBase.IsNoRows classifies the
		// missing (or foreign) document as a 404.
		return zero, &modelBase.DatabaseError{Op: "find " + r.cfg.Alias, Err: err}
	}
	return document, nil
}

// ListByOwnerID returns the owner's non-deleted documents, newest first,
// restricted to the given categories. The category list is the caller's
// visibility set — empty means the caller may see no category at all, so the
// result is empty without touching the database.
func (r *Repository[T, C]) ListByOwnerID(ctx context.Context, ownerID int64, categories []string) ([]T, error) {
	if len(categories) == 0 {
		return []T{}, nil
	}

	var rows []T
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&rows).
		ModelTableExpr(r.tableExpr()).
		Where(r.col(r.cfg.OwnerColumn)+" = ?", ownerID).
		Where(r.col("category")+" IN (?)", bun.List(categories)).
		Order("created_at DESC", "id DESC")
	query = base.WithTenantFilter(ctx, query, r.cfg.Alias)

	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{Op: "list " + r.cfg.Alias, Err: err}
	}
	return rows, nil
}

// ListPendingFileCleanupByOwnerID returns documents whose stored bytes have
// not been removed yet, including soft-deleted rows, for owner teardown.
func (r *Repository[T, C]) ListPendingFileCleanupByOwnerID(ctx context.Context, ownerID int64) ([]T, error) {
	var rows []T
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&rows).
		ModelTableExpr(r.tableExpr()).
		Where(r.col(r.cfg.OwnerColumn)+" = ?", ownerID).
		Where(r.col("file_deleted_at") + " IS NULL").
		WhereAllWithDeleted()
	query = base.WithTenantFilter(ctx, query, r.cfg.Alias)

	if err := query.Scan(ctx, &rows); err != nil {
		return nil, &modelBase.DatabaseError{Op: "list pending " + r.cfg.Alias + " cleanup", Err: err}
	}
	return rows, nil
}

// ListOrphanedOwnerPendingFileCleanups returns every document whose owner is
// soft-deleted and whose stored bytes still need removal. Domains without a
// soft-deletable owner get an empty result.
func (r *Repository[T, C]) ListOrphanedOwnerPendingFileCleanups(ctx context.Context) ([]T, error) {
	if r.cfg.OwnerTable == "" {
		return []T{}, nil
	}
	var rows []T
	join := fmt.Sprintf(
		`JOIN %s AS "%s" ON "%s".tenant_id = "%s".tenant_id AND "%s".id = "%s".%s`,
		r.cfg.OwnerTable, r.cfg.OwnerAlias,
		r.cfg.OwnerAlias, r.cfg.Alias,
		r.cfg.OwnerAlias, r.cfg.Alias, r.cfg.OwnerColumn,
	)
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&rows).
		ModelTableExpr(r.tableExpr()).
		Join(join).
		Where(fmt.Sprintf(`"%s".deleted_at IS NOT NULL`, r.cfg.OwnerAlias)).
		Where(r.col("file_deleted_at") + " IS NULL").
		WhereAllWithDeleted()
	query = base.WithTenantFilter(ctx, query, r.cfg.Alias)

	if err := query.Scan(ctx, &rows); err != nil {
		return nil, &modelBase.DatabaseError{Op: "list orphaned " + r.cfg.Alias + " cleanup", Err: err}
	}
	return rows, nil
}

// ListDeletedPendingFileCleanups returns every soft-deleted document whose
// stored bytes still need removal, independent of owner lifecycle state.
func (r *Repository[T, C]) ListDeletedPendingFileCleanups(ctx context.Context) ([]T, error) {
	var rows []T
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&rows).
		ModelTableExpr(r.tableExpr()).
		Where(r.col("deleted_at") + " IS NOT NULL").
		Where(r.col("file_deleted_at") + " IS NULL").
		WhereAllWithDeleted()
	query = base.WithTenantFilter(ctx, query, r.cfg.Alias)

	if err := query.Scan(ctx, &rows); err != nil {
		return nil, &modelBase.DatabaseError{Op: "list deleted " + r.cfg.Alias + " cleanup", Err: err}
	}
	return rows, nil
}

// ListDeletedPendingFileCleanupByOwnerID limits retry candidates to
// soft-deleted documents in the caller's visible categories.
func (r *Repository[T, C]) ListDeletedPendingFileCleanupByOwnerID(ctx context.Context, ownerID int64, categories []string) ([]T, error) {
	if len(categories) == 0 {
		return []T{}, nil
	}
	var rows []T
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&rows).
		ModelTableExpr(r.tableExpr()).
		Where(r.col(r.cfg.OwnerColumn)+" = ?", ownerID).
		Where(r.col("category")+" IN (?)", bun.List(categories)).
		Where(r.col("deleted_at") + " IS NOT NULL").
		Where(r.col("file_deleted_at") + " IS NULL").
		WhereAllWithDeleted()
	query = base.WithTenantFilter(ctx, query, r.cfg.Alias)

	if err := query.Scan(ctx, &rows); err != nil {
		return nil, &modelBase.DatabaseError{Op: "list pending deleted " + r.cfg.Alias + " cleanup", Err: err}
	}
	return rows, nil
}

// SoftDelete stamps deleted_at and deleted_by in one update. The model's
// soft_delete tag then hides the row from every subsequent select.
func (r *Repository[T, C]) SoftDelete(ctx context.Context, document T, deletedBy int64) error {
	now := time.Now()
	res, err := base.GetDB(ctx, r.db).NewUpdate().
		Model(document).
		ModelTableExpr(r.tableExpr()).
		Set("deleted_at = ?", now).
		Set("deleted_by = ?", deletedBy).
		Where(r.col("id")+" = ?", document.GetID()).
		Where(r.col("deleted_at") + " IS NULL").
		Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{Op: "soft delete " + r.cfg.Alias, Err: err}
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return &modelBase.DatabaseError{Op: "soft delete " + r.cfg.Alias, Err: err}
	}
	if affected == 0 {
		return &modelBase.DatabaseError{Op: "soft delete " + r.cfg.Alias, Err: sql.ErrNoRows}
	}
	document.MarkSoftDeleted(now, deletedBy)
	return nil
}

// MarkFileDeleted records that the stored bytes are gone so cleanup is never
// retried for this document again.
func (r *Repository[T, C]) MarkFileDeleted(ctx context.Context, documentID int64) error {
	now := time.Now()
	query := base.GetDB(ctx, r.db).NewUpdate().
		Model(newEntity[T]()).
		ModelTableExpr(r.tableExpr()).
		Set("file_deleted_at = ?", now).
		Where(r.col("id")+" = ?", documentID).
		Where(r.col("file_deleted_at") + " IS NULL").
		WhereAllWithDeleted()
	query = base.WithTenantFilter(ctx, query, r.cfg.Alias)
	if _, err := query.Exec(ctx); err != nil {
		return &modelBase.DatabaseError{Op: "mark " + r.cfg.Alias + " file deleted", Err: err}
	}
	return nil
}

// QueueFileCleanup durably records the intent to remove an object before it
// is written.
//
// A settled intent is REVIVED rather than left alone, and that is the whole
// point of the conflict clause. Settling an intent is an update (cleaned_at is
// stamped, the row stays), so every object that was ever uploaded leaves a
// row behind under its stored name. Queueing again for that name — which is
// exactly what deleting the owner does, since the document rows cascade away
// and the intents are all that survive — would otherwise hit the unique index,
// do nothing, and strand the bytes on disk with nothing left pointing at them.
func (r *Repository[T, C]) QueueFileCleanup(ctx context.Context, cleanup C) error {
	base.EnsureTenantID(ctx, cleanup)
	if _, err := base.GetDB(ctx, r.db).NewInsert().
		Model(cleanup).
		ModelTableExpr(r.cleanupTableExpr()).
		On("CONFLICT (tenant_id, filename_stored) DO UPDATE").
		Set("cleaned_at = NULL").
		Set("retry_after = EXCLUDED.retry_after").
		Set("owner_id = EXCLUDED.owner_id").
		Exec(ctx); err != nil {
		return &modelBase.DatabaseError{Op: "queue " + r.cfg.Alias + " file cleanup", Err: err}
	}
	return nil
}

// ListQueuedFileCleanupByOwnerID returns eligible orphan rows for one owner.
func (r *Repository[T, C]) ListQueuedFileCleanupByOwnerID(ctx context.Context, ownerID int64) ([]C, error) {
	return r.listQueuedFileCleanups(ctx, r.cleanupCol("owner_id")+" = ?", ownerID)
}

// ListQueuedFileCleanups returns every eligible orphan row in the tenant.
func (r *Repository[T, C]) ListQueuedFileCleanups(ctx context.Context) ([]C, error) {
	return r.listQueuedFileCleanups(ctx, "", nil)
}

// listQueuedFileCleanups returns eligible orphan rows and locks them for the
// caller's transaction. FOR UPDATE SKIP LOCKED is load-bearing: an upload
// whose metadata transaction has already stamped cleaned_at but not yet
// committed holds that row lock, and skipping it keeps this pass from
// deleting the bytes of a document that is about to exist. Concurrent cleanup
// passes skip each other's rows for the same reason.
func (r *Repository[T, C]) listQueuedFileCleanups(ctx context.Context, where string, arg any) ([]C, error) {
	var cleanups []C
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&cleanups).
		ModelTableExpr(r.cleanupTableExpr()).
		Where(r.cleanupCol("retry_after")+" <= ?", time.Now()).
		Where(r.cleanupCol("cleaned_at") + " IS NULL").
		For("UPDATE SKIP LOCKED")
	if where != "" {
		query = query.Where(where, arg)
	}
	query = base.WithTenantFilter(ctx, query, r.cfg.CleanupAlias)
	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{Op: "list queued " + r.cfg.Alias + " file cleanup", Err: err}
	}
	return cleanups, nil
}

// MarkQueuedFileCleanupComplete settles one intent by ID.
func (r *Repository[T, C]) MarkQueuedFileCleanupComplete(ctx context.Context, cleanupID int64) error {
	return r.markQueuedFileCleanupComplete(ctx, r.cleanupCol("id")+" = ?", cleanupID)
}

// MarkQueuedFileCleanupCompleteByFilename settles the intent belonging to one
// stored object, which is how a successful upload retires its own intent.
func (r *Repository[T, C]) MarkQueuedFileCleanupCompleteByFilename(ctx context.Context, filename string) error {
	return r.markQueuedFileCleanupComplete(ctx, r.cleanupCol("filename_stored")+" = ?", filename)
}

// ActivateQueuedFileCleanupByFilename makes a failed upload eligible for retry
// immediately after its inline cleanup failed.
func (r *Repository[T, C]) ActivateQueuedFileCleanupByFilename(ctx context.Context, filename string) error {
	query := base.GetDB(ctx, r.db).NewUpdate().
		Model(newEntity[C]()).
		ModelTableExpr(r.cleanupTableExpr()).
		Set("retry_after = ?", time.Now()).
		Where(r.cleanupCol("filename_stored")+" = ?", filename).
		Where(r.cleanupCol("cleaned_at") + " IS NULL")
	query = base.WithTenantFilter(ctx, query, r.cfg.CleanupAlias)
	if _, err := query.Exec(ctx); err != nil {
		return &modelBase.DatabaseError{Op: "activate queued " + r.cfg.Alias + " file cleanup", Err: err}
	}
	return nil
}

func (r *Repository[T, C]) markQueuedFileCleanupComplete(ctx context.Context, where string, arg any) error {
	now := time.Now()
	query := base.GetDB(ctx, r.db).NewUpdate().
		Model(newEntity[C]()).
		ModelTableExpr(r.cleanupTableExpr()).
		Set("cleaned_at = ?", now).
		Where(where, arg).
		Where(r.cleanupCol("cleaned_at") + " IS NULL")
	query = base.WithTenantFilter(ctx, query, r.cfg.CleanupAlias)
	if _, err := query.Exec(ctx); err != nil {
		return &modelBase.DatabaseError{Op: "mark queued " + r.cfg.Alias + " file cleanup complete", Err: err}
	}
	return nil
}
