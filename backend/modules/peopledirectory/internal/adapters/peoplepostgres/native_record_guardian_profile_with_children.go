package users

import (
	directory "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	models "github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/rows"
)

func NativeGuardianProfileWithChildren(value *models.GuardianProfileWithChildren) *directory.GuardianProfileWithChildren {
	if value == nil {
		return nil
	}
	result := &directory.GuardianProfileWithChildren{}
	result.Profile = NativeGuardianProfile(value.Profile)
	result.PrimaryPhone = value.PrimaryPhone
	result.Children = mapRecordSlice(value.Children, func(x models.GuardianChildSummary) directory.GuardianChildSummary {
		return *NativeGuardianChildSummary(&x)
	})
	return result
}
