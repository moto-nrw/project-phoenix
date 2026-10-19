package parentportal

import "github.com/moto-nrw/project-phoenix/workflows/parentportal/internal/messaging"

func messagingToAppFamilyProtectionEvent(v FamilyProtectionEvent) messaging.FamilyProtectionEvent {
	return messaging.FamilyProtectionEvent{
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

func messagingToAppParentRequestShareEvent(v ParentRequestShareEvent) messaging.ParentRequestShareEvent {
	return messaging.ParentRequestShareEvent{
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
func messagingFromAppParentRequestShareEvent(v messaging.ParentRequestShareEvent) ParentRequestShareEvent {
	return ParentRequestShareEvent{
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
