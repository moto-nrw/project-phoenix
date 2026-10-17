package parentstore

import (
	models "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/communication"
	messageValues "github.com/moto-nrw/project-phoenix/modules/communication/internal/application"
)

func portalFromRetainedInboxThread(v models.InboxThread) (communication.PortalInboxThread, error) {
	payload, err := messageValues.ParentPayloadFromFacts(v.LastMessagePayload)
	if err != nil {
		return communication.PortalInboxThread{}, err
	}
	return communication.PortalInboxThread{
		ThreadID:               v.ThreadID,
		TenantID:               v.TenantID,
		StudentID:              v.StudentID,
		StudentName:            v.StudentName,
		SchoolClass:            v.SchoolClass,
		SchoolName:             v.SchoolName,
		GroupID:                v.GroupID,
		GroupName:              v.GroupName,
		GuardianAccountID:      v.GuardianAccountID,
		GuardianName:           v.GuardianName,
		RelationshipType:       v.RelationshipType,
		LastMessageAt:          v.LastMessageAt,
		LastSenderKind:         v.LastSenderKind,
		LastMessageBody:        v.LastMessageBody,
		LastMessageKind:        v.LastMessageKind,
		LastEventType:          v.LastEventType,
		LastRequestType:        v.LastRequestType,
		LastRequestStatus:      v.LastRequestStatus,
		LastMessagePayload:     payload,
		LastMessageReadByStaff: v.LastMessageReadByStaff,
		UnreadCount:            v.UnreadCount,
		ReadBound:              portalRecordPointer(v.ReadBound, portalFromRetainedReadCursorBound),
	}, nil
}

func portalFromRetainedParentMessage(v models.ParentMessage) (communication.PortalParentMessage, error) {
	payload, err := messageValues.ParentPayloadFromFacts(v.Payload)
	if err != nil {
		return communication.PortalParentMessage{}, err
	}
	return communication.PortalParentMessage{
		ID:               v.ID,
		CreatedAt:        v.CreatedAt,
		UpdatedAt:        v.UpdatedAt,
		TenantID:         v.TenantID,
		ThreadID:         v.ThreadID,
		StudentID:        v.StudentID,
		SenderAccountID:  v.SenderAccountID,
		SenderKind:       v.SenderKind,
		SenderName:       v.SenderName,
		StaffNameVisible: v.StaffNameVisible,
		Body:             v.Body,
		Kind:             v.Kind,
		EventType:        v.EventType,
		EventActorKind:   v.EventActorKind,
		RequestType:      v.RequestType,
		RequestStatus:    v.RequestStatus,
		Payload:          payload,
		RefTable:         v.RefTable,
		RefID:            v.RefID,
		AppliedAt:        v.AppliedAt,
		AppliedBy:        v.AppliedBy,
		DecisionReason:   v.DecisionReason,
		ReadByStaff:      v.ReadByStaff,
		ReadByGuardian:   v.ReadByGuardian,
	}, nil
}
func portalToRetainedParentMessage(v communication.PortalParentMessage) (models.ParentMessage, error) {
	payload, err := messageValues.ParentPayloadToFacts(v.Payload)
	if err != nil {
		return models.ParentMessage{}, err
	}
	return portalRetainedMessage(v, payload), nil
}
func portalRetainedMessage(v communication.PortalParentMessage, payload map[string]any) models.ParentMessage {
	return models.ParentMessage{
		Model:            models.Model{ID: v.ID, CreatedAt: v.CreatedAt, UpdatedAt: v.UpdatedAt},
		TenantModel:      models.TenantModel{TenantID: v.TenantID},
		ThreadID:         v.ThreadID,
		StudentID:        v.StudentID,
		SenderAccountID:  v.SenderAccountID,
		SenderKind:       v.SenderKind,
		SenderName:       v.SenderName,
		StaffNameVisible: v.StaffNameVisible,
		Body:             v.Body,
		Kind:             v.Kind,
		EventType:        v.EventType,
		EventActorKind:   v.EventActorKind,
		RequestType:      v.RequestType,
		RequestStatus:    v.RequestStatus,
		Payload:          payload,
		RefTable:         v.RefTable,
		RefID:            v.RefID,
		AppliedAt:        v.AppliedAt,
		AppliedBy:        v.AppliedBy,
		DecisionReason:   v.DecisionReason,
		ReadByStaff:      v.ReadByStaff,
		ReadByGuardian:   v.ReadByGuardian,
	}
}
func portalFromRetainedParentMessageThread(v models.ParentMessageThread) communication.PortalParentMessageThread {
	return communication.PortalParentMessageThread{
		ID:                             v.ID,
		CreatedAt:                      v.CreatedAt,
		UpdatedAt:                      v.UpdatedAt,
		TenantID:                       v.TenantID,
		StudentID:                      v.StudentID,
		GuardianAccountID:              v.GuardianAccountID,
		LastMessageAt:                  v.LastMessageAt,
		LastMessageID:                  v.LastMessageID,
		LastSenderKind:                 v.LastSenderKind,
		LastMessageBody:                v.LastMessageBody,
		StaffHandledUpToAt:             v.StaffHandledUpToAt,
		StaffHandledUpToMessageID:      v.StaffHandledUpToMessageID,
		StaffMarkedUnreadAt:            v.StaffMarkedUnreadAt,
		StaffMarkedUnreadByAccountID:   v.StaffMarkedUnreadByAccountID,
		LastStaffMessageNotificationAt: v.LastStaffMessageNotificationAt,
	}
}

func portalFromRetainedReadCursorBound(v models.ReadCursorBound) communication.PortalReadCursorBound {
	return communication.PortalReadCursorBound{
		ThreadID:  v.ThreadID,
		ReadAt:    v.ReadAt,
		MessageID: v.MessageID,
	}
}
