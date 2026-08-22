package parent

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/api/common"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	parentService "github.com/moto-nrw/project-phoenix/services/parent"
)

// MessageResponse is one message in a conversation. IDs stringified per the
// int64 -> string frontend convention. sender_kind is "guardian" or "staff".
// For staff messages sender_name is either the neutral OGS/school label or the
// individual staff member as "first name + last initial", depending on whether
// the message froze staff-name visibility on at send time (see toMessageResponses
// and operations.parent_message_staff_name_visible).
type MessageResponse struct {
	ID             string         `json:"id"`
	SenderKind     string         `json:"sender_kind"`
	SenderName     string         `json:"sender_name"`
	Body           string         `json:"body"`
	CreatedAt      time.Time      `json:"created_at"`
	Kind           string         `json:"kind"`
	EventType      string         `json:"event_type,omitempty"`
	RequestType    string         `json:"request_type,omitempty"`
	RequestStatus  string         `json:"request_status,omitempty"`
	Payload        map[string]any `json:"payload,omitempty"`
	RefTable       string         `json:"ref_table,omitempty"`
	RefID          string         `json:"ref_id,omitempty"`
	AppliedAt      *time.Time     `json:"applied_at,omitempty"`
	DecisionReason string         `json:"decision_reason,omitempty"`
	ReadByStaff    bool           `json:"read_by_staff,omitempty"`
}

// ThreadSummaryResponse is one row on the parent thread list. counterpart_name
// is the "OGS [Schulname]" the guardian is talking to.
type ThreadSummaryResponse struct {
	ThreadID        string     `json:"thread_id"`
	StudentID       string     `json:"student_id"`
	StudentName     string     `json:"student_name"`
	SchoolName      string     `json:"school_name"`
	CounterpartName string     `json:"counterpart_name"`
	LastMessageAt   *time.Time `json:"last_message_at,omitempty"`
	LastSenderKind  string     `json:"last_sender_kind,omitempty"`
	LastMessageBody string     `json:"last_message_body,omitempty"`
	// Structured fields of the last message so the localized parents portal
	// renders a request title / decision / withdrawal preview from fields instead
	// of the German LastMessageBody (which the full conversation already
	// localizes the same way). Empty for plain messages, where LastMessageBody is
	// the human-written, language-neutral text.
	LastMessageKind        string         `json:"last_message_kind,omitempty"`
	LastEventType          string         `json:"last_event_type,omitempty"`
	LastRequestType        string         `json:"last_request_type,omitempty"`
	LastRequestStatus      string         `json:"last_request_status,omitempty"`
	LastMessagePayload     map[string]any `json:"last_message_payload,omitempty"`
	LastMessageReadByStaff bool           `json:"last_message_read_by_staff"`
	Unread                 int            `json:"unread"`
}

// toThreadSummary maps a projected inbox thread to the parent-facing summary,
// masking the counterpart as the OGS/school label and carrying the structured
// last-message fields the localized portal previews from. Shared by the inbox
// list and the per-child list so the two cannot drift.
func toThreadSummary(t *usersModels.InboxThread) ThreadSummaryResponse {
	return ThreadSummaryResponse{
		ThreadID:               strconv.FormatInt(t.ThreadID, 10),
		StudentID:              strconv.FormatInt(t.StudentID, 10),
		StudentName:            t.StudentName,
		SchoolName:             t.SchoolName,
		CounterpartName:        ogsLabel(t.SchoolName),
		LastMessageAt:          t.LastMessageAt,
		LastSenderKind:         t.LastSenderKind,
		LastMessageBody:        t.LastMessageBody,
		LastMessageKind:        t.LastMessageKind,
		LastEventType:          t.LastEventType,
		LastRequestType:        t.LastRequestType,
		LastRequestStatus:      t.LastRequestStatus,
		LastMessagePayload:     t.LastMessagePayload,
		LastMessageReadByStaff: t.LastMessageReadByStaff,
		Unread:                 t.UnreadCount,
	}
}

// ThreadViewResponse is the full conversation for one child. thread_id is empty
// when no conversation exists yet (created on the first message).
type ThreadViewResponse struct {
	ThreadID        string            `json:"thread_id"`
	StudentID       string            `json:"student_id"`
	StudentName     string            `json:"student_name"`
	SchoolName      string            `json:"school_name"`
	CounterpartName string            `json:"counterpart_name"`
	Messages        []MessageResponse `json:"messages"`
}

// PostMessageRequest is the wire shape for POST .../children/{studentId}.
type PostMessageRequest struct {
	Body string `json:"body"`
}

// ogsLabel is the parent-facing name of the OGS counterpart for a school.
func ogsLabel(schoolName string) string {
	return strings.TrimSpace("OGS " + schoolName)
}

// staffShortName renders a staff member's stored full name as first name + last
// initial (e.g. "Anna Müller" -> "Anna M.") for the guardian view: enough to
// attribute a reply to a person without exposing the full surname. A single-token
// name (or the "OGS-Team" fallback resolveStaffName stamps when no person is
// linked) is returned unchanged. Rune-based so a non-ASCII surname initial (Ö, Ü)
// is taken as one character, not a broken byte.
func staffShortName(full string) string {
	parts := strings.Fields(full)
	if len(parts) < 2 {
		return full
	}
	last := []rune(parts[len(parts)-1])
	if len(last) == 0 {
		return parts[0]
	}
	return parts[0] + " " + string(last[0]) + "."
}

// toMessageResponses maps messages for the parent portal. A staff sender is
// shown as the individual person (first name + last initial) only when that
// message froze StaffNameVisible=true at send time; otherwise it collapses to
// the neutral "OGS [Schulname]" label. Guardian/system rows pass through
// unchanged.
func toMessageResponses(messages []*usersModels.ParentMessage, counterpart string) []MessageResponse {
	out := make([]MessageResponse, 0, len(messages))
	for _, m := range messages {
		senderName := m.SenderName
		if m.SenderKind == usersModels.ParentMessageSenderStaff {
			// StaffNameVisible is frozen per message at send time (see the model +
			// the operations.parent_message_staff_name_visible setting): true only
			// on replies written while the school had staff-name attribution on. So
			// older replies (and every reply when the school keeps it off) still
			// collapse to the neutral "OGS [Schulname]" label, while opted-in
			// replies show the person as "Vorname N.".
			if m.StaffNameVisible {
				senderName = staffShortName(m.SenderName)
			} else {
				senderName = counterpart
			}
		}
		refID := ""
		if m.RefID != nil {
			refID = strconv.FormatInt(*m.RefID, 10)
		}
		out = append(out, MessageResponse{
			ID:             strconv.FormatInt(m.ID, 10),
			SenderKind:     m.SenderKind,
			SenderName:     senderName,
			Body:           m.Body,
			CreatedAt:      m.CreatedAt,
			Kind:           m.Kind,
			EventType:      m.EventType,
			RequestType:    m.RequestType,
			RequestStatus:  m.RequestStatus,
			Payload:        m.Payload,
			RefTable:       m.RefTable,
			RefID:          refID,
			AppliedAt:      m.AppliedAt,
			DecisionReason: m.DecisionReason,
			ReadByStaff:    m.ReadByStaff,
		})
	}
	return out
}

func toThreadView(v *parentService.MessageThreadView) ThreadViewResponse {
	counterpart := ogsLabel(v.SchoolName)
	threadID := ""
	if v.ThreadID > 0 {
		threadID = strconv.FormatInt(v.ThreadID, 10)
	}
	return ThreadViewResponse{
		ThreadID:        threadID,
		StudentID:       strconv.FormatInt(v.StudentID, 10),
		StudentName:     v.StudentName,
		SchoolName:      v.SchoolName,
		CounterpartName: counterpart,
		Messages:        toMessageResponses(v.Messages, counterpart),
	}
}

func parseStudentID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	return common.ParsePositiveInt64IDWithError(w, r, "studentId", "invalid student ID")
}

// listMessageThreads returns every conversation the guardian owns.
func (rs *Resource) listMessageThreads(w http.ResponseWriter, r *http.Request) {
	accountID, ok := rs.parentAccountID(w, r)
	if !ok {
		return
	}
	threads, err := rs.ParentService.ListMessageThreads(r.Context(), accountID)
	if err != nil {
		renderParentWriteError(w, r, err)
		return
	}
	out := make([]ThreadSummaryResponse, 0, len(threads))
	for _, t := range threads {
		out = append(out, toThreadSummary(t))
	}
	common.Respond(w, r, http.StatusOK, out, "Threads retrieved")
}

// listChildThreads returns the guardian's conversation(s) about one owned child
// (at most one per the chat model). The child detail page uses this instead of
// fetching the whole inbox and filtering client-side. Reading it does NOT mark
// the thread read.
func (rs *Resource) listChildThreads(w http.ResponseWriter, r *http.Request) {
	accountID, ok := rs.parentAccountID(w, r)
	if !ok {
		return
	}
	studentID, ok := parseStudentID(w, r)
	if !ok {
		return
	}
	threads, err := rs.ParentService.ListChildThreads(r.Context(), accountID, studentID)
	if err != nil {
		renderParentWriteError(w, r, err)
		return
	}
	out := make([]ThreadSummaryResponse, 0, len(threads))
	for _, t := range threads {
		out = append(out, toThreadSummary(t))
	}
	common.Respond(w, r, http.StatusOK, out, "Threads retrieved")
}

// unreadMessageCount returns the guardian's total unread-conversation count for
// the portal sidebar badge — a light COUNT, so the badge never fetches the full
// thread list just to add up its numbers.
func (rs *Resource) unreadMessageCount(w http.ResponseWriter, r *http.Request) {
	accountID, ok := rs.parentAccountID(w, r)
	if !ok {
		return
	}
	count, err := rs.ParentService.UnreadMessageCount(r.Context(), accountID)
	if err != nil {
		renderParentWriteError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, map[string]int{"unread_count": count}, "Unread count retrieved")
}

// getChildConversation returns the guardian's conversation about one owned
// child and marks it read.
func (rs *Resource) getChildConversation(w http.ResponseWriter, r *http.Request) {
	accountID, ok := rs.parentAccountID(w, r)
	if !ok {
		return
	}
	studentID, ok := parseStudentID(w, r)
	if !ok {
		return
	}
	view, err := rs.ParentService.GetChildConversation(r.Context(), accountID, studentID)
	if err != nil {
		renderParentWriteError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, toThreadView(view), "Conversation retrieved")
}

// postChildMessage appends a guardian message to the child's conversation,
// creating it on the first message.
func (rs *Resource) postChildMessage(w http.ResponseWriter, r *http.Request) {
	accountID, ok := rs.parentAccountID(w, r)
	if !ok {
		return
	}
	studentID, ok := parseStudentID(w, r)
	if !ok {
		return
	}
	var req PostMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid request body")))
		return
	}
	view, err := rs.ParentService.PostChildMessage(r.Context(), accountID, studentID, req.Body)
	if err != nil {
		renderParentWriteError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusCreated, toThreadView(view), "Message sent")
}
