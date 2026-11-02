package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/domain"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/driver/pgdriver"
)

const (
	careDocumentCleanupBatchSize    = careplan.StudentDocumentSweepBatchSize
	careDocumentRequestCleanupLimit = 10
)

type careDocumentRow struct {
	bun.BaseModel   `bun:"table:student_documents,alias:care_document"`
	ID              int64      `bun:"id,pk,autoincrement"`
	TenantID        int64      `bun:"tenant_id,notnull"`
	CreatedAt       time.Time  `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt       time.Time  `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
	StudentID       int64      `bun:"student_id,notnull"`
	Category        string     `bun:"category,notnull"`
	FilenameDisplay string     `bun:"filename_display,notnull"`
	FilenameStored  string     `bun:"filename_stored,nullzero"`
	SizeBytes       int64      `bun:"size_bytes,notnull"`
	ContentType     string     `bun:"content_type,notnull"`
	UploadedBy      int64      `bun:"uploaded_by,notnull"`
	DeletedAt       *time.Time `bun:"deleted_at,soft_delete,nullzero"`
	DeletedBy       *int64     `bun:"deleted_by"`
	FileDeletedAt   *time.Time `bun:"file_deleted_at"`
	// The accident report of an unfallanzeige row (#4771).
	AccidentReport       json.RawMessage `bun:"accident_report,type:jsonb,nullzero"`
	AccidentSupersedesID *int64          `bun:"accident_supersedes_id"`
	AccidentSignatureID  *int64          `bun:"accident_signature_id"`
	AccidentSignedAt     *time.Time      `bun:"accident_signed_at"`
	AccidentSignedBy     *int64          `bun:"accident_signed_by"`
}

type careDocumentCleanupRow struct {
	bun.BaseModel  `bun:"table:student_document_file_cleanup,alias:care_document_cleanup"`
	ID             int64      `bun:"id,pk,autoincrement"`
	TenantID       int64      `bun:"tenant_id,notnull"`
	CreatedAt      time.Time  `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt      time.Time  `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
	OwnerID        int64      `bun:"owner_id,notnull"`
	FilenameStored string     `bun:"filename_stored,notnull"`
	RetryAfter     time.Time  `bun:"retry_after,notnull"`
	CleanedAt      *time.Time `bun:"cleaned_at"`
}

func (s *Store) FindCareDocument(ctx context.Context, studentID, documentID int64, includeDeleted bool) (domain.CareDocument, bool, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.CareDocument{}, false, domain.OperationStats{}, err
	}
	row := new(careDocumentRow)
	query := db.NewSelect().Model(row).ModelTableExpr(`users.student_documents AS "care_document"`).
		Where(`"care_document".id = ?`, documentID).
		Where(`"care_document".student_id = ?`, studentID)
	if includeDeleted {
		query = query.WhereAllWithDeleted()
	}
	query = withTenant(query, "care_document", tenantID)
	found, stats, err := scanOne(ctx, query, "find care document")
	if err != nil || !found {
		return domain.CareDocument{}, found, stats, err
	}
	value, err := careDocumentToDomain(*row)
	return value, err == nil, stats, err
}

func (s *Store) ListCareDocuments(ctx context.Context, studentID int64, categories []string) ([]domain.CareDocument, domain.OperationStats, error) {
	if len(categories) == 0 {
		return []domain.CareDocument{}, domain.OperationStats{}, nil
	}
	return s.listCareDocuments(ctx, studentID, categories, false, false, 0)
}

func (s *Store) ListPendingCareDocumentCleanup(ctx context.Context, studentID int64) ([]domain.CareDocument, domain.OperationStats, error) {
	return s.listCareDocuments(ctx, studentID, nil, true, false, 0)
}

func (s *Store) ListDeletedCareDocuments(ctx context.Context, studentID int64, categories []string) ([]domain.CareDocument, domain.OperationStats, error) {
	if studentID > 0 && len(categories) == 0 {
		return []domain.CareDocument{}, domain.OperationStats{}, nil
	}
	limit := careDocumentCleanupBatchSize
	if studentID > 0 {
		limit = careDocumentRequestCleanupLimit
	}
	return s.listCareDocuments(ctx, studentID, categories, false, true, limit)
}

func (s *Store) listCareDocuments(ctx context.Context, studentID int64, categories []string, pendingFile, deletedOnly bool, limit int) ([]domain.CareDocument, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	rows := []careDocumentRow{}
	query := db.NewSelect().Model(&rows).ModelTableExpr(`users.student_documents AS "care_document"`)
	if studentID > 0 {
		query = query.Where(`"care_document".student_id = ?`, studentID)
	}
	if len(categories) > 0 {
		query = query.Where(`"care_document".category IN (?)`, bun.List(categories))
	}
	if pendingFile || deletedOnly {
		// An accident report without a paper copy has no object to remove.
		query = query.Where(`"care_document".file_deleted_at IS NULL`).
			Where(`"care_document".filename_stored IS NOT NULL`).WhereAllWithDeleted()
	}
	if deletedOnly {
		query = query.Where(`"care_document".deleted_at IS NOT NULL`).OrderExpr(`"care_document".id ASC`)
	} else if !pendingFile {
		query = query.OrderExpr(`"care_document".created_at DESC, "care_document".id DESC`)
	}
	if limit > 0 {
		query = query.Limit(int64(limit))
	}
	query = withTenant(query, "care_document", tenantID)
	stats, err := scanAll(ctx, query, "list care documents")
	if err != nil {
		return nil, stats, err
	}
	result := make([]domain.CareDocument, 0, len(rows))
	for _, row := range rows {
		value, err := careDocumentToDomain(row)
		if err != nil {
			return nil, stats, err
		}
		result = append(result, value)
	}
	stats.Rows = int64(len(result))
	return result, stats, nil
}

func (s *Store) ListCareDocumentCleanups(ctx context.Context, studentID *int64) ([]domain.CareDocumentCleanup, domain.OperationStats, error) {
	db, tenantID, err := s.databaseForWrite(ctx, "list care document cleanups")
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	rows := []careDocumentCleanupRow{}
	query := db.NewSelect().Model(&rows).ModelTableExpr(`users.student_document_file_cleanup AS "care_document_cleanup"`).
		Where(`"care_document_cleanup".retry_after <= ?`, time.Now()).
		Where(`"care_document_cleanup".cleaned_at IS NULL`).
		Where(`"care_document_cleanup".tenant_id = ?`, tenantID).
		OrderExpr(`"care_document_cleanup".id ASC`).
		Limit(careDocumentCleanupBatchSize).
		For("UPDATE SKIP LOCKED")
	if studentID != nil {
		query = query.Where(`"care_document_cleanup".owner_id = ?`, *studentID)
	}
	stats, err := scanAll(ctx, query, "list care document cleanups")
	if err != nil {
		return nil, stats, err
	}
	result := make([]domain.CareDocumentCleanup, 0, len(rows))
	for _, row := range rows {
		result = append(result, careDocumentCleanupToDomain(row))
	}
	stats.Rows = int64(len(result))
	return result, stats, nil
}

func (s *Store) CreateCareDocument(ctx context.Context, value domain.CareDocument) (domain.CareDocument, domain.OperationStats, error) {
	db, tenantID, err := s.databaseForWrite(ctx, "create care document")
	if err != nil {
		return domain.CareDocument{}, domain.OperationStats{}, err
	}
	row, err := careDocumentFromDomain(value)
	if err != nil {
		return domain.CareDocument{}, domain.OperationStats{}, err
	}
	row.TenantID = tenantID
	stats, err := execAny(ctx, db.NewInsert().Model(&row).
		ModelTableExpr(`users.student_documents AS "care_document"`).Returning("*"), "create care document")
	if isAccidentSuccessorConflict(err) {
		return domain.CareDocument{}, stats, fmt.Errorf("%w: %v", careplan.ErrAccidentReportSuperseded, err)
	}
	if err != nil {
		return domain.CareDocument{}, stats, err
	}
	value, decodeErr := careDocumentToDomain(row)
	return value, stats, decodeErr
}

// UpdateOpenCareAccidentReport writes the report, signature and paper copy of
// a report that is still open. The signed and deleted guard is in the
// statement, so two signatures racing each other leave exactly one; the
// loser matches no row. A trigger refuses any change of a signed report
// independently of this guard.
func (s *Store) UpdateOpenCareAccidentReport(ctx context.Context, value domain.CareDocument) (domain.CareDocument, domain.OperationStats, error) {
	db, tenantID, err := s.databaseForWrite(ctx, "update open care accident report")
	if err != nil {
		return domain.CareDocument{}, domain.OperationStats{}, err
	}
	row, err := careDocumentFromDomain(value)
	if err != nil {
		return domain.CareDocument{}, domain.OperationStats{}, err
	}
	row.TenantID = tenantID
	stats, err := execGuarded(ctx, db.NewUpdate().Model(&row).
		ModelTableExpr(`users.student_documents AS "care_document"`).
		Column("accident_report", "accident_signature_id", "accident_signed_at", "accident_signed_by",
			"filename_display", "filename_stored", "size_bytes", "content_type").
		Where(`"care_document".id = ?`, value.ID).
		Where(`"care_document".student_id = ?`, value.StudentID).
		Where(`"care_document".accident_report IS NOT NULL`).
		Where(`"care_document".accident_signed_at IS NULL`).
		Where(`"care_document".deleted_at IS NULL`).
		Where(`"care_document".tenant_id = ?`, tenantID).
		Returning("*"), "update open care accident report", careplan.ErrAccidentReportSigned)
	if err != nil {
		return domain.CareDocument{}, stats, err
	}
	value, decodeErr := careDocumentToDomain(row)
	return value, stats, decodeErr
}

func (s *Store) SoftDeleteCareDocument(ctx context.Context, documentID, deletedBy int64) (time.Time, domain.OperationStats, error) {
	db, tenantID, err := s.databaseForWrite(ctx, "soft delete care document")
	if err != nil {
		return time.Time{}, domain.OperationStats{}, err
	}
	now := time.Now()
	stats, err := execGuarded(ctx, db.NewUpdate().Model((*careDocumentRow)(nil)).
		ModelTableExpr(`users.student_documents AS "care_document"`).
		Set("deleted_at = ?", now).Set("deleted_by = ?", deletedBy).
		Where(`"care_document".id = ?`, documentID).
		Where(`"care_document".deleted_at IS NULL`).
		Where(`"care_document".tenant_id = ?`, tenantID), "soft delete care document", careplan.ErrCareDocumentNotFound)
	return now, stats, err
}

func (s *Store) MarkCareDocumentFileDeleted(ctx context.Context, documentID int64) (domain.OperationStats, error) {
	db, tenantID, err := s.databaseForWrite(ctx, "mark care document file deleted")
	if err != nil {
		return domain.OperationStats{}, err
	}
	return execAny(ctx, db.NewUpdate().Model((*careDocumentRow)(nil)).
		ModelTableExpr(`users.student_documents AS "care_document"`).
		Set("file_deleted_at = ?", time.Now()).
		Where(`"care_document".id = ?`, documentID).
		Where(`"care_document".file_deleted_at IS NULL`).
		Where(`"care_document".tenant_id = ?`, tenantID).
		WhereAllWithDeleted(), "mark care document file deleted")
}

func (s *Store) QueueCareDocumentCleanup(ctx context.Context, value domain.CareDocumentCleanup) (domain.CareDocumentCleanup, domain.OperationStats, error) {
	db, tenantID, err := s.databaseForWrite(ctx, "queue care document cleanup")
	if err != nil {
		return domain.CareDocumentCleanup{}, domain.OperationStats{}, err
	}
	row := careDocumentCleanupFromDomain(value)
	row.TenantID = tenantID
	stats, err := execAny(ctx, db.NewInsert().Model(&row).
		ModelTableExpr(`users.student_document_file_cleanup AS "care_document_cleanup"`).
		On("CONFLICT (tenant_id, filename_stored) DO UPDATE").
		Set("cleaned_at = NULL").Set("retry_after = EXCLUDED.retry_after").Set("owner_id = EXCLUDED.owner_id").
		Returning("*"), "queue care document cleanup")
	if err != nil {
		return domain.CareDocumentCleanup{}, stats, err
	}
	return careDocumentCleanupToDomain(row), stats, nil
}

func (s *Store) CompleteCareDocumentCleanup(ctx context.Context, cleanupID int64) (domain.OperationStats, error) {
	db, tenantID, err := s.databaseForWrite(ctx, "complete care document cleanup")
	if err != nil {
		return domain.OperationStats{}, err
	}
	return execAny(ctx, db.NewUpdate().Model((*careDocumentCleanupRow)(nil)).
		ModelTableExpr(`users.student_document_file_cleanup AS "care_document_cleanup"`).
		Set("cleaned_at = ?", time.Now()).
		Where(`"care_document_cleanup".id = ?`, cleanupID).
		Where(`"care_document_cleanup".cleaned_at IS NULL`).
		Where(`"care_document_cleanup".tenant_id = ?`, tenantID), "complete care document cleanup")
}

func (s *Store) CompleteCareDocumentCleanupByFilename(ctx context.Context, filename string) (domain.OperationStats, error) {
	db, tenantID, err := s.databaseForWrite(ctx, "complete care document cleanup by filename")
	if err != nil {
		return domain.OperationStats{}, err
	}
	return execAny(ctx, db.NewUpdate().Model((*careDocumentCleanupRow)(nil)).
		ModelTableExpr(`users.student_document_file_cleanup AS "care_document_cleanup"`).
		Set("cleaned_at = ?", time.Now()).
		Where(`"care_document_cleanup".filename_stored = ?`, filename).
		Where(`"care_document_cleanup".cleaned_at IS NULL`).
		Where(`"care_document_cleanup".tenant_id = ?`, tenantID), "complete care document cleanup by filename")
}

func (s *Store) ActivateCareDocumentCleanup(ctx context.Context, filename string) (domain.OperationStats, error) {
	db, tenantID, err := s.databaseForWrite(ctx, "activate care document cleanup")
	if err != nil {
		return domain.OperationStats{}, err
	}
	return execAny(ctx, db.NewUpdate().Model((*careDocumentCleanupRow)(nil)).
		ModelTableExpr(`users.student_document_file_cleanup AS "care_document_cleanup"`).
		Set("retry_after = ?", time.Now()).
		Where(`"care_document_cleanup".filename_stored = ?`, filename).
		Where(`"care_document_cleanup".cleaned_at IS NULL`).
		Where(`"care_document_cleanup".tenant_id = ?`, tenantID), "activate care document cleanup")
}

func careDocumentFromDomain(value domain.CareDocument) (careDocumentRow, error) {
	row := careDocumentRow{ID: value.ID, TenantID: value.TenantID, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt, StudentID: value.StudentID, Category: value.Category, FilenameDisplay: value.FilenameDisplay, FilenameStored: value.FilenameStored, SizeBytes: value.SizeBytes, ContentType: value.ContentType, UploadedBy: value.UploadedBy, DeletedAt: value.DeletedAt, DeletedBy: value.DeletedBy, FileDeletedAt: value.FileDeletedAt,
		AccidentSupersedesID: value.AccidentSupersedesID, AccidentSignatureID: value.AccidentSignatureID, AccidentSignedAt: value.AccidentSignedAt, AccidentSignedBy: value.AccidentSignedBy}
	if value.AccidentReport != nil {
		encoded, err := json.Marshal(value.AccidentReport)
		if err != nil {
			return careDocumentRow{}, fmt.Errorf("encode accident report: %w", err)
		}
		row.AccidentReport = encoded
	}
	return row, nil
}

// careDocumentToDomain fails on a report column it cannot read: a broken
// report must surface as an error, not as a document without a report.
func careDocumentToDomain(row careDocumentRow) (domain.CareDocument, error) {
	value := domain.CareDocument{ID: row.ID, TenantID: row.TenantID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, StudentID: row.StudentID, Category: row.Category, FilenameDisplay: row.FilenameDisplay, FilenameStored: row.FilenameStored, SizeBytes: row.SizeBytes, ContentType: row.ContentType, UploadedBy: row.UploadedBy, DeletedAt: row.DeletedAt, DeletedBy: row.DeletedBy, FileDeletedAt: row.FileDeletedAt,
		AccidentSupersedesID: row.AccidentSupersedesID, AccidentSignatureID: row.AccidentSignatureID, AccidentSignedAt: row.AccidentSignedAt, AccidentSignedBy: row.AccidentSignedBy}
	if len(row.AccidentReport) > 0 {
		report := new(careplan.AccidentReport)
		if err := json.Unmarshal(row.AccidentReport, report); err != nil {
			return domain.CareDocument{}, fmt.Errorf("care plan postgres: decode accident report of document %d: %w", row.ID, err)
		}
		value.AccidentReport = report
	}
	return value, nil
}

func careDocumentCleanupFromDomain(value domain.CareDocumentCleanup) careDocumentCleanupRow {
	return careDocumentCleanupRow{ID: value.ID, TenantID: value.TenantID, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt, OwnerID: value.OwnerID, FilenameStored: value.FilenameStored, RetryAfter: value.RetryAfter, CleanedAt: value.CleanedAt}
}

func careDocumentCleanupToDomain(row careDocumentCleanupRow) domain.CareDocumentCleanup {
	return domain.CareDocumentCleanup{ID: row.ID, TenantID: row.TenantID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, OwnerID: row.OwnerID, FilenameStored: row.FilenameStored, RetryAfter: row.RetryAfter, CleanedAt: row.CleanedAt}
}

// accidentSuccessorConstraint keeps one live correction per accident report.
const accidentSuccessorConstraint = "uq_student_documents_accident_successor"

// isAccidentSuccessorConflict reports a second correction of the same report
// that lost the race to the first.
func isAccidentSuccessorConflict(err error) bool {
	var postgresError pgdriver.Error
	return errors.As(err, &postgresError) && postgresError.IntegrityViolation() &&
		postgresError.Field('n') == accidentSuccessorConstraint
}
