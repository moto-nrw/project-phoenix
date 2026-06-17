package parent

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/moto-nrw/project-phoenix/api/common"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	parentService "github.com/moto-nrw/project-phoenix/services/parent"
)

// MessageResponse is one message in a thread. IDs stringified per the int64 ->
// string frontend convention. sender_kind is "guardian" or "staff".
type MessageResponse struct {
	ID         string    `json:"id"`
	SenderKind string    `json:"sender_kind"`
	SenderName string    `json:"sender_name"`
	Body       string    `json:"body"`
	CreatedAt  time.Time `json:"created_at"`
}

// ThreadSummaryResponse is one row on the parent thread list.
type ThreadSummaryResponse struct {
	ThreadID        string     `json:"thread_id"`
	Subject         string     `json:"subject"`
	StudentID       string     `json:"student_id"`
	StudentName     string     `json:"student_name"`
	LastMessageAt   *time.Time `json:"last_message_at,omitempty"`
	LastSenderKind  string     `json:"last_sender_kind,omitempty"`
	LastMessageBody string     `json:"last_message_body,omitempty"`
	Unread          int        `json:"unread"`
}

// ThreadViewResponse is the full conversation for one thread.
type ThreadViewResponse struct {
	ThreadID    string            `json:"thread_id"`
	Subject     string            `json:"subject"`
	StudentID   string            `json:"student_id"`
	StudentName string            `json:"student_name"`
	Messages    []MessageResponse `json:"messages"`
}

// StartThreadRequest is the wire shape for POST /parent/me/messages/threads.
type StartThreadRequest struct {
	StudentID string `json:"student_id"`
	Subject   string `json:"subject"`
	Body      string `json:"body"`
}

// PostMessageRequest is the wire shape for POST .../threads/{threadId}.
type PostMessageRequest struct {
	Body string `json:"body"`
}

func toMessageResponses(messages []*usersModels.ParentMessage) []MessageResponse {
	out := make([]MessageResponse, 0, len(messages))
	for _, m := range messages {
		out = append(out, MessageResponse{
			ID:         strconv.FormatInt(m.ID, 10),
			SenderKind: m.SenderKind,
			SenderName: m.SenderName,
			Body:       m.Body,
			CreatedAt:  m.CreatedAt,
		})
	}
	return out
}

func toThreadView(v *parentService.MessageThreadView) ThreadViewResponse {
	return ThreadViewResponse{
		ThreadID:    strconv.FormatInt(v.ThreadID, 10),
		Subject:     v.Subject,
		StudentID:   strconv.FormatInt(v.StudentID, 10),
		StudentName: v.StudentName,
		Messages:    toMessageResponses(v.Messages),
	}
}

func parseThreadID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(chi.URLParam(r, "threadId"), 10, 64)
	if err != nil || id <= 0 {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid thread ID")))
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
			Subject:         t.Subject,
			StudentID:       strconv.FormatInt(t.StudentID, 10),
			StudentName:     t.StudentName,
			LastMessageAt:   t.LastMessageAt,
			LastSenderKind:  t.LastSenderKind,
			LastMessageBody: t.LastMessageBody,
			Unread:          t.UnreadCount,
		})
	}
	common.Respond(w, r, http.StatusOK, out, "Threads retrieved")
}

// getMessageThread returns one owned conversation and marks it read.
func (rs *Resource) getMessageThread(w http.ResponseWriter, r *http.Request) {
	accountID, ok := rs.parentAccountID(w, r)
	if !ok {
		return
	}
	threadID, ok := parseThreadID(w, r)
	if !ok {
		return
	}
	view, err := rs.ParentService.GetMessageThread(r.Context(), accountID, threadID)
	if err != nil {
		renderParentWriteError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, toThreadView(view), "Thread retrieved")
}

// postThreadMessage appends a guardian message to an owned thread.
func (rs *Resource) postThreadMessage(w http.ResponseWriter, r *http.Request) {
	accountID, ok := rs.parentAccountID(w, r)
	if !ok {
		return
	}
	threadID, ok := parseThreadID(w, r)
	if !ok {
		return
	}
	var req PostMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid request body")))
		return
	}
	view, err := rs.ParentService.PostThreadMessage(r.Context(), accountID, threadID, req.Body)
	if err != nil {
		renderParentWriteError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusCreated, toThreadView(view), "Message sent")
}

// startThread opens a new conversation about an owned child.
func (rs *Resource) startThread(w http.ResponseWriter, r *http.Request) {
	accountID, ok := rs.parentAccountID(w, r)
	if !ok {
		return
	}
	var req StartThreadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid request body")))
		return
	}
	studentID, err := strconv.ParseInt(req.StudentID, 10, 64)
	if err != nil || studentID <= 0 {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid student ID")))
		return
	}
	view, err := rs.ParentService.StartThread(r.Context(), accountID, studentID, req.Subject, req.Body)
	if err != nil {
		renderParentWriteError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusCreated, toThreadView(view), "Thread created")
}

// markMessageThreadRead advances the guardian's read cursor for a thread.
func (rs *Resource) markMessageThreadRead(w http.ResponseWriter, r *http.Request) {
	accountID, ok := rs.parentAccountID(w, r)
	if !ok {
		return
	}
	threadID, ok := parseThreadID(w, r)
	if !ok {
		return
	}
	if err := rs.ParentService.MarkMessageThreadRead(r.Context(), accountID, threadID); err != nil {
		renderParentWriteError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, map[string]bool{"ok": true}, "Thread marked read")
}
