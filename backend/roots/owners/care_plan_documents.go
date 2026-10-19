package owners

import (
	"context"
	"errors"
	"time"

	peopleValues "github.com/moto-nrw/project-phoenix/modules/peopledirectory"

	auditModels "github.com/moto-nrw/project-phoenix/modules/auditlog/records"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	carePlanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"

	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
)

// studentDocumentAccess answers Care Plan's document authority questions
// with the security runtime's rules over the People Directory row.
type studentDocumentAccess struct {
	students    studentDocumentStudents
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
	edits     studentDocumentEdits
	accessLog studentDocumentAccessLog
}

// studentDocumentStudents reads the child row the document gate decides on.
type studentDocumentStudents interface {
	FindByID(ctx context.Context, id any) (*peopleValues.StudentState, error)
}

// studentDocumentEdits and studentDocumentAccessLog are the Audit Platform
// trails of the documents.
type studentDocumentEdits interface {
	CreateBatch(ctx context.Context, edits []*auditModels.StudentFieldEdit) error
}

type studentDocumentAccessLog interface {
	Create(ctx context.Context, entry *auditModels.DataAccessLog) error
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

// StudentDocumentDependencies binds Care Plan's child document capability to
// its authority and audit. It refuses to compose without either audit
// repository: an unlogged upload, deletion or sensitive download of a child's
// paperwork is worse than a server that does not start. The composition root
// adds its database handle to the dependencies they bind.
func StudentDocumentDependencies(
	records careplan.Capability, students studentDocumentStudents,
	userContext securityruntime.StudentAccessUserContext,
	edits studentDocumentEdits, accessLog studentDocumentAccessLog, documentPermissions carePlanCompose.StudentDocumentPermissions,
) (carePlanCompose.StudentDocumentDependencies, error) {
	if students == nil || userContext == nil {
		return carePlanCompose.StudentDocumentDependencies{}, errors.New("student documents: student repository and user context are required")
	}
	if edits == nil || accessLog == nil {
		return carePlanCompose.StudentDocumentDependencies{}, errors.New("student documents: change history and data access log are required; refusing unaudited changes")
	}
	return carePlanCompose.StudentDocumentDependencies{
		Records:     records,
		Access:      studentDocumentAccess{students: students, userContext: userContext},
		Permissions: documentPermissions,
		Audit:       studentDocumentAudit{edits: edits, accessLog: accessLog},
	}, nil
}
