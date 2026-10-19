package parentportal

import "github.com/moto-nrw/project-phoenix/workflows/parentportal/internal/messaging"

func messagingToAppInboxThread(v InboxThread) messaging.InboxThread {
	return messaging.InboxThread{
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
		LastMessagePayload:     v.LastMessagePayload,
		LastMessageReadByStaff: v.LastMessageReadByStaff,
		UnreadCount:            v.UnreadCount,
		ReadBound:              messagingValuePointer(v.ReadBound, messagingToAppReadCursorBound),
	}
}
func messagingFromAppInboxThread(v messaging.InboxThread) InboxThread {
	return InboxThread{
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
		LastMessagePayload:     v.LastMessagePayload,
		LastMessageReadByStaff: v.LastMessageReadByStaff,
		UnreadCount:            v.UnreadCount,
		ReadBound:              messagingValuePointer(v.ReadBound, messagingFromAppReadCursorBound),
	}
}
func messagingToAppParentMessage(v ParentMessage) messaging.ParentMessage {
	return messaging.ParentMessage{
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
		ReadByStaff:      v.ReadByStaff,
		ReadByGuardian:   v.ReadByGuardian,
	}
}
func messagingFromAppParentMessage(v messaging.ParentMessage) ParentMessage {
	return ParentMessage{
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
		ReadByStaff:      v.ReadByStaff,
		ReadByGuardian:   v.ReadByGuardian,
	}
}

func messagingToAppParentMessageThread(v ParentMessageThread) messaging.ParentMessageThread {
	return messaging.ParentMessageThread{
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

func messagingToAppReadCursorBound(v ReadCursorBound) messaging.ReadCursorBound {
	return messaging.ReadCursorBound{
		ThreadID:  v.ThreadID,
		ReadAt:    v.ReadAt,
		MessageID: v.MessageID,
	}
}
func messagingFromAppReadCursorBound(v messaging.ReadCursorBound) ReadCursorBound {
	return ReadCursorBound{
		ThreadID:  v.ThreadID,
		ReadAt:    v.ReadAt,
		MessageID: v.MessageID,
	}
}
