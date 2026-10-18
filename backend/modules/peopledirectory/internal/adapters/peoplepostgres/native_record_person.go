package users

import (
	models "github.com/moto-nrw/project-phoenix/models/users"
	directory "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
)

func DirectoryPersonRow(value *directory.DirectoryPersonSnapshot) *models.Person {
	if value == nil {
		return nil
	}
	result := &models.Person{}
	result.ID = value.ID
	result.CreatedAt = value.CreatedAt
	result.UpdatedAt = value.UpdatedAt
	result.TenantID = value.TenantID
	result.FirstName = value.FirstName
	result.LastName = value.LastName
	result.Birthday = value.Birthday
	result.TagID = value.TagID
	result.AccountID = value.AccountID
	result.DeletedAt = value.DeletedAt
	if value.Account != nil {
		x := models.PersonAccount(*value.Account)
		result.Account = &x
	}
	if value.RFIDCard != nil {
		x := models.PersonRFIDCard(*value.RFIDCard)
		result.RFIDCard = &x
	}
	return result
}
