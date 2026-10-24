package owners

import (
	directory "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	parentportal "github.com/moto-nrw/project-phoenix/workflows/parentportal"
)

func parentMessagingFromOwnerFamilyProtectionEvent(v directory.GuardianFamilyProtectionRecord) parentportal.FamilyProtectionEvent {
	return parentportal.FamilyProtectionEvent{
		ID:             v.ID,
		CreatedAt:      v.CreatedAt,
		UpdatedAt:      v.UpdatedAt,
		TenantID:       v.TenantID,
		StudentID:      v.StudentID,
		Enabled:        v.Enabled,
		Reason:         v.Reason,
		ActorAccountID: v.ActorAccountID,
	}
}

func parentMessagingToOwnerParentRequestShareEvent(v parentportal.ParentRequestShareEvent) directory.ParentRequestShareRecord {
	return directory.ParentRequestShareRecord{
		ID:                  v.ID,
		CreatedAt:           v.CreatedAt,
		UpdatedAt:           v.UpdatedAt,
		TenantID:            v.TenantID,
		StudentID:           v.StudentID,
		RequestType:         v.RequestType,
		RequestID:           v.RequestID,
		AuthorAccountID:     v.AuthorAccountID,
		RecipientAccountIDs: v.RecipientAccountIDs,
	}
}

func parentMessagingFromOwnerParentRequestShareEvent(v directory.ParentRequestShareRecord) parentportal.ParentRequestShareEvent {
	return parentportal.ParentRequestShareEvent{
		ID:                  v.ID,
		CreatedAt:           v.CreatedAt,
		UpdatedAt:           v.UpdatedAt,
		TenantID:            v.TenantID,
		StudentID:           v.StudentID,
		RequestType:         v.RequestType,
		RequestID:           v.RequestID,
		AuthorAccountID:     v.AuthorAccountID,
		RecipientAccountIDs: v.RecipientAccountIDs,
	}
}
