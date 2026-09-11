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
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// Student documents (#777, Dokumente tab of a child's profile). The route
// gate only proves the caller may reach the tab at all — per-category
// authority is decided here: health documents need student_documents:health
// (Art. 9 GDPR), custody paperwork needs student_documents:legal, everything
// else belongs to the directory maintainers (users:update).
//
// Two rules are hard, mirroring the staff document service: every upload and
// delete writes an audit row in the same tenant transaction, and serving a
// sensitive category is refused outright when the data-access log write
// fails. An unlogged look at a child's medical certificate is worse than a
// failed request.

const (
	// StudentDocumentUploadDeadline bounds every step an upload request may
	// still perform once its cleanup intent exists: the object write and the
	// metadata transaction. The handler enforces it, so no request can persist
	// a document row after its queued intent became eligible for the cleanup
	// scheduler.
	StudentDocumentUploadDeadline = 2 * time.Minute

	// studentDocumentCleanupDelay keeps a queued intent ineligible until well
	// past StudentDocumentUploadDeadline. The difference is deliberate slack
	// for a metadata transaction whose commit was still in flight when the
	// deadline fired. It MUST stay larger than StudentDocumentUploadDeadline.
	studentDocumentCleanupDelay = 5 * time.Minute
)

// ErrStudentDocumentInvalid marks a semantically invalid payload — HTTP 400.
var ErrStudentDocumentInvalid = errors.New("invalid student document")

// ErrStudentDocumentForbidden marks a category the caller's permissions do
// not cover — HTTP 403.
var ErrStudentDocumentForbidden = errors.New("student document category not permitted")

// ErrStudentDocumentNoAccess marks a child the caller may not reach at all —
// HTTP 403. Separate from the category error because it answers a different
// question: not "which drawer" but "whose file cabinet".
var ErrStudentDocumentNoAccess = errors.New("no access to this child")

// StudentDocumentActor identifies the acting account for permission checks
// and audit rows.
type StudentDocumentActor struct {
	AccountID   int64
	Name        string
	Role        string
	Permissions []string
}

// CreateStudentDocumentInput carries the metadata of an already-stored
// upload. The handler owns the bytes (parse, magic-number validation, storage
// write); the service owns authority, the metadata row and the audit trail.
type CreateStudentDocumentInput struct {
	StudentID       int64
	Category        string
	FilenameDisplay string
	FilenameStored  string
	SizeBytes       int64
	ContentType     string
}

// StudentDocumentService is the business boundary of the child Dokumente tab.
type StudentDocumentService interface {
	// ListStudentDocuments returns the documents the actor may see, newest
	// first, plus the actor's visible categories (the frontend builds its
	// filter and upload choices from them).
	ListStudentDocuments(ctx context.Context, studentID int64, category string, actor StudentDocumentActor) ([]*userModels.StudentDocument, []string, error)
	// AuthorizeStudentDocumentUpload answers "may this caller add a document
	// of this category to this child" WITHOUT writing anything. The upload
	// handler calls it before it puts bytes on disk or a cleanup intent in
	// the database, so an unauthorized request costs nothing but the request
	// itself. CreateStudentDocument repeats both checks inside its own
	// transaction, which stays the authoritative one.
	AuthorizeStudentDocumentUpload(ctx context.Context, studentID int64, category string, actor StudentDocumentActor) error
	// CreateStudentDocument persists the metadata row and the audit entry in
	// one tenant transaction.
	CreateStudentDocument(ctx context.Context, input CreateStudentDocumentInput, actor StudentDocumentActor) (*userModels.StudentDocument, error)
	// ResolveStudentDocumentDownload authorizes the download and, for the
	// sensitive categories, writes the data-access log row. No file is served
	// when the returned error is non-nil.
	ResolveStudentDocumentDownload(ctx context.Context, studentID, documentID int64, actor StudentDocumentActor) (*userModels.StudentDocument, error)
	// DeleteStudentDocument soft-deletes the metadata row and writes the audit
	// entry in one tenant transaction. The handler removes the bytes only
	// after this transaction commits.
	DeleteStudentDocument(ctx context.Context, studentID, documentID int64, actor StudentDocumentActor) (*userModels.StudentDocument, error)
	// ResolveStudentDocumentCleanup authorizes a retry of the storage cleanup
	// for a document that may already be soft-deleted.
	ResolveStudentDocumentCleanup(ctx context.Context, studentID, documentID int64, actor StudentDocumentActor) (*userModels.StudentDocument, error)
	// ListDeletedStudentDocumentsPendingFileCleanup returns authorized retry
	// candidates whose previous unlink did not complete.
	ListDeletedStudentDocumentsPendingFileCleanup(ctx context.Context, studentID int64, actor StudentDocumentActor) ([]*userModels.StudentDocument, error)
	// ListDeletedStudentDocumentsPendingFileCleanups returns every
	// soft-deleted document in the tenant whose bytes need scheduler recovery.
	ListDeletedStudentDocumentsPendingFileCleanups(ctx context.Context) ([]*userModels.StudentDocument, error)
	// QueueStudentDocumentFileCleanup durably records the cleanup intent
	// before the object is written.
	QueueStudentDocumentFileCleanup(ctx context.Context, studentID int64, storedName string) error
	// QueueCleanupForAllDocuments queues an immediately-eligible intent for
	// every document of a child whose bytes still exist. It is the step that
	// keeps deleting a child from stranding their documents on disk: the
	// document rows cascade away, the intents do not.
	QueueCleanupForAllDocuments(ctx context.Context, studentID int64) error
	// ListQueuedStudentDocumentFileCleanups returns orphaned upload objects in
	// the tenant so retries do not depend on the referenced child.
	ListQueuedStudentDocumentFileCleanups(ctx context.Context) ([]*userModels.StudentDocumentFileCleanup, error)

	// The remaining methods implement the coordinator's Store interface.
	MarkFileDeleted(ctx context.Context, documentID int64) error
	MarkQueuedCleanupComplete(ctx context.Context, cleanupID int64) error
	MarkQueuedCleanupCompleteByFilename(ctx context.Context, storedName string) error
	ActivateQueuedCleanup(ctx context.Context, storedName string) error
}

// StudentDocumentUserContext is the slice of the user context the document
// service needs to answer "is this caller verified staff". It is
// authorize.StudentAccessUserContext, restated so the service depends on a
// named interface rather than a package alias.
type StudentDocumentUserContext interface {
	GetCurrentStaff(ctx context.Context) (*userModels.Staff, error)
	HasCurrentStaff(ctx context.Context) (bool, error)
}

type studentDocumentService struct {
	db          *bun.DB
	documents   userModels.StudentDocumentRepository
	students    userModels.StudentRepository
	audit       auditModels.StudentFieldEditRepository
	accessLog   auditModels.DataAccessLogRepository
	userContext StudentDocumentUserContext
	logger      *slog.Logger
}

// NewStudentDocumentService wires the child document service.
func NewStudentDocumentService(
	db *bun.DB,
	documents userModels.StudentDocumentRepository,
	students userModels.StudentRepository,
	audit auditModels.StudentFieldEditRepository,
	accessLog auditModels.DataAccessLogRepository,
	userContext StudentDocumentUserContext,
	logger *slog.Logger,
) StudentDocumentService {
	return &studentDocumentService{
		db:          db,
		documents:   documents,
		students:    students,
		audit:       audit,
		accessLog:   accessLog,
		userContext: userContext,
		logger:      logger,
	}
}

// requireStudentAccess enforces the per-child gate every other per-child
// endpoint applies: admin, or verified staff of the tenant (#2329). The route
// gate only proves the caller may reach documents at all — without this, a
// guest or guardian account could read and delete the paperwork of every
// child in the school.
//
// It lives here rather than in the handler because the upload and download
// routes deliberately run without the tenant-transaction middleware (a slow
// 10 MB body must not pin a pool connection), and this check needs to happen
// inside the same transaction as the work it guards.
func (s *studentDocumentService) requireStudentAccess(ctx context.Context, studentID int64, actor StudentDocumentActor) (*userModels.Student, error) {
	student, err := s.students.FindByID(ctx, studentID)
	if err != nil {
		return nil, err
	}
	if ok, _ := authorize.CanModifyStudent(ctx, actor.Permissions, student, s.userContext, "access documents of"); !ok {
		return nil, ErrStudentDocumentNoAccess
	}
	return student, nil
}

// studentDocumentCategoryPermission returns the permission a category
// requires. The mapping is the whole point of the permission design — keep it
// in one place.
func studentDocumentCategoryPermission(category string) string {
	switch {
	case userModels.IsHealthStudentDocumentCategory(category):
		return permissions.StudentDocumentsHealth
	case userModels.IsLegalStudentDocumentCategory(category):
		return permissions.StudentDocumentsLegal
	default:
		return permissions.UsersUpdate
	}
}

// studentDocumentCategorySensitive reports whether serving the category's
// content requires a data-access log row.
func studentDocumentCategorySensitive(category string) bool {
	return userModels.IsHealthStudentDocumentCategory(category) ||
		userModels.IsLegalStudentDocumentCategory(category)
}

// visibleStudentDocumentCategories filters the category enum down to what the
// actor may see (wildcard-aware, so admin:* matches everything).
func visibleStudentDocumentCategories(actor StudentDocumentActor) []string {
	visible := make([]string, 0, len(userModels.StudentDocumentCategories))
	for _, category := range userModels.StudentDocumentCategories {
		if authorize.HasPermission(studentDocumentCategoryPermission(category), actor.Permissions) {
			visible = append(visible, category)
		}
	}
	return visible
}

// CanSeeStudentDocumentCategory reports whether the given permissions cover a
// document category. Exported so readers of derived data — the per-child
// change history above all — can apply the same filter the Dokumente tab does,
// instead of becoming a way around it.
func CanSeeStudentDocumentCategory(category string, userPermissions []string) bool {
	return authorize.HasPermission(studentDocumentCategoryPermission(category), userPermissions)
}

// CanSeeEveryStudentDocumentCategory reports whether the permissions cover
// every category. Readers use it for rows whose category is unknown.
func CanSeeEveryStudentDocumentCategory(userPermissions []string) bool {
	for _, category := range userModels.StudentDocumentCategories {
		if !CanSeeStudentDocumentCategory(category, userPermissions) {
			return false
		}
	}
	return true
}

func (s *studentDocumentService) requireCategoryPermission(category string, actor StudentDocumentActor) error {
	if !authorize.HasPermission(studentDocumentCategoryPermission(category), actor.Permissions) {
		return fmt.Errorf("%w: %s", ErrStudentDocumentForbidden, category)
	}
	return nil
}

func (s *studentDocumentService) ListStudentDocuments(ctx context.Context, studentID int64, category string, actor StudentDocumentActor) ([]*userModels.StudentDocument, []string, error) {
	if _, err := s.requireStudentAccess(ctx, studentID, actor); err != nil {
		return nil, nil, err
	}

	visible := visibleStudentDocumentCategories(actor)
	query := visible
	if category != "" {
		if !userModels.IsValidStudentDocumentCategory(category) {
			return nil, nil, fmt.Errorf("%w: unknown category", ErrStudentDocumentInvalid)
		}
		if err := s.requireCategoryPermission(category, actor); err != nil {
			return nil, nil, err
		}
		query = []string{category}
	}

	docs, err := s.documents.ListByOwnerID(ctx, studentID, query)
	if err != nil {
		return nil, nil, err
	}
	return docs, visible, nil
}

func (s *studentDocumentService) AuthorizeStudentDocumentUpload(ctx context.Context, studentID int64, category string, actor StudentDocumentActor) error {
	if !userModels.IsValidStudentDocumentCategory(category) {
		return fmt.Errorf("%w: unknown category", ErrStudentDocumentInvalid)
	}
	if err := s.requireCategoryPermission(category, actor); err != nil {
		return err
	}
	tenantID := tenant.FromContext(ctx)
	return tenant.WithTenantTx(ctx, s.db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		_, err := s.requireStudentAccess(ctx, studentID, actor)
		return err
	})
}

func (s *studentDocumentService) CreateStudentDocument(ctx context.Context, input CreateStudentDocumentInput, actor StudentDocumentActor) (*userModels.StudentDocument, error) {
	if s.audit == nil {
		return nil, fmt.Errorf("student audit repository is not wired; refusing unaudited change")
	}
	if actor.AccountID <= 0 {
		return nil, fmt.Errorf("actor account id is required")
	}
	if !userModels.IsValidStudentDocumentCategory(input.Category) {
		return nil, fmt.Errorf("%w: unknown category", ErrStudentDocumentInvalid)
	}
	if err := s.requireCategoryPermission(input.Category, actor); err != nil {
		return nil, err
	}
	input.FilenameDisplay = strings.TrimSpace(input.FilenameDisplay)
	if input.FilenameDisplay == "" {
		return nil, fmt.Errorf("%w: filename is required", ErrStudentDocumentInvalid)
	}

	doc := &userModels.StudentDocument{StudentID: input.StudentID}
	doc.Category = input.Category
	doc.FilenameDisplay = input.FilenameDisplay
	doc.FilenameStored = input.FilenameStored
	doc.SizeBytes = input.SizeBytes
	doc.ContentType = input.ContentType
	doc.UploadedBy = actor.AccountID
	if err := doc.Validate(); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrStudentDocumentInvalid, err.Error())
	}

	tenantID := tenant.FromContext(ctx)
	err := tenant.WithTenantTx(ctx, s.db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		if _, err := s.requireStudentAccess(ctx, input.StudentID, actor); err != nil {
			return err
		}
		if err := s.recordAudit(ctx, input.StudentID, actor, input.Category, "", documentAuditValue(input.Category, input.FilenameDisplay)); err != nil {
			return err
		}
		if err := s.documents.Create(ctx, doc); err != nil {
			return err
		}
		if err := s.documents.MarkQueuedFileCleanupCompleteByFilename(ctx, input.FilenameStored); err != nil {
			return fmt.Errorf("complete document upload cleanup intent: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return doc, nil
}

func (s *studentDocumentService) ResolveStudentDocumentDownload(ctx context.Context, studentID, documentID int64, actor StudentDocumentActor) (*userModels.StudentDocument, error) {
	var document *userModels.StudentDocument
	tenantID := tenant.FromContext(ctx)
	err := tenant.WithTenantTx(ctx, s.db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		if _, err := s.requireStudentAccess(ctx, studentID, actor); err != nil {
			return err
		}
		doc, err := s.documents.FindForOwner(ctx, studentID, documentID)
		if err != nil {
			return err
		}
		if err := s.requireCategoryPermission(doc.Category, actor); err != nil {
			return err
		}

		if studentDocumentCategorySensitive(doc.Category) {
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
			// The child goes in the student_id COLUMN, not into metadata. It
			// is the column a per-child disclosure report reads, and its FK
			// carries ON DELETE SET NULL — a copy in the JSONB would survive
			// the child's deletion, which is the leftover that column exists
			// to prevent. Staff-document downloads leave it NULL legitimately,
			// because there no child is involved; here one is.
			subject := studentID
			if err := s.accessLog.Create(ctx, &auditModels.DataAccessLog{
				ActorAccountID: actor.AccountID,
				ActorRole:      role,
				ResourceType:   auditModels.ResourceTypeStudentDocumentDownload,
				StudentID:      &subject,
				RangeStart:     now,
				RangeEnd:       now,
				AccessedAt:     now,
				Metadata: map[string]interface{}{
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

func (s *studentDocumentService) DeleteStudentDocument(ctx context.Context, studentID, documentID int64, actor StudentDocumentActor) (*userModels.StudentDocument, error) {
	if s.audit == nil {
		return nil, fmt.Errorf("student audit repository is not wired; refusing unaudited change")
	}
	if actor.AccountID <= 0 {
		return nil, fmt.Errorf("actor account id is required")
	}

	var deleted *userModels.StudentDocument
	tenantID := tenant.FromContext(ctx)
	err := tenant.WithTenantTx(ctx, s.db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		if _, err := s.requireStudentAccess(ctx, studentID, actor); err != nil {
			return err
		}
		doc, err := s.documents.FindForOwner(ctx, studentID, documentID)
		if err != nil {
			return err
		}
		if err := s.requireCategoryPermission(doc.Category, actor); err != nil {
			return err
		}
		if err := s.documents.SoftDelete(ctx, doc, actor.AccountID); err != nil {
			return err
		}
		if err := s.recordAudit(ctx, studentID, actor, doc.Category, documentAuditValue(doc.Category, doc.FilenameDisplay), ""); err != nil {
			return err
		}
		deleted = doc
		return nil
	})
	if err != nil {
		return nil, err
	}
	return deleted, nil
}

func (s *studentDocumentService) ResolveStudentDocumentCleanup(ctx context.Context, studentID, documentID int64, actor StudentDocumentActor) (*userModels.StudentDocument, error) {
	if _, err := s.requireStudentAccess(ctx, studentID, actor); err != nil {
		return nil, err
	}
	doc, err := s.documents.FindForOwnerIncludingDeleted(ctx, studentID, documentID)
	if err != nil {
		return nil, err
	}
	if err := s.requireCategoryPermission(doc.Category, actor); err != nil {
		return nil, err
	}
	return doc, nil
}

func (s *studentDocumentService) ListDeletedStudentDocumentsPendingFileCleanup(ctx context.Context, studentID int64, actor StudentDocumentActor) ([]*userModels.StudentDocument, error) {
	if _, err := s.requireStudentAccess(ctx, studentID, actor); err != nil {
		return nil, err
	}
	return s.documents.ListDeletedPendingFileCleanupByOwnerID(ctx, studentID, visibleStudentDocumentCategories(actor))
}

func (s *studentDocumentService) ListDeletedStudentDocumentsPendingFileCleanups(ctx context.Context) ([]*userModels.StudentDocument, error) {
	return s.documents.ListDeletedPendingFileCleanups(ctx)
}

func (s *studentDocumentService) QueueStudentDocumentFileCleanup(ctx context.Context, studentID int64, storedName string) error {
	if studentID <= 0 || strings.TrimSpace(storedName) == "" {
		return fmt.Errorf("%w: cleanup file details are required", ErrStudentDocumentInvalid)
	}
	return s.queueCleanup(ctx, studentID, storedName, time.Now().Add(studentDocumentCleanupDelay))
}

func (s *studentDocumentService) QueueCleanupForAllDocuments(ctx context.Context, studentID int64) error {
	docs, err := s.documents.ListPendingFileCleanupByOwnerID(ctx, studentID)
	if err != nil {
		return err
	}
	for _, doc := range docs {
		// Eligible immediately: the child is being deleted, so no upload for
		// these objects can still be in flight.
		if err := s.queueCleanup(ctx, studentID, doc.FilenameStored, time.Now()); err != nil {
			return err
		}
	}
	return nil
}

func (s *studentDocumentService) queueCleanup(ctx context.Context, studentID int64, storedName string, retryAfter time.Time) error {
	tenantID := tenant.FromContext(ctx)
	cleanup := &userModels.StudentDocumentFileCleanup{}
	cleanup.OwnerID = studentID
	cleanup.FilenameStored = storedName
	cleanup.RetryAfter = retryAfter
	return tenant.WithTenantTx(ctx, s.db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		return s.documents.QueueFileCleanup(ctx, cleanup)
	})
}

func (s *studentDocumentService) ListQueuedStudentDocumentFileCleanups(ctx context.Context) ([]*userModels.StudentDocumentFileCleanup, error) {
	return s.documents.ListQueuedFileCleanups(ctx)
}

func (s *studentDocumentService) MarkFileDeleted(ctx context.Context, documentID int64) error {
	tenantID := tenant.FromContext(ctx)
	return tenant.WithTenantTx(ctx, s.db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		return s.documents.MarkFileDeleted(ctx, documentID)
	})
}

func (s *studentDocumentService) MarkQueuedCleanupComplete(ctx context.Context, cleanupID int64) error {
	tenantID := tenant.FromContext(ctx)
	return tenant.WithTenantTx(ctx, s.db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		return s.documents.MarkQueuedFileCleanupComplete(ctx, cleanupID)
	})
}

func (s *studentDocumentService) MarkQueuedCleanupCompleteByFilename(ctx context.Context, storedName string) error {
	tenantID := tenant.FromContext(ctx)
	return tenant.WithTenantTx(ctx, s.db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		return s.documents.MarkQueuedFileCleanupCompleteByFilename(ctx, storedName)
	})
}

func (s *studentDocumentService) ActivateQueuedCleanup(ctx context.Context, storedName string) error {
	tenantID := tenant.FromContext(ctx)
	return tenant.WithTenantTx(ctx, s.db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		return s.documents.ActivateQueuedFileCleanupByFilename(ctx, storedName)
	})
}

// recordAudit writes one document event into the child's change history.
func (s *studentDocumentService) recordAudit(ctx context.Context, studentID int64, actor StudentDocumentActor, category, oldValue, newValue string) error {
	actorName := strings.TrimSpace(actor.Name)
	if actorName == "" {
		actorName = "Unbekannt"
	}
	edit := &auditModels.StudentFieldEdit{
		StudentID:    studentID,
		EditedBy:     actor.AccountID,
		EditedByName: actorName,
		FieldName:    auditModels.StudentDocumentField(category),
	}
	if oldValue != "" {
		edit.OldValue = &oldValue
	}
	if newValue != "" {
		edit.NewValue = &newValue
	}
	if err := s.audit.CreateBatch(ctx, []*auditModels.StudentFieldEdit{edit}); err != nil {
		return fmt.Errorf("write document audit: %w", err)
	}
	return nil
}

// documentAuditValue renders the audit cell: the German category label plus
// the display filename.
func documentAuditValue(category, filename string) string {
	label := userModels.StudentDocumentCategoryLabels[category]
	if label == "" {
		label = category
	}
	return label + ": " + filename
}
