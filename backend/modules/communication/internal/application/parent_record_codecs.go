package application

import (
	"bytes"
	"encoding/json"
	"maps"

	"github.com/moto-nrw/project-phoenix/modules/communication/internal/domain"

	"github.com/moto-nrw/project-phoenix/modules/communication"
)

func ParentPayloadFromFacts(v map[string]any) (map[string]json.RawMessage, error) {
	return domain.ParentPayloadFromFacts(v)
}
func ParentPayloadToFacts(v map[string]json.RawMessage) (map[string]any, error) {
	return domain.ParentPayloadToFacts(v)
}

func ParentMessageToRecord(v *ParentMessage) (*communication.PortalParentMessage, error) {
	if v == nil {
		return nil, nil
	}
	payload, err := ParentPayloadFromFacts(v.Payload)
	if err != nil {
		return nil, err
	}
	return &communication.PortalParentMessage{
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
func ParentMessageFromRecord(v *communication.PortalParentMessage) (*ParentMessage, error) {
	if v == nil {
		return nil, nil
	}
	payload, err := ParentPayloadToFacts(v.Payload)
	if err != nil {
		return nil, err
	}
	return &ParentMessage{
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

func InboxThreadFromRecord(v *communication.PortalInboxThread) (*InboxThread, error) {
	if v == nil {
		return nil, nil
	}
	payload, err := ParentPayloadToFacts(v.LastMessagePayload)
	if err != nil {
		return nil, err
	}
	return &InboxThread{
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
		ReadBound:              ReadCursorBoundFromRecord(v.ReadBound),
	}, nil
}

// SnapshotParentMessagePayload snapshots values only for the duration of a write.
// Unchanged JSON must not replace the caller's original typed payload map.
func SnapshotParentMessagePayload(record *communication.PortalParentMessage) map[string]json.RawMessage {
	if record == nil {
		return nil
	}
	copy := maps.Clone(record.Payload)
	for key, value := range copy {
		copy[key] = bytes.Clone(value)
	}
	return copy
}
func SameParentRecordPayload(a, b map[string]json.RawMessage) bool {
	if (a == nil) != (b == nil) {
		return false
	}
	return maps.EqualFunc(a, b, func(x, y json.RawMessage) bool { return (x == nil) == (y == nil) && bytes.Equal(x, y) })
}
