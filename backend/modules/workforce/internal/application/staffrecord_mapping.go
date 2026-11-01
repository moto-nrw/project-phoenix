package application

import (
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/domain"
)

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
			ID: value.ID, TenantID: value.TenantID, StaffID: value.StaffID, TypeID: value.TypeID, Name: value.Name,
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
	case errors.Is(err, domain.ErrStaffMasterDataNotFound), errors.Is(err, workforce.ErrStaffMasterDataNotFound):
		return workforce.ErrStaffMasterDataNotFound
	case errors.Is(err, domain.ErrStaffFinancialDataNotFound), errors.Is(err, workforce.ErrStaffFinancialDataNotFound):
		return workforce.ErrStaffFinancialDataNotFound
	case errors.Is(err, domain.ErrStaffDocumentNotFound), errors.Is(err, workforce.ErrStaffDocumentNotFound):
		return workforce.ErrStaffDocumentNotFound
	case errors.Is(err, domain.ErrInvalidStaffRecord), errors.Is(err, workforce.ErrInvalidStaffRecord):
		return &workforce.InvalidStaffRecordError{Reason: err.Error()}
	default:
		return MapError(err)
	}
}
