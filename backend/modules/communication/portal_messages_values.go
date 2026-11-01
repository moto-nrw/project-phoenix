package communication

import (
	"encoding/json"
	"time"
)

// Complete native values retain source metadata, nullable fields and JSON presence.
type PortalInboxThread struct {
	ThreadID               int64                      `json:"thread_id"`
	TenantID               int64                      `json:"-"`
	StudentID              int64                      `json:"student_id"`
	StudentName            string                     `json:"student_name"`
	SchoolClass            string                     `json:"school_class"`
	SchoolName             string                     `json:"school_name"`
	GroupID                *int64                     `json:"group_id,omitempty"`
	GroupName              string                     `json:"group_name,omitempty"`
	GuardianAccountID      int64                      `json:"guardian_account_id"`
	GuardianName           string                     `json:"guardian_name"`
	RelationshipType       string                     `json:"relationship_type"`
	Audience               string                     `json:"audience"`
	LastMessageAt          *time.Time                 `json:"last_message_at,omitempty"`
	LastSenderKind         string                     `json:"last_sender_kind,omitempty"`
	LastMessageBody        string                     `json:"last_message_body,omitempty"`
	LastMessageKind        string                     `json:"-"`
	LastEventType          string                     `json:"-"`
	LastRequestType        string                     `json:"-"`
	LastRequestStatus      string                     `json:"-"`
	LastMessagePayload     map[string]json.RawMessage `json:"-"`
	LastMessageReadByStaff bool                       `json:"-"`
	UnreadCount            int                        `json:"unread_count"`
	ReadBound              *PortalReadCursorBound     `json:"-"`
}
type PortalParentMessage struct {
	ClientMessageID  string                     `json:"client_message_id,omitempty"`
	SendReplayed     bool                       `json:"-"`
	ID               int64                      `json:"id"`
	CreatedAt        time.Time                  `json:"created_at"`
	UpdatedAt        time.Time                  `json:"updated_at"`
	TenantID         int64                      `json:"tenant_id"`
	ThreadID         int64                      `json:"thread_id"`
	StudentID        int64                      `json:"student_id"`
	SenderAccountID  int64                      `json:"sender_account_id"`
	SenderKind       string                     `json:"sender_kind"`
	SenderName       string                     `json:"sender_name"`
	StaffNameVisible bool                       `json:"staff_name_visible"`
	Body             string                     `json:"body"`
	Kind             string                     `json:"kind"`
	EventType        string                     `json:"event_type,omitempty"`
	EventActorKind   string                     `json:"event_actor_kind,omitempty"`
	RequestType      string                     `json:"request_type,omitempty"`
	RequestStatus    string                     `json:"request_status,omitempty"`
	Payload          map[string]json.RawMessage `json:"payload,omitempty"`
	RefTable         string                     `json:"ref_table,omitempty"`
	RefID            *int64                     `json:"ref_id,omitempty"`
	AppliedAt        *time.Time                 `json:"applied_at,omitempty"`
	AppliedBy        *int64                     `json:"applied_by,omitempty"`
	DecisionReason   string                     `json:"decision_reason,omitempty"`
	GroupThreadID    *int64                     `json:"group_thread_id,omitempty"`
	GroupThreadTitle string                     `json:"group_thread_title,omitempty"`
	ReadByStaff      bool                       `json:"read_by_staff,omitempty"`
	ReadByGuardian   bool                       `json:"read_by_guardian,omitempty"`
}
type PortalParentMessageThread struct {
	ID                             int64      `json:"id"`
	CreatedAt                      time.Time  `json:"created_at"`
	UpdatedAt                      time.Time  `json:"updated_at"`
	TenantID                       int64      `json:"tenant_id"`
	StudentID                      int64      `json:"student_id"`
	GuardianAccountID              int64      `json:"guardian_account_id"`
	Audience                       string     `json:"audience"`
	LastMessageAt                  *time.Time `json:"last_message_at,omitempty"`
	LastMessageID                  *int64     `json:"last_message_id,omitempty"`
	LastSenderKind                 *string    `json:"last_sender_kind,omitempty"`
	LastMessageBody                string     `json:"last_message_body,omitempty"`
	StaffHandledUpToAt             *time.Time `json:"-"`
	StaffHandledUpToMessageID      *int64     `json:"-"`
	StaffMarkedUnreadAt            *time.Time `json:"-"`
	StaffMarkedUnreadByAccountID   *int64     `json:"-"`
	LastStaffMessageNotificationAt *time.Time `json:"-"`
}
type PortalReadCursorBound struct {
	ThreadID  int64
	ReadAt    time.Time
	MessageID int64
}
