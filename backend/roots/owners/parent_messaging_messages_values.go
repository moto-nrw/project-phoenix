package owners

import (
	"github.com/moto-nrw/project-phoenix/modules/communication"
	parentportal "github.com/moto-nrw/project-phoenix/workflows/parentportal"
)

func parentMessagingFromOwnerInboxThread(v communication.PortalInboxThread) parentportal.InboxThread {
	return parentportal.InboxThread{
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
		Audience:               v.Audience,
		LastMessageAt:          v.LastMessageAt,
		LastSenderKind:         v.LastSenderKind,
		LastMessageBody:        v.LastMessageBody,
		LastMessageKind:        v.LastMessageKind,
		LastEventType:          v.LastEventType,
		LastRequestType:        v.LastRequestType,
		LastRequestStatus:      v.LastRequestStatus,
		LastMessagePayload:     v.LastMessagePayload,
		LastMessageReadByStaff: v.LastMessageReadByStaff,
		UnreadCount:            v.UnreadCount,
		ReadBound:              ParentMessagingPointer(v.ReadBound, parentMessagingFromOwnerReadCursorBound),
	}
}

func parentMessagingToOwnerParentMessage(v parentportal.ParentMessage) communication.PortalParentMessage {
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
		Payload:          v.Payload,
		RefTable:         v.RefTable,
		RefID:            v.RefID,
		AppliedAt:        v.AppliedAt,
		AppliedBy:        v.AppliedBy,
		DecisionReason:   v.DecisionReason,
		GroupThreadID:    v.GroupThreadID,
		GroupThreadTitle: v.GroupThreadTitle,
		ReadByStaff:      v.ReadByStaff,
		ReadByGuardian:   v.ReadByGuardian,
	}
}

func ParentMessagingFromOwnerParentMessage(v communication.PortalParentMessage) parentportal.ParentMessage {
	return parentportal.ParentMessage{
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
		Payload:          v.Payload,
		RefTable:         v.RefTable,
		RefID:            v.RefID,
		AppliedAt:        v.AppliedAt,
		AppliedBy:        v.AppliedBy,
		DecisionReason:   v.DecisionReason,
		GroupThreadID:    v.GroupThreadID,
		GroupThreadTitle: v.GroupThreadTitle,
		ReadByStaff:      v.ReadByStaff,
		ReadByGuardian:   v.ReadByGuardian,
	}
}

func parentMessagingFromOwnerParentMessageThread(v communication.PortalParentMessageThread) parentportal.ParentMessageThread {
	return parentportal.ParentMessageThread{
		ID:                             v.ID,
		CreatedAt:                      v.CreatedAt,
		UpdatedAt:                      v.UpdatedAt,
		TenantID:                       v.TenantID,
		StudentID:                      v.StudentID,
		GuardianAccountID:              v.GuardianAccountID,
		Audience:                       v.Audience,
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

func parentMessagingFromOwnerReadCursorBound(v communication.PortalReadCursorBound) parentportal.ReadCursorBound {
	return parentportal.ReadCursorBound{
		ThreadID:  v.ThreadID,
		ReadAt:    v.ReadAt,
		MessageID: v.MessageID,
	}
}
