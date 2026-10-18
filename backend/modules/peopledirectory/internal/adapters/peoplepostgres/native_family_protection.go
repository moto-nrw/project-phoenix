package users

import (
	"context"

	directory "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	models "github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/rows"
)

type nativeFamilyProtectionRecords struct {
	*FamilyProtectionEventRepository
}

func (r nativeFamilyProtectionRecords) Create(ctx context.Context, value *directory.GuardianFamilyProtectionRecord) error {
	var row *models.FamilyProtectionEvent
	if value != nil {
		row = &models.FamilyProtectionEvent{StudentID: value.StudentID, Enabled: value.Enabled, Reason: value.Reason, ActorAccountID: value.ActorAccountID}
		row.ID, row.TenantID, row.CreatedAt, row.UpdatedAt = value.ID, value.TenantID, value.CreatedAt, value.UpdatedAt
	}
	err := r.FamilyProtectionEventRepository.Create(ctx, row)
	if value != nil && row != nil {
		*value = *nativeFamilyProtectionEvent(row)
	}
	return err
}
