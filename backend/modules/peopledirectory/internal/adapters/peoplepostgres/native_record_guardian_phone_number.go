package users

import (
	directory "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	models "github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/rows"
)

func GuardianPhoneRow(value *directory.GuardianProfilePhoneSnapshot) *models.GuardianPhoneNumber {
	if value == nil {
		return nil
	}
	result := &models.GuardianPhoneNumber{}
	result.ID = value.ID
	result.CreatedAt = value.CreatedAt
	result.UpdatedAt = value.UpdatedAt
	result.TenantID = value.TenantID
	result.GuardianProfileID = value.GuardianProfileID
	result.PhoneNumber = value.PhoneNumber
	result.PhoneType = models.PhoneType(value.PhoneType)
	result.Label = value.Label
	result.IsPrimary = value.IsPrimary
	result.Priority = value.Priority
	return result
}
