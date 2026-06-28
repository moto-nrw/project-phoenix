package parent

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/moto-nrw/project-phoenix/api/common"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	messagingService "github.com/moto-nrw/project-phoenix/services/messaging"
	parentService "github.com/moto-nrw/project-phoenix/services/parent"
)

// MessageResponse is one message in a conversation. IDs stringified per the
// int64 -> string frontend convention. sender_kind is "guardian" or "staff".
// For staff messages sender_name is the OGS/school label, never an individual
// staff member's name — the parent talks to "the OGS", not to a person.
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
	Diff           []DiffEntry    `json:"diff,omitempty"`
}

// DiffEntry is one field an open request would change, shown to the guardian as
// "current → requested" so they see what they asked to change, not just a
// status. Absent for closed/applied requests.
type DiffEntry struct {
	Label string `json:"label"`
	Old   string `json:"old"`
	New   string `json:"new"`
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
	Unread          int        `json:"unread"`
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

// CreateChildRequestRequest is the wire shape for POST
// .../children/{studentId}/requests.
type CreateChildRequestRequest struct {
	RequestType string         `json:"request_type"`
	Payload     map[string]any `json:"payload"`
}

// ogsLabel is the parent-facing name of the OGS counterpart for a school.
func ogsLabel(schoolName string) string {
	return strings.TrimSpace("OGS " + schoolName)
}

// toMessageResponses maps messages for the parent portal, replacing staff
// sender names with the OGS/school label so individual staff names never leave
// the backend.
func toMessageResponses(messages []*usersModels.ParentMessage, counterpart string, diffs map[int64][]messagingService.RequestDiffEntry) []MessageResponse {
	out := make([]MessageResponse, 0, len(messages))
	for _, m := range messages {
		senderName := m.SenderName
		if m.SenderKind == usersModels.ParentMessageSenderStaff {
			senderName = counterpart
		}
		refID := ""
		if m.RefID != nil {
			refID = strconv.FormatInt(*m.RefID, 10)
		}
		var diff []DiffEntry
		for _, d := range diffs[m.ID] {
			diff = append(diff, DiffEntry{Label: d.Label, Old: d.Old, New: d.New})
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
			Diff:           diff,
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
		Messages:        toMessageResponses(v.Messages, counterpart, v.Diffs),
	}
}

// createChildRequest appends a structured parent change request to the child's
// conversation, creating the conversation on the first message.
func (rs *Resource) createChildRequest(w http.ResponseWriter, r *http.Request) {
	accountID, ok := rs.parentAccountID(w, r)
	if !ok {
		return
	}
	studentID, ok := parseStudentID(w, r)
	if !ok {
		return
	}
	var req CreateChildRequestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid request body")))
		return
	}
	view, err := rs.ParentService.CreateChildRequest(r.Context(), accountID, studentID, req.RequestType, req.Payload)
	if err != nil {
		renderParentWriteError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusCreated, toThreadView(view), "Request created")
}

// withdrawChildRequest flips an open guardian request to withdrawn.
func (rs *Resource) withdrawChildRequest(w http.ResponseWriter, r *http.Request) {
	accountID, ok := rs.parentAccountID(w, r)
	if !ok {
		return
	}
	studentID, ok := parseStudentID(w, r)
	if !ok {
		return
	}
	requestID, err := strconv.ParseInt(chi.URLParam(r, "requestId"), 10, 64)
	if err != nil || requestID <= 0 {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid request ID")))
		return
	}
	view, err := rs.ParentService.WithdrawChildRequest(r.Context(), accountID, studentID, requestID)
	if err != nil {
		renderParentWriteError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, toThreadView(view), "Request withdrawn")
}

func parseStudentID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(chi.URLParam(r, "studentId"), 10, 64)
	if err != nil || id <= 0 {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid student ID")))
		return 0, false
	}
	return id, true
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
		out = append(out, ThreadSummaryResponse{
			ThreadID:        strconv.FormatInt(t.ThreadID, 10),
			StudentID:       strconv.FormatInt(t.StudentID, 10),
			StudentName:     t.StudentName,
			SchoolName:      t.SchoolName,
			CounterpartName: ogsLabel(t.SchoolName),
			LastMessageAt:   t.LastMessageAt,
			LastSenderKind:  t.LastSenderKind,
			LastMessageBody: t.LastMessageBody,
			Unread:          t.UnreadCount,
		})
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
		out = append(out, ThreadSummaryResponse{
			ThreadID:        strconv.FormatInt(t.ThreadID, 10),
			StudentID:       strconv.FormatInt(t.StudentID, 10),
			StudentName:     t.StudentName,
			SchoolName:      t.SchoolName,
			CounterpartName: ogsLabel(t.SchoolName),
			LastMessageAt:   t.LastMessageAt,
			LastSenderKind:  t.LastSenderKind,
			LastMessageBody: t.LastMessageBody,
			Unread:          t.UnreadCount,
		})
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
