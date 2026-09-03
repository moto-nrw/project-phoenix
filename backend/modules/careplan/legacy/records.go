package legacy

import (
	"context"
	"errors"

	usersRepo "github.com/moto-nrw/project-phoenix/database/repositories/users"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
)

type companionRepository struct{ capability careplan.Capability }

var _ userModels.StudentCompanionRepository = companionRepository{}

type CompanionRepository struct{ companionRepository }

func NewCompanionRepository(capability careplan.Capability) *CompanionRepository {
	return &CompanionRepository{companionRepository{capability: capability}}
}

func (r companionRepository) ListForStudent(ctx context.Context, studentID int64) ([]*userModels.StudentCompanion, error) {
	values, err := r.capability.ListCompanionEdges(ctx, studentID)
	if err != nil {
		return nil, usersRepo.WrapError("list student companions", err)
	}
	result := make([]*userModels.StudentCompanion, 0, len(values))
	for _, value := range values {
		result = append(result, companionToLegacy(value))
	}
	return result, nil
}

func (r companionRepository) ListLinksForStudent(ctx context.Context, studentID int64) ([]userModels.CompanionLink, error) {
	values, err := r.ListLinksForStudents(ctx, []int64{studentID})
	if err != nil {
		return nil, err
	}
	return values[studentID], nil
}

func (r companionRepository) ListLinksForStudents(ctx context.Context, studentIDs []int64) (map[int64][]userModels.CompanionLink, error) {
	values, err := r.capability.ListCompanionLinks(ctx, studentIDs)
	if err != nil {
		return nil, usersRepo.WrapError("list student companions", err)
	}
	result := make(map[int64][]userModels.CompanionLink, len(values))
	for studentID, links := range values {
		converted := make([]userModels.CompanionLink, 0, len(links))
		for _, link := range links {
			converted = append(converted, userModels.CompanionLink{CompanionStudentID: link.CompanionStudentID, FirstName: link.FirstName, LastName: link.LastName, Weekdays: link.Weekdays})
		}
		result[studentID] = converted
	}
	return result, nil
}

func (r companionRepository) ReplaceForStudent(ctx context.Context, studentID int64, edges []*userModels.StudentCompanion) error {
	values := make([]careplan.CompanionEdge, 0, len(edges))
	for _, edge := range edges {
		if edge == nil {
			return errors.New("companion edge cannot be nil")
		}
		values = append(values, companionToPublic(edge))
	}
	return usersRepo.WrapError("replace student companions", r.capability.ReplaceCompanionEdges(ctx, studentID, values))
}

func (r companionRepository) CompanionIDsForWeekday(ctx context.Context, studentIDs []int64, weekday int) (map[int64][]int64, error) {
	if _, ok := userModels.CompanionWeekdayKeys[weekday]; !ok {
		return nil, userModels.ErrCompanionInvalidWeekday
	}
	values, err := r.capability.CompanionIDsForWeekday(ctx, studentIDs, weekday)
	return values, usersRepo.WrapError("list companions for weekday", err)
}

func (r companionRepository) CompanionCountsExcluding(ctx context.Context, studentIDs []int64, excludeID int64) (map[int64]int, error) {
	values, err := r.capability.CompanionCountsExcluding(ctx, studentIDs, excludeID)
	return values, usersRepo.WrapError("count student companions", err)
}

func (r companionRepository) CompanionDaysCoveredExcluding(ctx context.Context, studentIDs []int64, excludeID int64) (map[int64]map[string]bool, error) {
	values, err := r.capability.CompanionDaysCoveredExcluding(ctx, studentIDs, excludeID)
	return values, usersRepo.WrapError("list student companion days", err)
}

func (r companionRepository) CompanionWeekdays(ctx context.Context, studentID int64) ([]int, error) {
	values, err := r.capability.CompanionWeekdays(ctx, studentID)
	return values, usersRepo.WrapError("check student companion links", err)
}

func (r companionRepository) DeleteEdges(ctx context.Context, edgeIDs []int64) error {
	return usersRepo.WrapError("delete student companion edges", r.capability.DeleteCompanionEdges(ctx, edgeIDs))
}

func companionToPublic(value *userModels.StudentCompanion) careplan.CompanionEdge {
	return careplan.CompanionEdge{ID: value.ID, TenantID: value.TenantID, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt, StudentLowID: value.StudentLowID, StudentHighID: value.StudentHighID, Weekday: value.Weekday}
}

func companionToLegacy(value careplan.CompanionEdge) *userModels.StudentCompanion {
	result := &userModels.StudentCompanion{StudentLowID: value.StudentLowID, StudentHighID: value.StudentHighID, Weekday: value.Weekday}
	result.ID, result.TenantID, result.CreatedAt, result.UpdatedAt = value.ID, value.TenantID, value.CreatedAt, value.UpdatedAt
	return result
}

type careDocumentRepository struct{ capability careplan.Capability }

var _ userModels.StudentDocumentRepository = careDocumentRepository{}

func NewCareDocumentRepository(capability careplan.Capability) userModels.StudentDocumentRepository {
	return careDocumentRepository{capability: capability}
}

func (r careDocumentRepository) Create(ctx context.Context, document *userModels.StudentDocument) error {
	if document == nil {
		return errors.New("StudentDocument cannot be nil")
	}
	if err := document.Validate(); err != nil {
		return err
	}
	value, err := r.capability.CreateCareDocument(ctx, careDocumentToPublic(document))
	if err != nil {
		return usersRepo.WrapError("create student_document", err)
	}
	applyCareDocument(document, value)
	return nil
}

func (r careDocumentRepository) FindForOwner(ctx context.Context, studentID, documentID int64) (*userModels.StudentDocument, error) {
	return r.find(ctx, studentID, documentID, false)
}

func (r careDocumentRepository) FindForOwnerIncludingDeleted(ctx context.Context, studentID, documentID int64) (*userModels.StudentDocument, error) {
	return r.find(ctx, studentID, documentID, true)
}

func (r careDocumentRepository) find(ctx context.Context, studentID, documentID int64, includeDeleted bool) (*userModels.StudentDocument, error) {
	value, err := r.capability.FindCareDocument(ctx, studentID, documentID, includeDeleted)
	if errors.Is(err, careplan.ErrCareDocumentNotFound) {
		return nil, usersRepo.NotFoundError("find student_document")
	}
	if err != nil {
		return nil, usersRepo.WrapError("find student_document", err)
	}
	return careDocumentToLegacy(value), nil
}

func (r careDocumentRepository) ListByOwnerID(ctx context.Context, studentID int64, categories []string) ([]*userModels.StudentDocument, error) {
	values, err := r.capability.ListCareDocuments(ctx, studentID, categories)
	return careDocumentsToLegacy(values), usersRepo.WrapError("list student_document", err)
}

func (r careDocumentRepository) ListPendingFileCleanupByOwnerID(ctx context.Context, studentID int64) ([]*userModels.StudentDocument, error) {
	values, err := r.capability.ListPendingCareDocumentCleanup(ctx, studentID)
	return careDocumentsToLegacy(values), usersRepo.WrapError("list pending student_document cleanup", err)
}

func (r careDocumentRepository) ListDeletedPendingFileCleanups(ctx context.Context) ([]*userModels.StudentDocument, error) {
	values, err := r.capability.ListDeletedCareDocuments(ctx, 0, nil)
	return careDocumentsToLegacy(values), usersRepo.WrapError("list deleted student_document cleanup", err)
}

func (r careDocumentRepository) ListDeletedPendingFileCleanupByOwnerID(ctx context.Context, studentID int64, categories []string) ([]*userModels.StudentDocument, error) {
	values, err := r.capability.ListDeletedCareDocuments(ctx, studentID, categories)
	return careDocumentsToLegacy(values), usersRepo.WrapError("list pending deleted student_document cleanup", err)
}

func (r careDocumentRepository) SoftDelete(ctx context.Context, document *userModels.StudentDocument, deletedBy int64) error {
	at, err := r.capability.SoftDeleteCareDocument(ctx, document.ID, deletedBy)
	if errors.Is(err, careplan.ErrCareDocumentNotFound) {
		return usersRepo.NotFoundError("soft delete student_document")
	}
	if err != nil {
		return usersRepo.WrapError("soft delete student_document", err)
	}
	document.MarkSoftDeleted(at, deletedBy)
	return nil
}

func (r careDocumentRepository) MarkFileDeleted(ctx context.Context, documentID int64) error {
	return usersRepo.WrapError("mark student_document file deleted", r.capability.MarkCareDocumentFileDeleted(ctx, documentID))
}

func (r careDocumentRepository) QueueFileCleanup(ctx context.Context, cleanup *userModels.StudentDocumentFileCleanup) error {
	if cleanup == nil {
		return errors.New("StudentDocumentFileCleanup cannot be nil")
	}
	value, err := r.capability.QueueCareDocumentCleanup(ctx, careDocumentCleanupToPublic(cleanup))
	if err != nil {
		return usersRepo.WrapError("queue student_document file cleanup", err)
	}
	applyCareDocumentCleanup(cleanup, value)
	return nil
}

func (r careDocumentRepository) ListQueuedFileCleanups(ctx context.Context) ([]*userModels.StudentDocumentFileCleanup, error) {
	return r.listCleanups(ctx, nil)
}

func (r careDocumentRepository) ListQueuedFileCleanupByOwnerID(ctx context.Context, studentID int64) ([]*userModels.StudentDocumentFileCleanup, error) {
	return r.listCleanups(ctx, &studentID)
}

func (r careDocumentRepository) listCleanups(ctx context.Context, studentID *int64) ([]*userModels.StudentDocumentFileCleanup, error) {
	values, err := r.capability.ListCareDocumentCleanups(ctx, studentID)
	if err != nil {
		return nil, usersRepo.WrapError("list queued student_document file cleanup", err)
	}
	result := make([]*userModels.StudentDocumentFileCleanup, 0, len(values))
	for _, value := range values {
		result = append(result, careDocumentCleanupToLegacy(value))
	}
	return result, nil
}

func (r careDocumentRepository) MarkQueuedFileCleanupComplete(ctx context.Context, cleanupID int64) error {
	return usersRepo.WrapError("mark queued student_document file cleanup complete", r.capability.CompleteCareDocumentCleanup(ctx, cleanupID))
}

func (r careDocumentRepository) MarkQueuedFileCleanupCompleteByFilename(ctx context.Context, filename string) error {
	return usersRepo.WrapError("mark queued student_document file cleanup complete", r.capability.CompleteCareDocumentCleanupByFilename(ctx, filename))
}

func (r careDocumentRepository) ActivateQueuedFileCleanupByFilename(ctx context.Context, filename string) error {
	return usersRepo.WrapError("activate queued student_document file cleanup", r.capability.ActivateCareDocumentCleanup(ctx, filename))
}

func careDocumentToPublic(value *userModels.StudentDocument) careplan.CareDocument {
	return careplan.CareDocument{ID: value.ID, TenantID: value.TenantID, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt, StudentID: value.StudentID, Category: value.Category, FilenameDisplay: value.FilenameDisplay, FilenameStored: value.FilenameStored, SizeBytes: value.SizeBytes, ContentType: value.ContentType, UploadedBy: value.UploadedBy, DeletedAt: value.DeletedAt, DeletedBy: value.DeletedBy, FileDeletedAt: value.FileDeletedAt}
}

func careDocumentToLegacy(value careplan.CareDocument) *userModels.StudentDocument {
	result := new(userModels.StudentDocument)
	applyCareDocument(result, value)
	return result
}

func applyCareDocument(result *userModels.StudentDocument, value careplan.CareDocument) {
	result.ID, result.TenantID, result.CreatedAt, result.UpdatedAt = value.ID, value.TenantID, value.CreatedAt, value.UpdatedAt
	result.StudentID, result.Category, result.FilenameDisplay, result.FilenameStored = value.StudentID, value.Category, value.FilenameDisplay, value.FilenameStored
	result.SizeBytes, result.ContentType, result.UploadedBy = value.SizeBytes, value.ContentType, value.UploadedBy
	result.DeletedAt, result.DeletedBy, result.FileDeletedAt = value.DeletedAt, value.DeletedBy, value.FileDeletedAt
}

func careDocumentsToLegacy(values []careplan.CareDocument) []*userModels.StudentDocument {
	if values == nil {
		return nil
	}
	result := make([]*userModels.StudentDocument, 0, len(values))
	for _, value := range values {
		result = append(result, careDocumentToLegacy(value))
	}
	return result
}

func careDocumentCleanupToPublic(value *userModels.StudentDocumentFileCleanup) careplan.CareDocumentCleanup {
	return careplan.CareDocumentCleanup{ID: value.ID, TenantID: value.TenantID, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt, OwnerID: value.OwnerID, FilenameStored: value.FilenameStored, RetryAfter: value.RetryAfter, CleanedAt: value.CleanedAt}
}

func careDocumentCleanupToLegacy(value careplan.CareDocumentCleanup) *userModels.StudentDocumentFileCleanup {
	result := &userModels.StudentDocumentFileCleanup{}
	applyCareDocumentCleanup(result, value)
	return result
}

func applyCareDocumentCleanup(result *userModels.StudentDocumentFileCleanup, value careplan.CareDocumentCleanup) {
	result.ID, result.TenantID, result.CreatedAt, result.UpdatedAt = value.ID, value.TenantID, value.CreatedAt, value.UpdatedAt
	result.OwnerID, result.FilenameStored, result.RetryAfter, result.CleanedAt = value.OwnerID, value.FilenameStored, value.RetryAfter, value.CleanedAt
}
