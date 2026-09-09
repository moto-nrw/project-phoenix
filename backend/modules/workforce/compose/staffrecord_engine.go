package compose

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/domain"
)

// --- master data ---

func (e engine) FindStaffMasterData(ctx context.Context, staffID int64) (workforce.StaffMasterData, error) {
	value, err := e.service.FindStaffMasterData(ctx, staffID)
	return masterDataToPublic(value), mapStaffRecordError(err)
}

func (e engine) CreateStaffMasterData(ctx context.Context, value workforce.StaffMasterData) (workforce.StaffMasterData, error) {
	created, err := e.service.CreateStaffMasterData(ctx, masterDataToDomain(value))
	return masterDataToPublic(created), mapStaffRecordError(err)
}

func (e engine) UpdateStaffMasterData(ctx context.Context, value workforce.StaffMasterData) (workforce.StaffMasterData, error) {
	updated, err := e.service.UpdateStaffMasterData(ctx, masterDataToDomain(value))
	return masterDataToPublic(updated), mapStaffRecordError(err)
}

// --- qualifications ---

func (e engine) ListStaffQualifications(ctx context.Context, staffID int64) ([]workforce.StaffQualification, error) {
	values, err := e.service.ListStaffQualifications(ctx, staffID)
	return qualificationsToPublic(values), mapStaffRecordError(err)
}

func (e engine) ReplaceStaffQualifications(ctx context.Context, staffID int64, values []workforce.StaffQualification) ([]workforce.StaffQualification, error) {
	rows := make([]domain.StaffQualification, 0, len(values))
	for _, value := range values {
		rows = append(rows, domain.StaffQualification{
			ID: value.ID, TenantID: value.TenantID, StaffID: value.StaffID, Name: value.Name,
			AcquiredOn: value.AcquiredOn, ExpiresOn: value.ExpiresOn, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
		})
	}
	stored, err := e.service.ReplaceStaffQualifications(ctx, staffID, rows)
	return qualificationsToPublic(stored), mapStaffRecordError(err)
}

// --- financial data ---

func (e engine) FindStaffFinancialData(ctx context.Context, staffID int64) (workforce.StaffFinancialData, error) {
	value, err := e.service.FindStaffFinancialData(ctx, staffID)
	return financialToPublic(value), mapStaffRecordError(err)
}

func (e engine) CreateStaffFinancialData(ctx context.Context, value workforce.StaffFinancialData) (workforce.StaffFinancialData, error) {
	created, err := e.service.CreateStaffFinancialData(ctx, financialToDomain(value))
	return financialToPublic(created), mapStaffRecordError(err)
}

func (e engine) UpdateStaffFinancialData(ctx context.Context, value workforce.StaffFinancialData) (workforce.StaffFinancialData, error) {
	updated, err := e.service.UpdateStaffFinancialData(ctx, financialToDomain(value))
	return financialToPublic(updated), mapStaffRecordError(err)
}

// --- documents ---

func (e engine) CreateStaffDocument(ctx context.Context, value workforce.StaffDocument) (workforce.StaffDocument, error) {
	created, err := e.service.CreateStaffDocument(ctx, documentToDomain(value))
	return documentToPublic(created), mapStaffRecordError(err)
}

func (e engine) FindStaffDocument(ctx context.Context, staffID, documentID int64, includeDeleted bool) (workforce.StaffDocument, error) {
	value, err := e.service.FindStaffDocument(ctx, staffID, documentID, includeDeleted)
	return documentToPublic(value), mapStaffRecordError(err)
}

func (e engine) ListStaffDocuments(ctx context.Context, filter workforce.StaffDocumentFilter) ([]workforce.StaffDocument, error) {
	values, err := e.service.ListStaffDocuments(ctx, domain.StaffDocumentFilter{
		StaffID: filter.StaffID, StaffIDs: filter.StaffIDs, Categories: filter.Categories,
		IncludeDeleted: filter.IncludeDeleted, DeletedOnly: filter.DeletedOnly, FilePending: filter.FilePending,
	})
	if values == nil {
		return nil, mapStaffRecordError(err)
	}
	result := make([]workforce.StaffDocument, 0, len(values))
	for _, value := range values {
		result = append(result, documentToPublic(value))
	}
	return result, mapStaffRecordError(err)
}

func (e engine) SoftDeleteStaffDocument(ctx context.Context, id, deletedBy int64, at time.Time) (int64, error) {
	value, err := e.service.SoftDeleteStaffDocument(ctx, id, deletedBy, at)
	return value, mapStaffRecordError(err)
}

func (e engine) MarkStaffDocumentFileDeleted(ctx context.Context, id int64, at time.Time) error {
	return mapStaffRecordError(e.service.MarkStaffDocumentFileDeleted(ctx, id, at))
}

// --- file cleanup intents ---

func (e engine) QueueStaffDocumentFileCleanup(ctx context.Context, value workforce.StaffDocumentFileCleanup) error {
	return mapStaffRecordError(e.service.QueueStaffDocumentFileCleanup(ctx, domain.StaffDocumentFileCleanup{
		ID: value.ID, TenantID: value.TenantID, StaffID: value.StaffID, FilenameStored: value.FilenameStored,
		RetryAfter: value.RetryAfter, CleanedAt: value.CleanedAt, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
	}))
}

func (e engine) ListQueuedStaffDocumentFileCleanups(ctx context.Context, staffID int64) ([]workforce.StaffDocumentFileCleanup, error) {
	values, err := e.service.ListQueuedStaffDocumentFileCleanups(ctx, staffID)
	if values == nil {
		return nil, mapStaffRecordError(err)
	}
	result := make([]workforce.StaffDocumentFileCleanup, 0, len(values))
	for _, value := range values {
		result = append(result, workforce.StaffDocumentFileCleanup{
			ID: value.ID, TenantID: value.TenantID, StaffID: value.StaffID, FilenameStored: value.FilenameStored,
			RetryAfter: value.RetryAfter, CleanedAt: value.CleanedAt, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
		})
	}
	return result, mapStaffRecordError(err)
}

func (e engine) CompleteStaffDocumentFileCleanup(ctx context.Context, id int64) error {
	return mapStaffRecordError(e.service.CompleteStaffDocumentFileCleanup(ctx, id))
}

func (e engine) CompleteStaffDocumentFileCleanupByFilename(ctx context.Context, filename string) error {
	return mapStaffRecordError(e.service.CompleteStaffDocumentFileCleanupByFilename(ctx, filename))
}

func (e engine) ActivateStaffDocumentFileCleanup(ctx context.Context, filename string) error {
	return mapStaffRecordError(e.service.ActivateStaffDocumentFileCleanup(ctx, filename))
}

// --- mapping ---

func masterDataToDomain(value workforce.StaffMasterData) domain.StaffMasterData {
	return domain.StaffMasterData{
		ID: value.ID, TenantID: value.TenantID, StaffID: value.StaffID, Gender: value.Gender,
		AddressStreet: value.AddressStreet, AddressPostalCode: value.AddressPostalCode, AddressCity: value.AddressCity,
		Phone: value.Phone, Email: value.Email, EmergencyContactName: value.EmergencyContactName,
		EmergencyContactPhone: value.EmergencyContactPhone, EntryDate: value.EntryDate, ContractEndDate: value.ContractEndDate,
		ProbationEndDate: value.ProbationEndDate, WeeklyHours: value.WeeklyHours, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
	}
}

func masterDataToPublic(value domain.StaffMasterData) workforce.StaffMasterData {
	return workforce.StaffMasterData{
		ID: value.ID, TenantID: value.TenantID, StaffID: value.StaffID, Gender: value.Gender,
		AddressStreet: value.AddressStreet, AddressPostalCode: value.AddressPostalCode, AddressCity: value.AddressCity,
		Phone: value.Phone, Email: value.Email, EmergencyContactName: value.EmergencyContactName,
		EmergencyContactPhone: value.EmergencyContactPhone, EntryDate: value.EntryDate, ContractEndDate: value.ContractEndDate,
		ProbationEndDate: value.ProbationEndDate, WeeklyHours: value.WeeklyHours, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
	}
}

func qualificationsToPublic(values []domain.StaffQualification) []workforce.StaffQualification {
	if values == nil {
		return nil
	}
	result := make([]workforce.StaffQualification, 0, len(values))
	for _, value := range values {
		result = append(result, workforce.StaffQualification{
			ID: value.ID, TenantID: value.TenantID, StaffID: value.StaffID, Name: value.Name,
			AcquiredOn: value.AcquiredOn, ExpiresOn: value.ExpiresOn, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
		})
	}
	return result
}

func financialToDomain(value workforce.StaffFinancialData) domain.StaffFinancialData {
	return domain.StaffFinancialData{
		ID: value.ID, TenantID: value.TenantID, StaffID: value.StaffID, IBAN: value.IBAN, TaxID: value.TaxID,
		SocialSecurityNumber: value.SocialSecurityNumber, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
	}
}

func financialToPublic(value domain.StaffFinancialData) workforce.StaffFinancialData {
	return workforce.StaffFinancialData{
		ID: value.ID, TenantID: value.TenantID, StaffID: value.StaffID, IBAN: value.IBAN, TaxID: value.TaxID,
		SocialSecurityNumber: value.SocialSecurityNumber, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
	}
}

func documentToDomain(value workforce.StaffDocument) domain.StaffDocument {
	return domain.StaffDocument{
		ID: value.ID, TenantID: value.TenantID, StaffID: value.StaffID, Category: value.Category,
		FilenameDisplay: value.FilenameDisplay, FilenameStored: value.FilenameStored, SizeBytes: value.SizeBytes,
		ContentType: value.ContentType, UploadedBy: value.UploadedBy, DeletedAt: value.DeletedAt, DeletedBy: value.DeletedBy,
		FileDeletedAt: value.FileDeletedAt, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
	}
}

func documentToPublic(value domain.StaffDocument) workforce.StaffDocument {
	return workforce.StaffDocument{
		ID: value.ID, TenantID: value.TenantID, StaffID: value.StaffID, Category: value.Category,
		FilenameDisplay: value.FilenameDisplay, FilenameStored: value.FilenameStored, SizeBytes: value.SizeBytes,
		ContentType: value.ContentType, UploadedBy: value.UploadedBy, DeletedAt: value.DeletedAt, DeletedBy: value.DeletedBy,
		FileDeletedAt: value.FileDeletedAt, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
	}
}

func mapStaffRecordError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, domain.ErrStaffMasterDataNotFound):
		return workforce.ErrStaffMasterDataNotFound
	case errors.Is(err, domain.ErrStaffFinancialDataNotFound):
		return workforce.ErrStaffFinancialDataNotFound
	case errors.Is(err, domain.ErrStaffDocumentNotFound):
		return workforce.ErrStaffDocumentNotFound
	case errors.Is(err, domain.ErrInvalidStaffRecord):
		return &workforce.InvalidStaffRecordError{Reason: err.Error()}
	default:
		return mapError(err)
	}
}
