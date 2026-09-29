package application

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/domain"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The Dokumente tab (#1424, phase 1). The route gate only proves the caller may
// reach the tab at all — per-category authority is decided here. Every upload
// and delete writes a Stammdaten audit row in the same tenant transaction;
// serving a sensitive category is refused when the access-log write fails.

// CreateDocumentInput carries the metadata of an already-stored upload. The
// handler owns the file bytes (parse, magic-number validation, disk write);
// the administration owns authority, the metadata row and the audit trail.
type CreateDocumentInput struct {
	StaffID         int64
	Category        string
	FilenameDisplay string
	FilenameStored  string
	SizeBytes       int64
	ContentType     string
}

// ListStaffDocuments returns the documents the actor may see, newest first,
// plus the actor's visible categories (the frontend builds its filter and
// upload choices from them). A non-empty category narrows the list and must be
// visible to the actor.
func (a *StaffAdmin) ListStaffDocuments(ctx context.Context, staffID int64, category string, actor domain.DocumentActor) ([]domain.StaffDocumentInfo, []string, error) {
	if err := a.subjects.StaffExists(ctx, staffID); err != nil {
		return nil, nil, err
	}

	visible := a.authority.Visible(actor)
	query := visible
	if category != "" {
		if !domain.IsValidStaffDocumentCategory(category) {
			return nil, nil, fmt.Errorf("%w: unknown category", domain.ErrStaffDocumentInvalid)
		}
		if err := a.authority.Require(category, actor); err != nil {
			return nil, nil, err
		}
		query = []string{category}
	}

	documents := []domain.StaffDocument{}
	if len(query) > 0 {
		var err error
		documents, err = a.records.ListStaffDocuments(ctx, domain.StaffDocumentFilter{StaffID: staffID, Categories: query})
		if err != nil {
			return nil, nil, err
		}
	}

	contractEnd, err := a.contractEnd(ctx, staffID)
	if err != nil {
		return nil, nil, err
	}
	infos := make([]domain.StaffDocumentInfo, 0, len(documents))
	for _, document := range documents {
		infos = append(infos, documentInfo(document, contractEnd))
	}
	return infos, visible, nil
}

// CreateStaffDocument persists the metadata row and the audit entry in one
// tenant transaction.
func (a *StaffAdmin) CreateStaffDocument(ctx context.Context, input CreateDocumentInput, actor domain.DocumentActor) (domain.StaffDocumentInfo, error) {
	document, err := a.newDocument(input, actor)
	if err != nil {
		return domain.StaffDocumentInfo{}, err
	}

	var info domain.StaffDocumentInfo
	err = a.transaction.RunWrite(ctx, func(ctx context.Context) error {
		if err := a.subjects.StaffExists(ctx, input.StaffID); err != nil {
			return err
		}
		if err := a.audit.RecordMasterDataChange(ctx, domain.MasterDataChange{
			StaffID: input.StaffID, ChangedBy: actor.AccountID, Section: domain.AuditSectionDokumente,
			Field: input.Category, NewValue: input.FilenameDisplay,
		}); err != nil {
			return fmt.Errorf("write document audit: %w", err)
		}
		created, err := a.records.CreateStaffDocument(ctx, document)
		if err != nil {
			return err
		}
		if err := a.records.CompleteStaffDocumentFileCleanupByFilename(ctx, input.FilenameStored); err != nil {
			return fmt.Errorf("complete document upload cleanup intent: %w", err)
		}
		contractEnd, retentionErr := a.contractEnd(ctx, input.StaffID)
		if retentionErr != nil {
			// Retention is advisory; preserving the successful upload matches the
			// established response contract. This lookup still runs in the tenant
			// transaction so the production role can read the master data table.
			a.warn("staff document retention lookup failed",
				"staff_id", input.StaffID,
				"error", retentionErr.Error(),
			)
			contractEnd = ""
		}
		info = documentInfo(created, contractEnd)
		return nil
	})
	if err != nil {
		return domain.StaffDocumentInfo{}, err
	}
	return info, nil
}

// newDocument validates an upload's metadata and the actor's authority over its
// category before anything is written.
func (a *StaffAdmin) newDocument(input CreateDocumentInput, actor domain.DocumentActor) (domain.StaffDocument, error) {
	if actor.AccountID <= 0 {
		return domain.StaffDocument{}, errors.New("actor account id is required")
	}
	if !domain.IsValidStaffDocumentCategory(input.Category) {
		return domain.StaffDocument{}, fmt.Errorf("%w: unknown category", domain.ErrStaffDocumentInvalid)
	}
	if err := a.authority.Require(input.Category, actor); err != nil {
		return domain.StaffDocument{}, err
	}
	input.FilenameDisplay = strings.TrimSpace(input.FilenameDisplay)
	if input.FilenameDisplay == "" {
		return domain.StaffDocument{}, fmt.Errorf("%w: filename is required", domain.ErrStaffDocumentInvalid)
	}
	document := domain.StaffDocument{
		StaffID: input.StaffID, Category: input.Category, FilenameDisplay: input.FilenameDisplay,
		FilenameStored: input.FilenameStored, SizeBytes: input.SizeBytes, ContentType: input.ContentType, UploadedBy: actor.AccountID,
	}
	if err := document.Validate(); err != nil {
		return domain.StaffDocument{}, fmt.Errorf("%w: %s", domain.ErrStaffDocumentInvalid, err.Error())
	}
	return document, nil
}

// ResolveStaffDocumentDownload authorizes the download and, for the sensitive
// categories, writes the data-access log row. No file is served when the
// returned error is non-nil.
func (a *StaffAdmin) ResolveStaffDocumentDownload(ctx context.Context, staffID, documentID int64, actor domain.DocumentActor) (domain.StaffDocument, error) {
	var resolved domain.StaffDocument
	err := a.transaction.RunWrite(ctx, func(ctx context.Context) error {
		if err := a.subjects.StaffExists(ctx, staffID); err != nil {
			return err
		}
		document, err := a.authorizedDocument(ctx, staffID, documentID, false, actor)
		if err != nil {
			return err
		}
		if domain.SensitiveStaffDocumentCategory(document.Category) {
			if actor.AccountID <= 0 {
				return errors.New("actor account id is required for document downloads")
			}
			if err := a.audit.RecordDataAccess(ctx, domain.DataAccess{
				ActorAccountID: actor.AccountID, ActorRole: domain.ActorRoleOrUnknown(actor.Role),
				Resource: domain.DataAccessDocumentDownload, StaffID: staffID, DocumentID: document.ID,
				Category: document.Category, At: a.clock.Now(),
			}); err != nil {
				return fmt.Errorf("write document access audit: %w", err)
			}
		}
		resolved = document
		return nil
	})
	if err != nil {
		return domain.StaffDocument{}, err
	}
	return resolved, nil
}

// DeleteStaffDocument soft-deletes the metadata row and writes the audit entry
// in one tenant transaction. The handler unlinks the file only after this
// transaction commits.
func (a *StaffAdmin) DeleteStaffDocument(ctx context.Context, staffID, documentID int64, actor domain.DocumentActor) (domain.StaffDocument, error) {
	if actor.AccountID <= 0 {
		return domain.StaffDocument{}, errors.New("actor account id is required")
	}
	var deleted domain.StaffDocument
	err := a.transaction.RunWrite(ctx, func(ctx context.Context) error {
		document, err := a.authorizedDocument(ctx, staffID, documentID, false, actor)
		if err != nil {
			return err
		}
		at := a.clock.Now()
		affected, err := a.records.SoftDeleteStaffDocument(ctx, document.ID, actor.AccountID, at)
		if err != nil {
			return err
		}
		if affected == 0 {
			// A row that is already gone reports the not-found shape the HTTP
			// layer maps to 404.
			return &domain.RecordNotFoundError{Op: "soft delete staff document"}
		}
		document.DeletedAt = &at
		deletedBy := actor.AccountID
		document.DeletedBy = &deletedBy
		if err := a.audit.RecordMasterDataChange(ctx, domain.MasterDataChange{
			StaffID: staffID, ChangedBy: actor.AccountID, Section: domain.AuditSectionDokumente,
			Field: document.Category, OldValue: document.FilenameDisplay,
		}); err != nil {
			return fmt.Errorf("write document audit: %w", err)
		}
		deleted = document
		return nil
	})
	if err != nil {
		return domain.StaffDocument{}, err
	}
	return deleted, nil
}

// ResolveStaffDocumentCleanup authorizes a retry of the filesystem cleanup for
// a document that may already be soft-deleted.
func (a *StaffAdmin) ResolveStaffDocumentCleanup(ctx context.Context, staffID, documentID int64, actor domain.DocumentActor) (domain.StaffDocument, error) {
	return a.authorizedDocument(ctx, staffID, documentID, true, actor)
}

// ListOffboardedStaffDocumentsPendingFileCleanup returns tenant-wide cleanup
// candidates whose staff member has already been offboarded.
func (a *StaffAdmin) ListOffboardedStaffDocumentsPendingFileCleanup(ctx context.Context) ([]domain.StaffDocument, error) {
	offboarded, err := a.subjects.OffboardedStaffIDs(ctx)
	if err != nil {
		return nil, fmt.Errorf("list offboarded staff: %w", err)
	}
	if len(offboarded) == 0 {
		return []domain.StaffDocument{}, nil
	}
	return a.records.ListStaffDocuments(ctx, domain.OffboardedPendingFileCleanupFilter(offboarded))
}

// ListDeletedStaffDocumentsPendingFileCleanup returns authorized retry
// candidates whose previous file unlink did not complete.
func (a *StaffAdmin) ListDeletedStaffDocumentsPendingFileCleanup(ctx context.Context, staffID int64, actor domain.DocumentActor) ([]domain.StaffDocument, error) {
	categories := a.authority.Visible(actor)
	if len(categories) == 0 {
		return []domain.StaffDocument{}, nil
	}
	return a.records.ListStaffDocuments(ctx, domain.DeletedPendingFileCleanupFilter(staffID, categories))
}

// MarkStaffDocumentFileDeleted records successful physical cleanup so it is
// never retried during a later list or offboarding request.
func (a *StaffAdmin) MarkStaffDocumentFileDeleted(ctx context.Context, documentID int64) error {
	return a.transaction.RunWrite(ctx, func(ctx context.Context) error {
		return a.records.MarkStaffDocumentFileDeleted(ctx, documentID, a.clock.Now())
	})
}

// QueueStaffDocumentFileCleanup durably records the cleanup intent before the
// file is written. It stays ineligible for retries until the upload explicitly
// fails or domain.StaffDocumentCleanupDelay has elapsed — by which time the
// upload request has been cut off by the upload deadline, so an eligible
// intent can never race a still-running upload.
func (a *StaffAdmin) QueueStaffDocumentFileCleanup(ctx context.Context, staffID int64, storedName string) error {
	if staffID <= 0 || strings.TrimSpace(storedName) == "" {
		return fmt.Errorf("%w: cleanup file details are required", domain.ErrStaffDocumentInvalid)
	}
	return a.transaction.RunWrite(ctx, func(ctx context.Context) error {
		return a.records.QueueStaffDocumentFileCleanup(ctx, domain.StaffDocumentFileCleanup{
			StaffID: staffID, FilenameStored: storedName, RetryAfter: a.clock.Now().Add(domain.StaffDocumentCleanupDelay),
		})
	})
}

func (a *StaffAdmin) MarkQueuedStaffDocumentFileCleanupComplete(ctx context.Context, cleanupID int64) error {
	return a.transaction.RunWrite(ctx, func(ctx context.Context) error {
		return a.records.CompleteStaffDocumentFileCleanup(ctx, cleanupID)
	})
}

func (a *StaffAdmin) MarkQueuedStaffDocumentFileCleanupCompleteByFilename(ctx context.Context, storedName string) error {
	return a.transaction.RunWrite(ctx, func(ctx context.Context) error {
		return a.records.CompleteStaffDocumentFileCleanupByFilename(ctx, storedName)
	})
}

// ActivateQueuedStaffDocumentFileCleanup makes a failed upload eligible for
// retry after its immediate filesystem cleanup failed.
func (a *StaffAdmin) ActivateQueuedStaffDocumentFileCleanup(ctx context.Context, storedName string) error {
	return a.transaction.RunWrite(ctx, func(ctx context.Context) error {
		return a.records.ActivateStaffDocumentFileCleanup(ctx, storedName)
	})
}

// authorizedDocument loads one document of a staff member and refuses a
// category the actor's permissions do not cover.
func (a *StaffAdmin) authorizedDocument(ctx context.Context, staffID, documentID int64, includeDeleted bool, actor domain.DocumentActor) (domain.StaffDocument, error) {
	document, err := a.findDocument(ctx, staffID, documentID, includeDeleted)
	if err != nil {
		return domain.StaffDocument{}, err
	}
	if err := a.authority.Require(document.Category, actor); err != nil {
		return domain.StaffDocument{}, err
	}
	return document, nil
}

// findDocument loads one document of a staff member; a missing row reports the
// not-found shape the HTTP layer maps to 404.
func (a *StaffAdmin) findDocument(ctx context.Context, staffID, documentID int64, includeDeleted bool) (domain.StaffDocument, error) {
	document, err := a.records.FindStaffDocument(ctx, staffID, documentID, includeDeleted)
	if err == nil {
		return document, nil
	}
	if errors.Is(err, domain.ErrStaffDocumentNotFound) || errors.Is(err, domain.ErrInvalidStaffRecord) {
		return domain.StaffDocument{}, &domain.RecordNotFoundError{Op: "find staff document"}
	}
	return domain.StaffDocument{}, err
}

// contractEnd loads the staff member's contract end from the Stammdaten row —
// the anchor of the Arbeitsvertrag retention rule. Empty when no master data or
// no end date exists (unbefristet: retention stays open).
func (a *StaffAdmin) contractEnd(ctx context.Context, staffID int64) (string, error) {
	row, err := a.masterData(ctx, staffID)
	if err != nil || row == nil {
		return "", err
	}
	return row.ContractEndDate, nil
}

func documentInfo(document domain.StaffDocument, contractEnd string) domain.StaffDocumentInfo {
	return domain.NewStaffDocumentInfo(document, calendar.DateFromTime(document.CreatedAt).String(), contractEnd)
}
