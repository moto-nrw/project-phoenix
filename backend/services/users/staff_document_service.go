package users

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// Staff documents (#1424, Dokumente tab phase 1). The route gate only proves
// the caller may reach the tab at all — per-category authority is decided
// here: AU-Bescheinigungen are Art. 9 health data (staff_documents:health),
// Lohnabrechnungen belong to the payroll tier (staff:financial), everything
// else to the directory maintainers (users:update). Every upload and delete
// writes a Stammdaten audit row in the same tenant transaction; serving a
// sensitive category is refused when the access-log write fails — the same
// two hard rules as the Stammdaten service.

// ErrStaffDocumentInvalid marks a semantically invalid document payload —
// HTTP 400. Wrapped with a field-specific message.
var ErrStaffDocumentInvalid = errors.New("invalid staff document")

// ErrStaffDocumentForbidden marks a category the caller's permissions do not
// cover — HTTP 403.
var ErrStaffDocumentForbidden = errors.New("staff document category not permitted")

// StaffDocumentActor identifies the acting account for permission checks and
// audit rows.
type StaffDocumentActor struct {
	AccountID   int64
	Role        string
	Permissions []string
}

// StaffDocumentInfo is one document plus its derived retention view.
// RetainUntil is the earliest calendar day the category's statutory
// retention allows disposal ("aufbewahren bis"); ReviewDue is the
// BDSG-motivated review date for application documents. Both are advisory —
// phase 1 never auto-deletes, and deletes are not blocked either (a wrong
// upload must stay removable).
type StaffDocumentInfo struct {
	Document    *userModels.StaffDocument
	RetainUntil *timezone.Date
	ReviewDue   *timezone.Date
}

// CreateStaffDocumentInput carries the metadata of an already-stored upload.
// The handler owns the file bytes (parse, magic-number validation, disk
// write); the service owns authority, the metadata row and the audit trail.
type CreateStaffDocumentInput struct {
	StaffID         int64
	Category        string
	FilenameDisplay string
	FilenameStored  string
	SizeBytes       int64
	ContentType     string
}

// StaffDocumentService is the business boundary of the Dokumente tab.
type StaffDocumentService interface {
	// ListStaffDocuments returns the documents the actor may see, newest
	// first, plus the actor's visible categories (the frontend builds its
	// filter and upload choices from them). A non-empty category narrows
	// the list and must be visible to the actor.
	ListStaffDocuments(ctx context.Context, staffID int64, category string, actor StaffDocumentActor) ([]*StaffDocumentInfo, []string, error)
	// CreateStaffDocument persists the metadata row and the audit entry in
	// one tenant transaction.
	CreateStaffDocument(ctx context.Context, input CreateStaffDocumentInput, actor StaffDocumentActor) (*StaffDocumentInfo, error)
	// ResolveStaffDocumentDownload authorizes the download and, for the
	// sensitive categories, writes the data-access log row. No file is
	// served when the returned error is non-nil.
	ResolveStaffDocumentDownload(ctx context.Context, staffID, documentID int64, actor StaffDocumentActor) (*userModels.StaffDocument, error)
	// DeleteStaffDocument soft-deletes the metadata row and writes the
	// audit entry in one tenant transaction. The handler unlinks the file only
	// after this transaction commits.
	DeleteStaffDocument(ctx context.Context, staffID, documentID int64, actor StaffDocumentActor) (*userModels.StaffDocument, error)
	// ListStaffDocumentsPendingFileCleanup returns stored-file cleanup
	// candidates for offboarding, excluding files already removed.
	ListStaffDocumentsPendingFileCleanup(ctx context.Context, staffID int64) ([]*userModels.StaffDocument, error)
	// ListOffboardedStaffDocumentsPendingFileCleanup returns tenant-wide cleanup
	// candidates whose staff member has already been offboarded.
	ListOffboardedStaffDocumentsPendingFileCleanup(ctx context.Context) ([]*userModels.StaffDocument, error)
	// ListDeletedStaffDocumentsPendingFileCleanup returns authorized retry
	// candidates whose previous file unlink did not complete.
	ListDeletedStaffDocumentsPendingFileCleanup(ctx context.Context, staffID int64, actor StaffDocumentActor) ([]*userModels.StaffDocument, error)
	// MarkStaffDocumentFileDeleted records successful physical cleanup so it is
	// never retried during a later list or offboarding request.
	MarkStaffDocumentFileDeleted(ctx context.Context, documentID int64) error
	// QueueStaffDocumentFileCleanup durably records the cleanup intent before
	// the file is written. It remains ineligible for retries until the upload
	// fails or the initial grace period has elapsed after an interrupted upload.
	QueueStaffDocumentFileCleanup(ctx context.Context, staffID int64, storedName string) error
	// ListQueuedStaffDocumentFileCleanup returns orphaned upload files that
	// should be retried for this staff record during list or offboarding flows.
	ListQueuedStaffDocumentFileCleanup(ctx context.Context, staffID int64) ([]*userModels.StaffDocumentFileCleanup, error)
	// ListQueuedStaffDocumentFileCleanups returns all orphaned upload files in
	// the tenant so retries do not depend on the referenced staff record.
	ListQueuedStaffDocumentFileCleanups(ctx context.Context) ([]*userModels.StaffDocumentFileCleanup, error)
	MarkQueuedStaffDocumentFileCleanupComplete(ctx context.Context, cleanupID int64) error
	MarkQueuedStaffDocumentFileCleanupCompleteByFilename(ctx context.Context, storedName string) error
	// ActivateQueuedStaffDocumentFileCleanup makes a failed upload eligible for
	// retry after its immediate filesystem cleanup failed.
	ActivateQueuedStaffDocumentFileCleanup(ctx context.Context, storedName string) error
	// ResolveStaffDocumentCleanup authorizes a retry of the filesystem cleanup
	// for a document that may already be soft-deleted.
	ResolveStaffDocumentCleanup(ctx context.Context, staffID, documentID int64, actor StaffDocumentActor) (*userModels.StaffDocument, error)
}

type staffDocumentService struct {
	db         *bun.DB
	documents  userModels.StaffDocumentRepository
	staff      userModels.StaffRepository
	masterData userModels.StaffMasterDataRepository
	audit      auditModels.StaffMasterDataChangeCreator
	accessLog  auditModels.DataAccessLogRepository
	logger     *slog.Logger
}

// NewStaffDocumentService wires the document service.
func NewStaffDocumentService(
	db *bun.DB,
	documents userModels.StaffDocumentRepository,
	staff userModels.StaffRepository,
	masterData userModels.StaffMasterDataRepository,
	audit auditModels.StaffMasterDataChangeCreator,
	accessLog auditModels.DataAccessLogRepository,
	logger *slog.Logger,
) StaffDocumentService {
	return &staffDocumentService{
		db:         db,
		documents:  documents,
		staff:      staff,
		masterData: masterData,
		audit:      audit,
		accessLog:  accessLog,
		logger:     logger,
	}
}

// getLogger returns the injected logger, falling back to slog.Default() so
// bare-constructed services (tests) stay nil-safe.
func (s *staffDocumentService) getLogger() *slog.Logger {
	if s.logger == nil {
		return slog.Default()
	}
	return s.logger
}

// staffDocumentCategoryPermission returns the permission a category
// requires. The mapping is the whole point of the permission design — keep
// it in one place.
func staffDocumentCategoryPermission(category string) string {
	switch category {
	case userModels.StaffDocumentCategoryAUBescheinigung:
		return permissions.StaffDocumentsHealth
	case userModels.StaffDocumentCategoryLohnabrechnung:
		return permissions.StaffFinancial
	default:
		return permissions.UsersUpdate
	}
}

// staffDocumentCategorySensitive reports whether serving the category's file
// content requires a data-access log row.
func staffDocumentCategorySensitive(category string) bool {
	return category == userModels.StaffDocumentCategoryAUBescheinigung ||
		category == userModels.StaffDocumentCategoryLohnabrechnung
}

// visibleStaffDocumentCategories filters the category enum down to what the
// actor may see (wildcard-aware, so admin:* matches everything).
func visibleStaffDocumentCategories(actor StaffDocumentActor) []string {
	visible := make([]string, 0, len(userModels.StaffDocumentCategories))
	for _, category := range userModels.StaffDocumentCategories {
		if authorize.HasPermission(staffDocumentCategoryPermission(category), actor.Permissions) {
			visible = append(visible, category)
		}
	}
	return visible
}

func (s *staffDocumentService) requireCategoryPermission(category string, actor StaffDocumentActor) error {
	if !authorize.HasPermission(staffDocumentCategoryPermission(category), actor.Permissions) {
		return fmt.Errorf("%w: %s", ErrStaffDocumentForbidden, category)
	}
	return nil
}

func (s *staffDocumentService) ListStaffDocuments(ctx context.Context, staffID int64, category string, actor StaffDocumentActor) ([]*StaffDocumentInfo, []string, error) {
	if _, err := s.staff.FindByID(ctx, staffID); err != nil {
		return nil, nil, err
	}

	visible := visibleStaffDocumentCategories(actor)
	query := visible
	if category != "" {
		if !userModels.IsValidStaffDocumentCategory(category) {
			return nil, nil, fmt.Errorf("%w: unknown category", ErrStaffDocumentInvalid)
		}
		if err := s.requireCategoryPermission(category, actor); err != nil {
			return nil, nil, err
		}
		query = []string{category}
	}

	docs, err := s.documents.ListByStaffID(ctx, staffID, query)
	if err != nil {
		return nil, nil, err
	}

	contractEnd, err := s.contractEndDate(ctx, staffID)
	if err != nil {
		return nil, nil, err
	}

	infos := make([]*StaffDocumentInfo, 0, len(docs))
	for _, doc := range docs {
		infos = append(infos, newStaffDocumentInfo(doc, contractEnd))
	}
	return infos, visible, nil
}

func (s *staffDocumentService) CreateStaffDocument(ctx context.Context, input CreateStaffDocumentInput, actor StaffDocumentActor) (*StaffDocumentInfo, error) {
	if s.audit == nil {
		return nil, fmt.Errorf("stammdaten audit repository is not wired; refusing unaudited change")
	}
	if actor.AccountID <= 0 {
		return nil, fmt.Errorf("actor account id is required")
	}
	if !userModels.IsValidStaffDocumentCategory(input.Category) {
		return nil, fmt.Errorf("%w: unknown category", ErrStaffDocumentInvalid)
	}
	if err := s.requireCategoryPermission(input.Category, actor); err != nil {
		return nil, err
	}
	input.FilenameDisplay = strings.TrimSpace(input.FilenameDisplay)
	if input.FilenameDisplay == "" {
		return nil, fmt.Errorf("%w: filename is required", ErrStaffDocumentInvalid)
	}

	doc := &userModels.StaffDocument{
		StaffID:         input.StaffID,
		Category:        input.Category,
		FilenameDisplay: input.FilenameDisplay,
		FilenameStored:  input.FilenameStored,
		SizeBytes:       input.SizeBytes,
		ContentType:     input.ContentType,
		UploadedBy:      actor.AccountID,
	}
	if err := doc.Validate(); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrStaffDocumentInvalid, err.Error())
	}

	tenantID := tenant.FromContext(ctx)
	var contractEnd *timezone.Date
	err := tenant.WithTenantTx(ctx, s.db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		if _, err := s.staff.FindByID(ctx, input.StaffID); err != nil {
			return err
		}
		if err := s.audit.Create(ctx, &auditModels.StaffMasterDataChange{
			StaffID:   input.StaffID,
			ChangedBy: actor.AccountID,
			Section:   auditModels.StammdatenSectionDokumente,
			FieldName: input.Category,
			OldValue:  "",
			NewValue:  input.FilenameDisplay,
		}); err != nil {
			return fmt.Errorf("write document audit: %w", err)
		}
		if err := s.documents.Create(ctx, doc); err != nil {
			return err
		}
		if err := s.documents.MarkQueuedFileCleanupCompleteByFilename(ctx, input.FilenameStored); err != nil {
			return fmt.Errorf("complete document upload cleanup intent: %w", err)
		}
		var retentionErr error
		contractEnd, retentionErr = s.contractEndDate(ctx, input.StaffID)
		if retentionErr != nil {
			// Retention is advisory; preserving the successful upload matches the
			// established response contract. This lookup still runs in the tenant
			// transaction so the production role can read the master-data table.
			s.getLogger().Warn("staff document retention lookup failed",
				"staff_id", input.StaffID,
				"error", retentionErr.Error(),
			)
			contractEnd = nil
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return newStaffDocumentInfo(doc, contractEnd), nil
}

func (s *staffDocumentService) ResolveStaffDocumentDownload(ctx context.Context, staffID, documentID int64, actor StaffDocumentActor) (*userModels.StaffDocument, error) {
	var document *userModels.StaffDocument
	tenantID := tenant.FromContext(ctx)
	err := tenant.WithTenantTx(ctx, s.db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		if _, err := s.staff.FindByID(ctx, staffID); err != nil {
			return err
		}
		doc, err := s.documents.FindForStaff(ctx, staffID, documentID)
		if err != nil {
			return err
		}
		if err := s.requireCategoryPermission(doc.Category, actor); err != nil {
			return err
		}

		if staffDocumentCategorySensitive(doc.Category) {
			if s.accessLog == nil {
				return fmt.Errorf("data access log repository is not wired; refusing unaudited document download")
			}
			if actor.AccountID <= 0 {
				return fmt.Errorf("actor account id is required for document downloads")
			}
			role := strings.TrimSpace(actor.Role)
			if role == "" {
				role = "unknown"
			}
			now := time.Now()
			if err := s.accessLog.Create(ctx, &auditModels.DataAccessLog{
				ActorAccountID: actor.AccountID,
				ActorRole:      role,
				ResourceType:   auditModels.ResourceTypeStaffDocumentDownload,
				RangeStart:     now,
				RangeEnd:       now,
				AccessedAt:     now,
				Metadata: map[string]interface{}{
					"staff_id":    staffID,
					"document_id": doc.ID,
					"category":    doc.Category,
				},
			}); err != nil {
				return fmt.Errorf("write document access audit: %w", err)
			}
		}
		document = doc
		return nil
	})
	if err != nil {
		return nil, err
	}
	return document, nil
}

func (s *staffDocumentService) DeleteStaffDocument(ctx context.Context, staffID, documentID int64, actor StaffDocumentActor) (*userModels.StaffDocument, error) {
	if s.audit == nil {
		return nil, fmt.Errorf("stammdaten audit repository is not wired; refusing unaudited change")
	}
	if actor.AccountID <= 0 {
		return nil, fmt.Errorf("actor account id is required")
	}

	var deleted *userModels.StaffDocument
	tenantID := tenant.FromContext(ctx)
	err := tenant.WithTenantTx(ctx, s.db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		doc, err := s.documents.FindForStaff(ctx, staffID, documentID)
		if err != nil {
			return err
		}
		if err := s.requireCategoryPermission(doc.Category, actor); err != nil {
			return err
		}
		if err := s.documents.SoftDelete(ctx, doc, actor.AccountID); err != nil {
			return err
		}
		if err := s.audit.Create(ctx, &auditModels.StaffMasterDataChange{
			StaffID:   staffID,
			ChangedBy: actor.AccountID,
			Section:   auditModels.StammdatenSectionDokumente,
			FieldName: doc.Category,
			OldValue:  doc.FilenameDisplay,
			NewValue:  "",
		}); err != nil {
			return fmt.Errorf("write document audit: %w", err)
		}
		deleted = doc
		return nil
	})
	if err != nil {
		return nil, err
	}
	return deleted, nil
}

func (s *staffDocumentService) ListStaffDocumentsPendingFileCleanup(ctx context.Context, staffID int64) ([]*userModels.StaffDocument, error) {
	return s.documents.ListPendingFileCleanupByStaffID(ctx, staffID)
}

func (s *staffDocumentService) ListOffboardedStaffDocumentsPendingFileCleanup(ctx context.Context) ([]*userModels.StaffDocument, error) {
	return s.documents.ListOffboardedPendingFileCleanups(ctx)
}

func (s *staffDocumentService) ListDeletedStaffDocumentsPendingFileCleanup(ctx context.Context, staffID int64, actor StaffDocumentActor) ([]*userModels.StaffDocument, error) {
	return s.documents.ListDeletedPendingFileCleanupByStaffID(ctx, staffID, visibleStaffDocumentCategories(actor))
}

func (s *staffDocumentService) MarkStaffDocumentFileDeleted(ctx context.Context, documentID int64) error {
	tenantID := tenant.FromContext(ctx)
	return tenant.WithTenantTx(ctx, s.db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		return s.documents.MarkFileDeleted(ctx, documentID)
	})
}

func (s *staffDocumentService) QueueStaffDocumentFileCleanup(ctx context.Context, staffID int64, storedName string) error {
	if staffID <= 0 || strings.TrimSpace(storedName) == "" {
		return fmt.Errorf("%w: cleanup file details are required", ErrStaffDocumentInvalid)
	}
	tenantID := tenant.FromContext(ctx)
	return tenant.WithTenantTx(ctx, s.db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		return s.documents.QueueFileCleanup(ctx, &userModels.StaffDocumentFileCleanup{
			StaffID:        staffID,
			FilenameStored: storedName,
			RetryAfter:     time.Now().Add(5 * time.Minute),
		})
	})
}

func (s *staffDocumentService) ListQueuedStaffDocumentFileCleanup(ctx context.Context, staffID int64) ([]*userModels.StaffDocumentFileCleanup, error) {
	return s.documents.ListQueuedFileCleanupByStaffID(ctx, staffID)
}

func (s *staffDocumentService) ListQueuedStaffDocumentFileCleanups(ctx context.Context) ([]*userModels.StaffDocumentFileCleanup, error) {
	return s.documents.ListQueuedFileCleanups(ctx)
}

func (s *staffDocumentService) MarkQueuedStaffDocumentFileCleanupComplete(ctx context.Context, cleanupID int64) error {
	tenantID := tenant.FromContext(ctx)
	return tenant.WithTenantTx(ctx, s.db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		return s.documents.MarkQueuedFileCleanupComplete(ctx, cleanupID)
	})
}

func (s *staffDocumentService) MarkQueuedStaffDocumentFileCleanupCompleteByFilename(ctx context.Context, storedName string) error {
	tenantID := tenant.FromContext(ctx)
	return tenant.WithTenantTx(ctx, s.db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		return s.documents.MarkQueuedFileCleanupCompleteByFilename(ctx, storedName)
	})
}

func (s *staffDocumentService) ActivateQueuedStaffDocumentFileCleanup(ctx context.Context, storedName string) error {
	tenantID := tenant.FromContext(ctx)
	return tenant.WithTenantTx(ctx, s.db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		return s.documents.ActivateQueuedFileCleanupByFilename(ctx, storedName)
	})
}

func (s *staffDocumentService) ResolveStaffDocumentCleanup(ctx context.Context, staffID, documentID int64, actor StaffDocumentActor) (*userModels.StaffDocument, error) {
	doc, err := s.documents.FindForStaffIncludingDeleted(ctx, staffID, documentID)
	if err != nil {
		return nil, err
	}
	if err := s.requireCategoryPermission(doc.Category, actor); err != nil {
		return nil, err
	}
	return doc, nil
}

// contractEndDate loads the staff member's contract end from the Stammdaten
// row — the anchor of the Arbeitsvertrag retention rule. Nil when no master
// data or no end date exists (unbefristet: retention stays open).
func (s *staffDocumentService) contractEndDate(ctx context.Context, staffID int64) (*timezone.Date, error) {
	masterData, err := s.masterData.FindByStaffID(ctx, staffID)
	if err != nil {
		return nil, err
	}
	if masterData == nil {
		return nil, nil
	}
	return masterData.ContractEndDate, nil
}

// newStaffDocumentInfo derives the advisory retention view of one document.
// Rules from #1424: Lohnabrechnungen 10 years (tax law), AU-Bescheinigungen
// 4 years (tax audit analogy), Arbeitsvertrag contract end + 6 years,
// Bewerbungsunterlagen get a 6-month review date (BDSG), Zeugnis and
// Sonstiges carry no schedule.
func newStaffDocumentInfo(doc *userModels.StaffDocument, contractEnd *timezone.Date) *StaffDocumentInfo {
	info := &StaffDocumentInfo{Document: doc}
	uploaded := timezone.DateFromTime(doc.CreatedAt)

	switch doc.Category {
	case userModels.StaffDocumentCategoryLohnabrechnung:
		info.RetainUntil = addToDate(uploaded, 10, 0)
	case userModels.StaffDocumentCategoryAUBescheinigung:
		info.RetainUntil = addToDate(uploaded, 4, 0)
	case userModels.StaffDocumentCategoryArbeitsvertrag:
		if contractEnd != nil {
			info.RetainUntil = addToDate(*contractEnd, 6, 0)
		}
	case userModels.StaffDocumentCategoryBewerbung:
		info.ReviewDue = addToDate(uploaded, 0, 6)
	}
	return info
}

// addToDate returns d shifted by whole years and months, normalized the way
// time.Date normalizes out-of-range components (Feb 29 + 1y → Mar 1).
func addToDate(d timezone.Date, years int, months int) *timezone.Date {
	shifted := timezone.NewDate(d.Year+years, d.Month+time.Month(months), d.Day)
	return &shifted
}
