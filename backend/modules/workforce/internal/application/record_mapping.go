package application

import (
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/domain"
)

func qualificationsToDomain(values []workforce.StaffQualification) []domain.StaffQualification {
	if values == nil {
		return nil
	}
	result := make([]domain.StaffQualification, 0, len(values))
	for _, v := range values {
		result = append(result, domain.StaffQualification{ID: v.ID, TenantID: v.TenantID, StaffID: v.StaffID, TypeID: v.TypeID, Name: v.Name, AcquiredOn: v.AcquiredOn, ExpiresOn: v.ExpiresOn, CreatedAt: v.CreatedAt, UpdatedAt: v.UpdatedAt})
	}
	return result
}
func documentsToDomain(values []workforce.StaffDocument) []domain.StaffDocument {
	if values == nil {
		return nil
	}
	result := make([]domain.StaffDocument, 0, len(values))
	for _, v := range values {
		result = append(result, domain.StaffDocument(v))
	}
	return result
}
func publicRecordDocuments(values []workforce.StaffDocument, err error) ([]workforce.StaffDocument, error) {
	if err != nil {
		return nil, MapStaffAdminError(err)
	}
	if values == nil {
		return []workforce.StaffDocument{}, nil
	}
	return values, nil
}
func publicRecordCleanups(values []workforce.StaffDocumentFileCleanup, err error) ([]workforce.StaffDocumentFileCleanup, error) {
	if err != nil {
		return nil, MapStaffAdminError(err)
	}
	if values == nil {
		return []workforce.StaffDocumentFileCleanup{}, nil
	}
	return values, nil
}
