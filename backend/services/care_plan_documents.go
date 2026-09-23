package services

import (
	"context"
	"errors"
	"time"

	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	carePlanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
	"github.com/uptrace/bun"
)

// studentDocumentAccess answers Care Plan's document authority questions
// with the security runtime's rules over the People Directory row.
type studentDocumentAccess struct {
	students    userModels.StudentRepository
	userContext securityruntime.StudentAccessUserContext
}

func (a studentDocumentAccess) HasPermission(required string, granted []string) bool {
	return securityruntime.HasPermission(required, granted)
}

// CanAccessStudent is the per-child gate every other per-child endpoint
// applies: an administrator, or verified staff of the tenant (#2329).
func (a studentDocumentAccess) CanAccessStudent(ctx context.Context, studentID int64, granted []string) (bool, error) {
	student, err := a.students.FindByID(ctx, studentID)
	if err != nil {
		return false, err
	}
	return securityruntime.CanModifyStudent(ctx, granted, student, a.userContext), nil
}

// studentDocumentAudit writes Care Plan's document trail into the Audit
// Platform: the child's change history and the data-access log.
type studentDocumentAudit struct {
	edits     auditModels.StudentFieldEditRepository
	accessLog auditModels.DataAccessLogRepository
}

func (a studentDocumentAudit) RecordDocumentChange(ctx context.Context, studentID int64, actor careplan.StudentDocumentActor, category, oldValue, newValue string) error {
	edit := &auditModels.StudentFieldEdit{
		StudentID: studentID, EditedBy: actor.AccountID, EditedByName: actor.Name,
		FieldName: auditModels.StudentDocumentField(category),
	}
	if oldValue != "" {
		edit.OldValue = &oldValue
	}
	if newValue != "" {
		edit.NewValue = &newValue
	}
	return a.edits.CreateBatch(ctx, []*auditModels.StudentFieldEdit{edit})
}

// RecordDocumentDownload logs one sensitive download. The child goes in the
// student_id COLUMN, not into the metadata: it is the column a per-child
// disclosure report reads, and its foreign key clears it when the child is
// deleted, where a copy in the JSONB would survive.
func (a studentDocumentAudit) RecordDocumentDownload(ctx context.Context, studentID int64, actor careplan.StudentDocumentActor, document careplan.CareDocument, at time.Time) error {
	subject := studentID
	return a.accessLog.Create(ctx, &auditModels.DataAccessLog{
		ActorAccountID: actor.AccountID,
		ActorRole:      actor.Role,
		ResourceType:   auditModels.ResourceTypeStudentDocumentDownload,
		StudentID:      &subject,
		RangeStart:     at,
		RangeEnd:       at,
		AccessedAt:     at,
		Metadata: map[string]interface{}{
			"document_id": document.ID,
			"category":    document.Category,
		},
	})
}

// newStudentDocuments composes Care Plan's child document capability with
// its authority and audit bound. It refuses to compose without either audit
// repository: an unlogged upload, deletion or sensitive download of a child's
// paperwork is worse than a server that does not start.
func newStudentDocuments(
	db *bun.DB, records careplan.Capability, students userModels.StudentRepository,
	userContext securityruntime.StudentAccessUserContext,
	edits auditModels.StudentFieldEditRepository, accessLog auditModels.DataAccessLogRepository,
) (careplan.StudentDocuments, error) {
	if students == nil || userContext == nil {
		return nil, errors.New("student documents: student repository and user context are required")
	}
	if edits == nil || accessLog == nil {
		return nil, errors.New("student documents: change history and data access log are required; refusing unaudited changes")
	}
	return carePlanCompose.NewStudentDocuments(carePlanCompose.StudentDocumentDependencies{
		DB: db, Records: records,
		Access: studentDocumentAccess{students: students, userContext: userContext},
		Permissions: carePlanCompose.StudentDocumentPermissions{
			Health:  securityruntime.PermissionStudentDocumentsHealth,
			Legal:   securityruntime.PermissionStudentDocumentsLegal,
			Default: securityruntime.PermissionUsersUpdate,
		},
		Audit: studentDocumentAudit{edits: edits, accessLog: accessLog},
	})
}
