package application

import (
	"time"

	"github.com/moto-nrw/project-phoenix/modules/communication"
)

// Native application values are distinct from retained ORM models.
type InboxThread struct {
	ThreadID               int64            `json:"thread_id"`
	TenantID               int64            `json:"-"`
	StudentID              int64            `json:"student_id"`
	StudentName            string           `json:"student_name"`
	SchoolClass            string           `json:"school_class"`
	SchoolName             string           `json:"school_name"`
	GroupID                *int64           `json:"group_id,omitempty"`
	GroupName              string           `json:"group_name,omitempty"`
	GuardianAccountID      int64            `json:"guardian_account_id"`
	GuardianName           string           `json:"guardian_name"`
	RelationshipType       string           `json:"relationship_type"`
	LastMessageAt          *time.Time       `json:"last_message_at,omitempty"`
	LastSenderKind         string           `json:"last_sender_kind,omitempty"`
	LastMessageBody        string           `json:"last_message_body,omitempty"`
	LastMessageKind        string           `json:"-"`
	LastEventType          string           `json:"-"`
	LastRequestType        string           `json:"-"`
	LastRequestStatus      string           `json:"-"`
	LastMessagePayload     map[string]any   `json:"-"`
	LastMessageReadByStaff bool             `json:"-"`
	UnreadCount            int              `json:"unread_count"`
	ReadBound              *ReadCursorBound `json:"-"`
}
type MessageableGuardian communication.ParentGuardianRecipient
type ParentMessage struct {
	ID               int64          `json:"id"`
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
	TenantID         int64          `json:"tenant_id"`
	ThreadID         int64          `json:"thread_id"`
	StudentID        int64          `json:"student_id"`
	SenderAccountID  int64          `json:"sender_account_id"`
	SenderKind       string         `json:"sender_kind"`
	SenderName       string         `json:"sender_name"`
	StaffNameVisible bool           `json:"staff_name_visible"`
	Body             string         `json:"body"`
	Kind             string         `json:"kind"`
	EventType        string         `json:"event_type,omitempty"`
	EventActorKind   string         `json:"event_actor_kind,omitempty"`
	RequestType      string         `json:"request_type,omitempty"`
	RequestStatus    string         `json:"request_status,omitempty"`
	Payload          map[string]any `json:"payload,omitempty"`
	RefTable         string         `json:"ref_table,omitempty"`
	RefID            *int64         `json:"ref_id,omitempty"`
	AppliedAt        *time.Time     `json:"applied_at,omitempty"`
	AppliedBy        *int64         `json:"applied_by,omitempty"`
	DecisionReason   string         `json:"decision_reason,omitempty"`
	GroupThreadID    *int64         `json:"group_thread_id,omitempty"`
	GroupThreadTitle string         `json:"group_thread_title,omitempty"`
	ReadByStaff      bool           `json:"read_by_staff,omitempty"`
	ReadByGuardian   bool           `json:"read_by_guardian,omitempty"`
}
type ParentMessageThread communication.PortalParentMessageThread
type ReadCursor communication.ParentReadCursorRecord
type ReadCursorBound communication.PortalReadCursorBound
type ThreadHeader communication.ParentThreadHeaderRecord
type ParentMessageReadRepositorySource interface {
	communication.ParentReadRecords
}
type ParentMessageRepositorySource interface {
	communication.ParentMessageRecords
}
type ParentMessageThreadRepositorySource interface {
	communication.ParentThreadRecords
}

func MessageableGuardianFromRecord(v *communication.ParentGuardianRecipient) *MessageableGuardian {
	return (*MessageableGuardian)(v)
}

func ParentMessageThreadFromRecord(v *communication.PortalParentMessageThread) *ParentMessageThread {
	return (*ParentMessageThread)(v)
}

func ReadCursorFromRecord(v *communication.ParentReadCursorRecord) *ReadCursor {
	return (*ReadCursor)(v)
}

func ReadCursorBoundFromRecord(v *communication.PortalReadCursorBound) *ReadCursorBound {
	return (*ReadCursorBound)(v)
}

func ThreadHeaderFromRecord(v *communication.ParentThreadHeaderRecord) *ThreadHeader {
	return (*ThreadHeader)(v)
}

func ParentReadBoundsToRecords(rows []ReadCursorBound) []communication.PortalReadCursorBound {
	if rows == nil {
		return nil
	}
	out := make([]communication.PortalReadCursorBound, len(rows))
	for i, row := range rows {
		out[i] = communication.PortalReadCursorBound(row)
	}
	return out
}
