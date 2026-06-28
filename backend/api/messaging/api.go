// Package messaging is the staff-side (tenant portal) HTTP surface for the
// parent-OGS messaging feature: the central inbox, per-thread chats, replies,
// and starting new conversations. Mounted at /api/messages. The parent-portal
// side lives in api/parent.
package messaging

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	messagingService "github.com/moto-nrw/project-phoenix/services/messaging"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// Resource is the staff messaging HTTP resource.
type Resource struct {
	Service messagingService.Service
	db      *bun.DB
}

// NewResource wires the staff messaging resource.
func NewResource(service messagingService.Service, db *bun.DB) *Resource {
	return &Resource{Service: service, db: db}
}

// Router returns the chi router scoped to /messages.
func (rs *Resource) Router() chi.Router {
	r := chi.NewRouter()

	tokenAuth := jwt.MustNewTokenAuth()
	r.Group(func(r chi.Router) {
		r.Use(tokenAuth.Verifier())
		r.Use(jwt.Authenticator)
		r.Use(jwt.TenantMiddleware)
		withTx := tenant.TenantTxMiddleware(rs.db)

		// users:read is the coarse gate; per-child access is enforced in the
		// service via authorize.CanReadStudent. Starting/sending a thread
		// deliberately requires only users:read (+ CanReadStudent), NOT
		// users:update: messaging authority is defined to equal student-read
		// authority, so any staffer who may read a child may message that child's
		// guardians. Under gdpr.student_data_scope='all_staff' that is school-wide
		// — an accepted, signed-off policy (the guardian always sees the sender as
		// "OGS <Schule>", never an individual).
		read := authorize.RequiresPermission(permissions.UsersRead)
		// Confirming a request applies permanent schedule/master-data changes and
		// rejecting terminally decides it, so the decision endpoints require
		// users:update — the same coarse gate as the student mutation routes.
		// CanUpdateStudent (group supervision) alone does not imply that permission.
		write := authorize.RequiresPermission(permissions.UsersUpdate)
		r.With(read, withTx).Get("/", rs.listInbox)
		r.With(read, withTx).Get("/unread-count", rs.unreadCount)
		r.With(read, withTx).Post("/threads", rs.startThread)
		r.With(read, withTx).Post("/threads/open", rs.openThread)
		r.With(read, withTx).Get("/threads/{threadId}", rs.getThread)
		r.With(read, withTx).Post("/threads/{threadId}", rs.postMessage)
		r.With(read, withTx).Get("/students/{studentId}/guardians", rs.listGuardians)
		r.With(read, withTx).Get("/students/{studentId}/threads", rs.listStudentThreads)
		r.With(write, withTx).Post("/requests/{requestId}/confirm", rs.confirmRequest)
		r.With(write, withTx).Post("/requests/{requestId}/reject", rs.rejectRequest)
	})

	return r
}

// --- wire shapes (int64 ids stringified per the frontend convention) ---

type InboxThreadResponse struct {
	ThreadID         string     `json:"thread_id"`
	StudentID        string     `json:"student_id"`
	StudentName      string     `json:"student_name"`
	SchoolClass      string     `json:"school_class,omitempty"`
	GroupName        string     `json:"group_name,omitempty"`
	GuardianName     string     `json:"guardian_name"`
	RelationshipType string     `json:"relationship_type,omitempty"`
	LastMessageAt    *time.Time `json:"last_message_at,omitempty"`
	LastSenderKind   string     `json:"last_sender_kind,omitempty"`
	LastMessageBody  string     `json:"last_message_body,omitempty"`
	UnreadCount      int        `json:"unread_count"`
	OpenRequestCount int        `json:"open_request_count"`
}

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
	AppliedBy      string         `json:"applied_by,omitempty"`
	DecisionReason string         `json:"decision_reason,omitempty"`
	ReadByStaff    bool           `json:"read_by_staff,omitempty"`
	ReadByGuardian bool           `json:"read_by_guardian,omitempty"`
	Diff           []DiffEntry    `json:"diff,omitempty"`
}

// DiffEntry is one field an open request would change, shown to staff as
// "current → requested" so they can review before confirming.
type DiffEntry struct {
	Label string `json:"label"`
	Old   string `json:"old"`
	New   string `json:"new"`
}

type ThreadDetailResponse struct {
	ThreadID         string            `json:"thread_id"`
	StudentID        string            `json:"student_id"`
	StudentName      string            `json:"student_name"`
	GuardianName     string            `json:"guardian_name"`
	RelationshipType string            `json:"relationship_type,omitempty"`
	Messages         []MessageResponse `json:"messages"`
}

type GuardianResponse struct {
	AccountID        string `json:"account_id"`
	Name             string `json:"name"`
	RelationshipType string `json:"relationship_type,omitempty"`
	IsPrimary        bool   `json:"is_primary"`
}

type StartThreadRequest struct {
	StudentID         string `json:"student_id"`
	GuardianAccountID string `json:"guardian_account_id"`
	Body              string `json:"body"`
}

type PostMessageRequest struct {
	Body string `json:"body"`
}

type RejectRequestRequest struct {
	Reason string `json:"reason"`
}

type OpenThreadRequest struct {
	StudentID         string `json:"student_id"`
	GuardianAccountID string `json:"guardian_account_id"`
}

func toMessageResponses(messages []*usersModels.ParentMessage, diffs map[int64][]messagingService.RequestDiffEntry) []MessageResponse {
	out := make([]MessageResponse, 0, len(messages))
	for _, m := range messages {
		refID := ""
		if m.RefID != nil {
			refID = strconv.FormatInt(*m.RefID, 10)
		}
		appliedBy := ""
		if m.AppliedBy != nil {
			appliedBy = strconv.FormatInt(*m.AppliedBy, 10)
		}
		var diff []DiffEntry
		for _, d := range diffs[m.ID] {
			diff = append(diff, DiffEntry{Label: d.Label, Old: d.Old, New: d.New})
		}
		out = append(out, MessageResponse{
			ID:             strconv.FormatInt(m.ID, 10),
			SenderKind:     m.SenderKind,
			SenderName:     m.SenderName,
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
			AppliedBy:      appliedBy,
			DecisionReason: m.DecisionReason,
			ReadByStaff:    m.ReadByStaff,
			ReadByGuardian: m.ReadByGuardian,
			Diff:           diff,
		})
	}
	return out
}

func toThreadDetail(d *messagingService.ThreadDetail) ThreadDetailResponse {
	return ThreadDetailResponse{
		ThreadID:         strconv.FormatInt(d.ThreadID, 10),
		StudentID:        strconv.FormatInt(d.StudentID, 10),
		StudentName:      d.StudentName,
		GuardianName:     d.GuardianName,
		RelationshipType: d.RelationshipType,
		Messages:         toMessageResponses(d.Messages, d.Diffs),
	}
}

func (rs *Resource) listInbox(w http.ResponseWriter, r *http.Request) {
	rows, err := rs.Service.ListInbox(
		r.Context(),
		r.URL.Query().Get("unread") == "true",
		r.URL.Query().Get("open_requests") == "true",
	)
	if err != nil {
		renderMessagingError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, toInboxThreadResponses(rows), "Inbox retrieved")
}

func toInboxThreadResponses(rows []*usersModels.InboxThread) []InboxThreadResponse {
	out := make([]InboxThreadResponse, 0, len(rows))
	for _, t := range rows {
		out = append(out, InboxThreadResponse{
			ThreadID:         strconv.FormatInt(t.ThreadID, 10),
			StudentID:        strconv.FormatInt(t.StudentID, 10),
			StudentName:      t.StudentName,
			SchoolClass:      t.SchoolClass,
			GroupName:        t.GroupName,
			GuardianName:     t.GuardianName,
			RelationshipType: t.RelationshipType,
			LastMessageAt:    t.LastMessageAt,
			LastSenderKind:   t.LastSenderKind,
			LastMessageBody:  t.LastMessageBody,
			UnreadCount:      t.UnreadCount,
			OpenRequestCount: t.OpenRequestCount,
		})
	}
	return out
}

// listStudentThreads returns one child's conversations (staff view), so the
// student-detail card fetches only that child's threads instead of the whole
// tenant inbox.
func (rs *Resource) listStudentThreads(w http.ResponseWriter, r *http.Request) {
	studentID, ok := parseInt64Param(w, r, "studentId", "student")
	if !ok {
		return
	}
	rows, err := rs.Service.ListStudentThreads(r.Context(), studentID)
	if err != nil {
		renderMessagingError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, toInboxThreadResponses(rows), "Student threads retrieved")
}

func (rs *Resource) unreadCount(w http.ResponseWriter, r *http.Request) {
	count, err := rs.Service.UnreadMessageCount(r.Context())
	if err != nil {
		renderMessagingError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, map[string]int{"unread_count": count}, "Unread count retrieved")
}

func (rs *Resource) getThread(w http.ResponseWriter, r *http.Request) {
	threadID, ok := parseInt64Param(w, r, "threadId", "thread")
	if !ok {
		return
	}
	detail, err := rs.Service.GetThread(r.Context(), threadID)
	if err != nil {
		renderMessagingError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, toThreadDetail(detail), "Thread retrieved")
}

func (rs *Resource) confirmRequest(w http.ResponseWriter, r *http.Request) {
	requestID, ok := parseInt64Param(w, r, "requestId", "request")
	if !ok {
		return
	}
	messages, err := rs.Service.ConfirmRequest(r.Context(), requestID)
	if err != nil {
		renderMessagingError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, toMessageResponses(messages, nil), "Request confirmed")
}

func (rs *Resource) rejectRequest(w http.ResponseWriter, r *http.Request) {
	requestID, ok := parseInt64Param(w, r, "requestId", "request")
	if !ok {
		return
	}
	var req RejectRequestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid request body")))
		return
	}
	messages, err := rs.Service.RejectRequest(r.Context(), requestID, req.Reason)
	if err != nil {
		renderMessagingError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, toMessageResponses(messages, nil), "Request rejected")
}

func (rs *Resource) postMessage(w http.ResponseWriter, r *http.Request) {
	threadID, ok := parseInt64Param(w, r, "threadId", "thread")
	if !ok {
		return
	}
	var req PostMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid request body")))
		return
	}
	messages, err := rs.Service.PostMessage(r.Context(), threadID, req.Body)
	if err != nil {
		renderMessagingError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusCreated, toMessageResponses(messages, nil), "Message sent")
}

func (rs *Resource) startThread(w http.ResponseWriter, r *http.Request) {
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
	guardianID, err := strconv.ParseInt(req.GuardianAccountID, 10, 64)
	if err != nil || guardianID <= 0 {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid guardian ID")))
		return
	}
	detail, err := rs.Service.StartThread(r.Context(), studentID, guardianID, req.Body)
	if err != nil {
		renderMessagingError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusCreated, toThreadDetail(detail), "Thread created")
}

func (rs *Resource) openThread(w http.ResponseWriter, r *http.Request) {
	var req OpenThreadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid request body")))
		return
	}
	studentID, err := strconv.ParseInt(req.StudentID, 10, 64)
	if err != nil || studentID <= 0 {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid student ID")))
		return
	}
	guardianID, err := strconv.ParseInt(req.GuardianAccountID, 10, 64)
	if err != nil || guardianID <= 0 {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid guardian ID")))
		return
	}
	detail, err := rs.Service.OpenThread(r.Context(), studentID, guardianID)
	if err != nil {
		renderMessagingError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, toThreadDetail(detail), "Conversation opened")
}

func (rs *Resource) listGuardians(w http.ResponseWriter, r *http.Request) {
	studentID, ok := parseInt64Param(w, r, "studentId", "student")
	if !ok {
		return
	}
	guardians, err := rs.Service.ListGuardians(r.Context(), studentID)
	if err != nil {
		renderMessagingError(w, r, err)
		return
	}
	out := make([]GuardianResponse, 0, len(guardians))
	for _, g := range guardians {
		out = append(out, GuardianResponse{
			AccountID:        strconv.FormatInt(g.AccountID, 10),
			Name:             g.Name,
			RelationshipType: g.RelationshipType,
			IsPrimary:        g.IsPrimary,
		})
	}
	common.Respond(w, r, http.StatusOK, out, "Guardians retrieved")
}

func parseInt64Param(w http.ResponseWriter, r *http.Request, param, label string) (int64, bool) {
	id, err := strconv.ParseInt(chi.URLParam(r, param), 10, 64)
	if err != nil || id <= 0 {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid "+label+" ID")))
		return 0, false
	}
	return id, true
}

// renderMessagingError maps service sentinels to HTTP status codes.
func renderMessagingError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, messagingService.ErrThreadNotFound):
		common.RenderError(w, r, common.ErrorNotFound(err))
	case errors.Is(err, messagingService.ErrRequestNotFound):
		common.RenderError(w, r, common.ErrorNotFound(err))
	case errors.Is(err, messagingService.ErrForbidden), errors.Is(err, messagingService.ErrMessagingDisabled):
		common.RenderError(w, r, common.ErrorForbidden(err))
	case errors.Is(err, messagingService.ErrEmptyBody),
		errors.Is(err, messagingService.ErrBodyTooLong),
		errors.Is(err, messagingService.ErrInvalidGuardian),
		errors.Is(err, messagingService.ErrInvalidRequestStatus),
		errors.Is(err, messagingService.ErrRejectReasonRequired),
		errors.Is(err, messagingService.ErrRejectReasonTooLong),
		errors.Is(err, messagingService.ErrInvalidRequestType),
		errors.Is(err, messagingService.ErrInvalidRequestPayload):
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
	case errors.Is(err, messagingService.ErrRequestNoLongerPossible):
		common.RenderError(w, r, common.ErrorConflictMessage("Anfrage ist nicht mehr möglich. Bitte prüfen Sie die aktuellen Daten."))
	case errors.Is(err, messagingService.ErrGuardianAccessRevoked):
		common.RenderError(w, r, common.ErrorConflictMessage("Der Empfänger hat keinen Zugriff mehr auf dieses Kind."))
	default:
		common.RenderError(w, r, common.ErrorInternalServerWrap("messaging request failed", err))
	}
}
