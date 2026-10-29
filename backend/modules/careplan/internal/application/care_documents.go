package application

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/ports"
)

// The child documents (#777, #3427). The route gate only proves the caller
// may reach the tab at all; per-category authority is decided here: health
// documents need the health permission (Art. 9 GDPR), custody paperwork the
// legal one, everything else the directory maintainers' permission.
//
// Two rules are hard: every upload and delete writes a change-history row in
// the same tenant transaction, and serving a sensitive category is refused
// outright when the data-access log write fails. An unlogged look at a
// child's medical certificate is worse than a failed request.

// studentDocumentCleanupDelay keeps a queued intent ineligible until well
// past careplan.StudentDocumentUploadDeadline. The difference is slack for a
// metadata transaction whose commit was still in flight when the deadline
// fired; it MUST stay larger than the deadline.
const studentDocumentCleanupDelay = 5 * time.Minute

// StudentDocumentDependencies wire the document capability. Transaction runs
// each command in its OWN tenant transaction, committed before the command
// returns: the upload and download routes run without the request
// transaction, and the delete route removes the bytes only after the
// metadata change is durable.
type StudentDocumentDependencies struct {
	Records     careplan.Capability
	Access      ports.StudentDocumentAccess
	Permissions ports.StudentDocumentPermissions
	Audit       ports.StudentDocumentAudit
	Signatures  ports.StudentAccidentSignatures
	Transaction ports.UnitOfWork
	Now         func() time.Time
}

// StudentDocuments is the native child document application.
type StudentDocuments struct {
	records     careplan.Capability
	access      ports.StudentDocumentAccess
	permissions ports.StudentDocumentPermissions
	audit       ports.StudentDocumentAudit
	signatures  ports.StudentAccidentSignatures
	transaction ports.UnitOfWork
	now         func() time.Time
}

// NewStudentDocuments validates the wiring and builds the capability.
func NewStudentDocuments(deps StudentDocumentDependencies) (*StudentDocuments, error) {
	if deps.Records == nil || deps.Access == nil || deps.Audit == nil || deps.Signatures == nil || deps.Transaction == nil {
		return nil, errors.New("care plan documents: records, access, audit, signatures and transaction are required")
	}
	if deps.Permissions.Health == "" || deps.Permissions.Legal == "" || deps.Permissions.Default == "" ||
		deps.Permissions.AccidentRead == "" || deps.Permissions.AccidentManage == "" {
		return nil, errors.New("care plan documents: every category permission is required")
	}
	if deps.Now == nil {
		deps.Now = time.Now
	}
	return &StudentDocuments{
		records: deps.Records, access: deps.Access, permissions: deps.Permissions,
		audit: deps.Audit, signatures: deps.Signatures, transaction: deps.Transaction, now: deps.Now,
	}, nil
}

// requireStudentAccess enforces the per-child gate every other per-child
// endpoint applies. It runs inside the same transaction as the work it
// guards, which is why it lives here rather than in the handler.
func (s *StudentDocuments) requireStudentAccess(ctx context.Context, studentID int64, actor careplan.StudentDocumentActor) error {
	allowed, err := s.access.CanAccessStudent(ctx, studentID, actor.Permissions)
	if err != nil {
		return err
	}
	if !allowed {
		return careplan.ErrStudentDocumentNoAccess
	}
	return nil
}

// categoryPermission returns the permission a category requires. The mapping
// is the whole point of the permission design; keep it in one place.
func (s *StudentDocuments) categoryPermission(category string) string {
	switch {
	case careplan.IsHealthStudentDocumentCategory(category):
		return s.permissions.Health
	case careplan.IsLegalStudentDocumentCategory(category):
		return s.permissions.Legal
	case careplan.IsAccidentStudentDocumentCategory(category):
		return s.permissions.AccidentRead
	default:
		return s.permissions.Default
	}
}

// writePermission returns the permission adding or removing a document of
// the category requires. Only accident reports separate reading from writing.
func (s *StudentDocuments) writePermission(category string) string {
	if careplan.IsAccidentStudentDocumentCategory(category) {
		return s.permissions.AccidentManage
	}
	return s.categoryPermission(category)
}

// requirePlainUpload refuses the accident category on the plain upload: a
// report is recorded with its fields and carries its paper copy on its own
// route, so a loose file there would have no status and no lock.
func requirePlainUpload(category string) error {
	if careplan.IsAccidentStudentDocumentCategory(category) {
		return fmt.Errorf("%w: accident reports are recorded, not uploaded", careplan.ErrStudentDocumentInvalid)
	}
	return nil
}

func (s *StudentDocuments) requireWritePermission(category string, actor careplan.StudentDocumentActor) error {
	if err := s.requireCategoryPermission(category, actor); err != nil {
		return err
	}
	if !s.access.HasPermission(s.writePermission(category), actor.Permissions) {
		return fmt.Errorf("%w: %s", careplan.ErrStudentDocumentForbidden, category)
	}
	return nil
}

// visibleCategories filters the categories down to what the actor may see.
func (s *StudentDocuments) visibleCategories(actor careplan.StudentDocumentActor) []string {
	visible := make([]string, 0, len(careplan.StudentDocumentCategories))
	for _, category := range careplan.StudentDocumentCategories {
		if s.access.HasPermission(s.categoryPermission(category), actor.Permissions) {
			visible = append(visible, category)
		}
	}
	return visible
}

func (s *StudentDocuments) CanSeeStudentDocumentCategory(category string, granted []string) bool {
	return s.access.HasPermission(s.categoryPermission(category), granted)
}

func (s *StudentDocuments) CanSeeEveryStudentDocumentCategory(granted []string) bool {
	for _, category := range careplan.StudentDocumentCategories {
		if !s.CanSeeStudentDocumentCategory(category, granted) {
			return false
		}
	}
	return true
}

func (s *StudentDocuments) requireCategoryPermission(category string, actor careplan.StudentDocumentActor) error {
	if !s.access.HasPermission(s.categoryPermission(category), actor.Permissions) {
		return fmt.Errorf("%w: %s", careplan.ErrStudentDocumentForbidden, category)
	}
	return nil
}

func (s *StudentDocuments) ListStudentDocuments(ctx context.Context, studentID int64, category string, actor careplan.StudentDocumentActor) ([]careplan.CareDocument, []string, error) {
	if err := s.requireStudentAccess(ctx, studentID, actor); err != nil {
		return nil, nil, err
	}
	visible := s.visibleCategories(actor)
	query := visible
	if category != "" {
		if !careplan.IsValidStudentDocumentCategory(category) {
			return nil, nil, fmt.Errorf("%w: unknown category", careplan.ErrStudentDocumentInvalid)
		}
		if err := s.requireCategoryPermission(category, actor); err != nil {
			return nil, nil, err
		}
		query = []string{category}
	}
	documents, err := s.records.ListCareDocuments(ctx, studentID, query)
	if err != nil {
		return nil, nil, fmt.Errorf("care plan documents: list student documents: %w", err)
	}
	return documents, visible, nil
}

// AuthorizeStudentDocumentUpload lets the upload handler refuse before it
// puts bytes on disk or a cleanup intent in the database. The create command
// repeats both checks inside its own transaction and stays authoritative.
func (s *StudentDocuments) AuthorizeStudentDocumentUpload(ctx context.Context, studentID int64, category string, actor careplan.StudentDocumentActor) error {
	if !careplan.IsValidStudentDocumentCategory(category) {
		return fmt.Errorf("%w: unknown category", careplan.ErrStudentDocumentInvalid)
	}
	if err := requirePlainUpload(category); err != nil {
		return err
	}
	if err := s.requireWritePermission(category, actor); err != nil {
		return err
	}
	return s.transaction(ctx, func(txCtx context.Context) error {
		return s.requireStudentAccess(txCtx, studentID, actor)
	})
}

func (s *StudentDocuments) CreateStudentDocument(ctx context.Context, input careplan.CreateStudentDocumentInput, actor careplan.StudentDocumentActor) (careplan.CareDocument, error) {
	if actor.AccountID <= 0 {
		return careplan.CareDocument{}, errors.New("actor account id is required")
	}
	if !careplan.IsValidStudentDocumentCategory(input.Category) {
		return careplan.CareDocument{}, fmt.Errorf("%w: unknown category", careplan.ErrStudentDocumentInvalid)
	}
	if err := requirePlainUpload(input.Category); err != nil {
		return careplan.CareDocument{}, err
	}
	if err := s.requireWritePermission(input.Category, actor); err != nil {
		return careplan.CareDocument{}, err
	}
	document := careplan.CareDocument{
		StudentID: input.StudentID, Category: input.Category,
		FilenameDisplay: strings.TrimSpace(input.FilenameDisplay), FilenameStored: input.FilenameStored,
		SizeBytes: input.SizeBytes, ContentType: input.ContentType, UploadedBy: actor.AccountID,
	}
	if document.FilenameDisplay == "" {
		return careplan.CareDocument{}, fmt.Errorf("%w: filename is required", careplan.ErrStudentDocumentInvalid)
	}
	if err := careplan.ValidateCareDocument(document); err != nil {
		return careplan.CareDocument{}, fmt.Errorf("%w: %s", careplan.ErrStudentDocumentInvalid, err.Error())
	}
	err := s.transaction(ctx, func(txCtx context.Context) error {
		if err := s.requireStudentAccess(txCtx, input.StudentID, actor); err != nil {
			return err
		}
		if err := s.recordChange(txCtx, input.StudentID, actor, input.Category, "", documentAuditValue(input.Category, document.FilenameDisplay)); err != nil {
			return err
		}
		created, err := s.records.CreateCareDocument(txCtx, document)
		if err != nil {
			return fmt.Errorf("care plan documents: create student document: %w", err)
		}
		document = created
		if err := s.records.CompleteCareDocumentCleanupByFilename(txCtx, input.FilenameStored); err != nil {
			return fmt.Errorf("complete document upload cleanup intent: %w", err)
		}
		return nil
	})
	if err != nil {
		return careplan.CareDocument{}, err
	}
	return document, nil
}

func (s *StudentDocuments) ResolveStudentDocumentDownload(ctx context.Context, studentID, documentID int64, actor careplan.StudentDocumentActor) (careplan.CareDocument, error) {
	var document careplan.CareDocument
	err := s.transaction(ctx, func(txCtx context.Context) error {
		if err := s.requireStudentAccess(txCtx, studentID, actor); err != nil {
			return err
		}
		found, err := s.records.FindCareDocument(txCtx, studentID, documentID, false)
		if err != nil {
			return err
		}
		if err := s.requireCategoryPermission(found.Category, actor); err != nil {
			return err
		}
		// An accident report without a paper copy has no file; its PDF comes
		// from the report route. Nothing is served, so nothing is logged.
		if found.FilenameStored == "" {
			return careplan.ErrCareDocumentNotFound
		}
		if err := s.recordSensitiveDownload(txCtx, studentID, actor, found); err != nil {
			return err
		}
		document = found
		return nil
	})
	if err != nil {
		return careplan.CareDocument{}, err
	}
	return document, nil
}

// recordSensitiveDownload writes the data-access log row a sensitive
// category requires. The download is refused when it cannot be written.
func (s *StudentDocuments) recordSensitiveDownload(ctx context.Context, studentID int64, actor careplan.StudentDocumentActor, document careplan.CareDocument) error {
	if !careplan.IsSensitiveStudentDocumentCategory(document.Category) {
		return nil
	}
	if actor.AccountID <= 0 {
		return errors.New("actor account id is required for document downloads")
	}
	if actor.Role = strings.TrimSpace(actor.Role); actor.Role == "" {
		actor.Role = "unknown"
	}
	if err := s.audit.RecordDocumentDownload(ctx, studentID, actor, document, time.Now()); err != nil {
		return fmt.Errorf("write document access audit: %w", err)
	}
	return nil
}

// DeleteStudentDocument soft-deletes the metadata row with its change-history
// row. The caller removes the bytes only after this transaction commits.
func (s *StudentDocuments) DeleteStudentDocument(ctx context.Context, studentID, documentID int64, actor careplan.StudentDocumentActor) (careplan.CareDocument, error) {
	if actor.AccountID <= 0 {
		return careplan.CareDocument{}, errors.New("actor account id is required")
	}
	var deleted careplan.CareDocument
	err := s.transaction(ctx, func(txCtx context.Context) error {
		if err := s.requireStudentAccess(txCtx, studentID, actor); err != nil {
			return err
		}
		document, err := s.records.FindCareDocument(txCtx, studentID, documentID, false)
		if err != nil {
			return err
		}
		if err := s.requireWritePermission(document.Category, actor); err != nil {
			return err
		}
		// A signed accident report is a record that stays; only a
		// correction may follow it.
		if document.AccidentSignedAt != nil {
			return careplan.ErrAccidentReportSigned
		}
		at, err := s.records.SoftDeleteCareDocument(txCtx, document.ID, actor.AccountID)
		if err != nil {
			return err
		}
		deletedBy := actor.AccountID
		document.DeletedAt, document.DeletedBy = &at, &deletedBy
		if err := s.recordChange(txCtx, studentID, actor, document.Category, documentAuditValue(document.Category, document.FilenameDisplay), ""); err != nil {
			return err
		}
		deleted = document
		return nil
	})
	if err != nil {
		return careplan.CareDocument{}, err
	}
	return deleted, nil
}

func (s *StudentDocuments) ResolveStudentDocumentCleanup(ctx context.Context, studentID, documentID int64, actor careplan.StudentDocumentActor) (careplan.CareDocument, error) {
	if err := s.requireStudentAccess(ctx, studentID, actor); err != nil {
		return careplan.CareDocument{}, err
	}
	document, err := s.records.FindCareDocument(ctx, studentID, documentID, true)
	if err != nil {
		return careplan.CareDocument{}, err
	}
	if err := s.requireCategoryPermission(document.Category, actor); err != nil {
		return careplan.CareDocument{}, err
	}
	return document, nil
}

func (s *StudentDocuments) ListDeletedStudentDocumentsPendingFileCleanup(ctx context.Context, studentID int64, actor careplan.StudentDocumentActor) ([]careplan.CareDocument, error) {
	if err := s.requireStudentAccess(ctx, studentID, actor); err != nil {
		return nil, err
	}
	documents, err := s.records.ListDeletedCareDocuments(ctx, studentID, s.visibleCategories(actor))
	if err != nil {
		return nil, fmt.Errorf("care plan documents: list deleted student documents: %w", err)
	}
	return documents, nil
}

// recordChange writes one document event into the child's change history,
// with the editor's name snapshot so the history still reads correctly after
// a rename or an account deletion.
func (s *StudentDocuments) recordChange(ctx context.Context, studentID int64, actor careplan.StudentDocumentActor, category, oldValue, newValue string) error {
	if actor.Name = strings.TrimSpace(actor.Name); actor.Name == "" {
		actor.Name = "Unbekannt"
	}
	if err := s.audit.RecordDocumentChange(ctx, studentID, actor, category, oldValue, newValue); err != nil {
		return fmt.Errorf("write document audit: %w", err)
	}
	return nil
}

// documentAuditValue renders the history cell: the German category label
// plus the display filename.
func documentAuditValue(category, filename string) string {
	label := careplan.StudentDocumentCategoryLabels[category]
	if label == "" {
		label = category
	}
	return label + ": " + filename
}
