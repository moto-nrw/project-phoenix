package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/filestorage/internal/domain"
	"github.com/uptrace/bun"
)

// documentColumns is the shape both document tables share. The owner column
// stays on the concrete row, because a document belongs to its owner through
// a real composite foreign key (tenant_id, owner_id) the database enforces.
type documentColumns struct {
	ID              int64      `bun:"id,pk,autoincrement"`
	TenantID        int64      `bun:"tenant_id,notnull"`
	Category        string     `bun:"category,notnull"`
	FilenameDisplay string     `bun:"filename_display,notnull"`
	FilenameStored  string     `bun:"filename_stored,notnull"`
	SizeBytes       int64      `bun:"size_bytes,notnull"`
	ContentType     string     `bun:"content_type,notnull"`
	UploadedBy      int64      `bun:"uploaded_by,notnull"`
	DeletedAt       *time.Time `bun:"deleted_at"`
	DeletedBy       *int64     `bun:"deleted_by"`
	FileDeletedAt   *time.Time `bun:"file_deleted_at"`
	CreatedAt       time.Time  `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt       time.Time  `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
}

func (c documentColumns) document(ownerID int64) domain.Document {
	return domain.Document{
		ID: c.ID, TenantID: c.TenantID, OwnerID: ownerID, Category: c.Category,
		FilenameDisplay: c.FilenameDisplay, FilenameStored: c.FilenameStored,
		SizeBytes: c.SizeBytes, ContentType: c.ContentType, UploadedBy: c.UploadedBy,
		CreatedAt: c.CreatedAt, DeletedAt: c.DeletedAt, DeletedBy: c.DeletedBy, FileDeletedAt: c.FileDeletedAt,
	}
}

func newDocumentColumns(tenantID int64, document domain.NewDocument) documentColumns {
	return documentColumns{
		TenantID: tenantID, Category: document.Category,
		FilenameDisplay: document.FilenameDisplay, FilenameStored: document.FilenameStored,
		SizeBytes: document.SizeBytes, ContentType: document.ContentType, UploadedBy: document.UploadedBy,
	}
}

type fileRow struct {
	bun.BaseModel `bun:"table:documents.files,alias:document"`
	documentColumns
	FolderID int64 `bun:"folder_id,notnull"`
}

func (r *fileRow) document() domain.Document { return r.documentColumns.document(r.FolderID) }

type attachmentRow struct {
	bun.BaseModel `bun:"table:documents.announcement_attachments,alias:document"`
	documentColumns
	AnnouncementID int64 `bun:"announcement_id,notnull"`
}

func (r *attachmentRow) document() domain.Document {
	return r.documentColumns.document(r.AnnouncementID)
}

type documentRow interface {
	*fileRow | *attachmentRow
	document() domain.Document
}

// documentTable holds the per-table query starters. Every table name and
// owner column is a constant here so the ownership check can resolve it; the
// shared predicates below only name the "document" alias.
type documentTable[R documentRow] struct {
	name       string
	selectRows func(bun.IDB, *[]R) *bun.SelectQuery
	selectNone func(bun.IDB) *bun.SelectQuery
	insertRow  func(bun.IDB, R) *bun.InsertQuery
	updateRows func(bun.IDB) *bun.UpdateQuery
	whereOwner func(*bun.SelectQuery, int64) *bun.SelectQuery
	newRow     func(tenantID int64, document domain.NewDocument) R
}

var filesTable = documentTable[*fileRow]{
	name: "files",
	selectRows: func(db bun.IDB, rows *[]*fileRow) *bun.SelectQuery {
		return db.NewSelect().Model(rows).ModelTableExpr(`documents.files AS "document"`)
	},
	selectNone: func(db bun.IDB) *bun.SelectQuery {
		return db.NewSelect().Model((*fileRow)(nil)).ModelTableExpr(`documents.files AS "document"`)
	},
	insertRow: func(db bun.IDB, row *fileRow) *bun.InsertQuery {
		return db.NewInsert().Model(row).ModelTableExpr(`documents.files`)
	},
	updateRows: func(db bun.IDB) *bun.UpdateQuery {
		return db.NewUpdate().Model((*fileRow)(nil)).ModelTableExpr(`documents.files AS "document"`)
	},
	whereOwner: func(query *bun.SelectQuery, ownerID int64) *bun.SelectQuery {
		return query.Where(`"document".folder_id = ?`, ownerID)
	},
	newRow: func(tenantID int64, document domain.NewDocument) *fileRow {
		return &fileRow{documentColumns: newDocumentColumns(tenantID, document), FolderID: document.OwnerID}
	},
}

var attachmentsTable = documentTable[*attachmentRow]{
	name: "announcement attachments",
	selectRows: func(db bun.IDB, rows *[]*attachmentRow) *bun.SelectQuery {
		return db.NewSelect().Model(rows).ModelTableExpr(`documents.announcement_attachments AS "document"`)
	},
	selectNone: func(db bun.IDB) *bun.SelectQuery {
		return db.NewSelect().Model((*attachmentRow)(nil)).ModelTableExpr(`documents.announcement_attachments AS "document"`)
	},
	insertRow: func(db bun.IDB, row *attachmentRow) *bun.InsertQuery {
		return db.NewInsert().Model(row).ModelTableExpr(`documents.announcement_attachments`)
	},
	updateRows: func(db bun.IDB) *bun.UpdateQuery {
		return db.NewUpdate().Model((*attachmentRow)(nil)).ModelTableExpr(`documents.announcement_attachments AS "document"`)
	},
	whereOwner: func(query *bun.SelectQuery, ownerID int64) *bun.SelectQuery {
		return query.Where(`"document".announcement_id = ?`, ownerID)
	},
	newRow: func(tenantID int64, document domain.NewDocument) *attachmentRow {
		return &attachmentRow{documentColumns: newDocumentColumns(tenantID, document), AnnouncementID: document.OwnerID}
	},
}

// DocumentStore implements ports.DocumentStore over one document table.
type DocumentStore[R documentRow] struct {
	database Database
	table    documentTable[R]
}

// NewFileStore persists documents.files.
func NewFileStore(database Database) *DocumentStore[*fileRow] {
	return newDocumentStore(database, filesTable)
}

// NewAttachmentStore persists documents.announcement_attachments.
func NewAttachmentStore(database Database) *DocumentStore[*attachmentRow] {
	return newDocumentStore(database, attachmentsTable)
}

func newDocumentStore[R documentRow](database Database, table documentTable[R]) *DocumentStore[R] {
	if database == nil {
		panic("file storage postgres: database runtime is required")
	}
	return &DocumentStore[R]{database: database, table: table}
}

func (s *DocumentStore[R]) Create(ctx context.Context, document domain.NewDocument) (domain.Document, domain.OperationStats, error) {
	db, tenantID, err := requireTenant(s.database, ctx)
	if err != nil {
		return domain.Document{}, domain.OperationStats{}, err
	}
	if err := document.Validate(); err != nil {
		return domain.Document{}, domain.OperationStats{}, fmt.Errorf("%w: %s", domain.ErrInvalid, err.Error())
	}
	row := s.table.newRow(tenantID, document)
	started := time.Now()
	err = s.table.insertRow(db, row).Returning("*").Scan(ctx)
	stats := measure(started, 1)
	if err != nil {
		return domain.Document{}, stats, fmt.Errorf("file storage postgres: create %s: %w", s.table.name, err)
	}
	return row.document(), stats, nil
}

func (s *DocumentStore[R]) FindForOwner(ctx context.Context, ownerID, documentID int64, includeDeleted bool) (domain.Document, bool, domain.OperationStats, error) {
	db, tenantID, err := requireTenant(s.database, ctx)
	if err != nil {
		return domain.Document{}, false, domain.OperationStats{}, err
	}
	var rows []R
	query := s.table.whereOwner(s.table.selectRows(db, &rows), ownerID).
		Where(`"document".id = ?`, documentID).
		Where(`"document".tenant_id = ?`, tenantID)
	if !includeDeleted {
		query = query.Where(`"document".deleted_at IS NULL`)
	}
	started := time.Now()
	err = query.Scan(ctx)
	stats := measure(started, int64(len(rows)))
	if err != nil {
		return domain.Document{}, false, stats, fmt.Errorf("file storage postgres: find %s: %w", s.table.name, err)
	}
	if len(rows) == 0 {
		return domain.Document{}, false, stats, nil
	}
	return rows[0].document(), true, stats, nil
}

func (s *DocumentStore[R]) ListByOwner(ctx context.Context, ownerID int64) ([]domain.Document, domain.OperationStats, error) {
	return s.list(ctx, "list", func(query *bun.SelectQuery) *bun.SelectQuery {
		return s.table.whereOwner(query, ownerID).
			Where(`"document".deleted_at IS NULL`).
			OrderExpr(`"document".created_at DESC, "document".id DESC`)
	})
}

func (s *DocumentStore[R]) ListPendingCleanupByOwner(ctx context.Context, ownerID int64) ([]domain.Document, domain.OperationStats, error) {
	return s.list(ctx, "list pending cleanup", func(query *bun.SelectQuery) *bun.SelectQuery {
		return s.table.whereOwner(query, ownerID).
			Where(`"document".file_deleted_at IS NULL`).
			OrderExpr(`"document".id ASC`)
	})
}

func (s *DocumentStore[R]) ListDeletedPendingCleanup(ctx context.Context) ([]domain.Document, domain.OperationStats, error) {
	return s.list(ctx, "list deleted pending cleanup", func(query *bun.SelectQuery) *bun.SelectQuery {
		return query.
			Where(`"document".deleted_at IS NOT NULL`).
			Where(`"document".file_deleted_at IS NULL`).
			OrderExpr(`"document".id ASC`).
			Limit(domain.CleanupBatchSize)
	})
}

func (s *DocumentStore[R]) ListDeletedPendingCleanupByOwner(ctx context.Context, ownerID int64) ([]domain.Document, domain.OperationStats, error) {
	return s.list(ctx, "list deleted pending cleanup by owner", func(query *bun.SelectQuery) *bun.SelectQuery {
		return s.table.whereOwner(query, ownerID).
			Where(`"document".deleted_at IS NOT NULL`).
			Where(`"document".file_deleted_at IS NULL`).
			OrderExpr(`"document".id ASC`).
			Limit(domain.RequestCleanupRetryLimit)
	})
}

func (s *DocumentStore[R]) list(ctx context.Context, operation string, narrow func(*bun.SelectQuery) *bun.SelectQuery) ([]domain.Document, domain.OperationStats, error) {
	db, tenantID, err := requireTenant(s.database, ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	var rows []R
	query := narrow(s.table.selectRows(db, &rows)).Where(`"document".tenant_id = ?`, tenantID)
	started := time.Now()
	err = query.Scan(ctx)
	stats := measure(started, int64(len(rows)))
	if err != nil {
		return nil, stats, fmt.Errorf("file storage postgres: %s %s: %w", operation, s.table.name, err)
	}
	result := make([]domain.Document, 0, len(rows))
	for _, row := range rows {
		result = append(result, row.document())
	}
	return result, stats, nil
}

// SoftDelete stamps deleted_at and deleted_by in one update on a live row.
func (s *DocumentStore[R]) SoftDelete(ctx context.Context, documentID, deletedBy int64) (domain.Document, domain.OperationStats, error) {
	db, tenantID, err := requireTenant(s.database, ctx)
	if err != nil {
		return domain.Document{}, domain.OperationStats{}, err
	}
	var rows []R
	started := time.Now()
	err = s.table.updateRows(db).
		Set(`deleted_at = ?`, time.Now()).
		Set(`deleted_by = ?`, deletedBy).
		Where(`"document".id = ?`, documentID).
		Where(`"document".tenant_id = ?`, tenantID).
		Where(`"document".deleted_at IS NULL`).
		Returning("*").
		Scan(ctx, &rows)
	stats := measure(started, int64(len(rows)))
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return domain.Document{}, stats, fmt.Errorf("file storage postgres: soft delete %s: %w", s.table.name, err)
	}
	if len(rows) == 0 {
		return domain.Document{}, stats, domain.ErrNotFound
	}
	return rows[0].document(), stats, nil
}

// MarkBytesDeleted records that the stored object is gone so cleanup is never
// retried for this document again.
func (s *DocumentStore[R]) MarkBytesDeleted(ctx context.Context, documentID int64) (domain.OperationStats, error) {
	db, tenantID, err := requireTenant(s.database, ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	started := time.Now()
	result, err := s.table.updateRows(db).
		Set(`file_deleted_at = ?`, time.Now()).
		Where(`"document".id = ?`, documentID).
		Where(`"document".tenant_id = ?`, tenantID).
		Where(`"document".file_deleted_at IS NULL`).
		Exec(ctx)
	stats := measure(started, 0)
	if err != nil {
		return stats, fmt.Errorf("file storage postgres: mark %s bytes deleted: %w", s.table.name, err)
	}
	if affected, err := result.RowsAffected(); err == nil {
		stats.Rows = affected
	}
	return stats, nil
}

func (s *DocumentStore[R]) CountByOwner(ctx context.Context, ownerID int64) (int, domain.OperationStats, error) {
	db, tenantID, err := requireTenant(s.database, ctx)
	if err != nil {
		return 0, domain.OperationStats{}, err
	}
	query := s.table.whereOwner(s.table.selectNone(db), ownerID).
		Where(`"document".tenant_id = ?`, tenantID).
		Where(`"document".deleted_at IS NULL`)
	started := time.Now()
	count, err := query.Count(ctx)
	stats := measure(started, 1)
	if err != nil {
		return 0, stats, fmt.Errorf("file storage postgres: count %s: %w", s.table.name, err)
	}
	return count, stats, nil
}

// TotalStoredBytes sums every document whose bytes still occupy the storage
// backend. Soft-deleted rows count until the sweep has removed their object.
func (s *DocumentStore[R]) TotalStoredBytes(ctx context.Context) (int64, domain.OperationStats, error) {
	db, tenantID, err := requireTenant(s.database, ctx)
	if err != nil {
		return 0, domain.OperationStats{}, err
	}
	var total sql.NullInt64
	query := s.table.selectNone(db).
		ColumnExpr(`COALESCE(SUM("document".size_bytes), 0)`).
		Where(`"document".tenant_id = ?`, tenantID).
		Where(`"document".file_deleted_at IS NULL`)
	started := time.Now()
	err = query.Scan(ctx, &total)
	stats := measure(started, 1)
	if err != nil {
		return 0, stats, fmt.Errorf("file storage postgres: sum stored %s bytes: %w", s.table.name, err)
	}
	return total.Int64, stats, nil
}
