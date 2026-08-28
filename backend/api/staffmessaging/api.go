// Package staffmessaging is the tenant-portal HTTP surface for the OGS-internal
// colleague chat (#2598): the caller's inbox, per-conversation chats, sending,
// and the recipient picker. Mounted at /api/staff-messages.
//
// There is no parent-portal counterpart and there never should be — this
// surface is staff-to-staff by definition.
package staffmessaging

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/api/common"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	service "github.com/moto-nrw/project-phoenix/services/staffmessaging"
)

// Resource is the internal messaging HTTP resource.
type Resource struct {
	Service *service.Service
	db      *bun.DB
}

// NewResource wires the internal messaging resource.
func NewResource(svc *service.Service, db *bun.DB) *Resource {
	return &Resource{Service: svc, db: db}
}

// Router returns the chi router scoped to /staff-messages.
//
// No permission gate beyond authentication, deliberately: every authenticated
// member of the school may write to every other member, and the conversations
// a caller can reach are decided by participation, enforced in the service and
// by RLS. Adding a permission here would suggest a granularity the feature does
// not have.
func (rs *Resource) Router() chi.Router {
	r := chi.NewRouter()
	common.ProtectedTenantGroup(r, rs.db, rs.registerRoutes)
	return r
}

// SchoolRouter is the school-portal mantle of the same surface (#2208):
// identical handlers and authorization (participation + active staff row),
// gated to school-scope tokens by ProtectedSchoolGroup. A Lehrkraft holds a
// users.staff row (services/auth/school_identity.go), so the staffJoin that
// decides who is addressable admits them without a special case.
func (rs *Resource) SchoolRouter() chi.Router {
	r := chi.NewRouter()
	common.ProtectedSchoolGroup(r, rs.db, rs.registerRoutes)
	return r
}

func (rs *Resource) registerRoutes(r chi.Router, withTx common.Middleware) {
	r.With(withTx).Get("/", rs.listInbox)
	r.With(withTx).Get("/unread-count", rs.unreadCount)
	r.With(withTx).Get("/recipients", rs.listRecipients)
	r.With(withTx).Post("/threads/open", rs.openThread)
	r.With(withTx).Get("/threads/{threadID}", rs.getThread)
	r.With(withTx).Post("/threads/{threadID}", rs.postMessage)
}

// ---------------------------------------------------------------------------
// Wire format
// ---------------------------------------------------------------------------

// InboxThreadResponse is one row of the caller's conversation list.
type InboxThreadResponse struct {
	ThreadID             string `json:"thread_id"`
	CounterpartAccountID string `json:"counterpart_account_id"`
	CounterpartName      string `json:"counterpart_name"`
	// CounterpartRoleKind: "lehrkraft" | "admin" | "staff" (#2208).
	CounterpartRoleKind string `json:"counterpart_role_kind"`
	LastMessageAt       string `json:"last_message_at,omitempty"`
	LastMessageBody     string `json:"last_message_body,omitempty"`
	LastMessageMine     bool   `json:"last_message_mine"`
	UnreadCount         int    `json:"unread_count"`
}

// MessageResponse is one chat message.
type MessageResponse struct {
	ID              string `json:"id"`
	SenderAccountID string `json:"sender_account_id"`
	SenderName      string `json:"sender_name"`
	Body            string `json:"body"`
	CreatedAt       string `json:"created_at"`
}

// ThreadDetailResponse is the chat window payload.
type ThreadDetailResponse struct {
	ThreadID             string            `json:"thread_id"`
	CounterpartAccountID string            `json:"counterpart_account_id"`
	CounterpartName      string            `json:"counterpart_name"`
	CounterpartRoleKind  string            `json:"counterpart_role_kind"`
	Messages             []MessageResponse `json:"messages"`
}

// RecipientResponse is one addressable colleague.
type RecipientResponse struct {
	AccountID string `json:"account_id"`
	Name      string `json:"name"`
	// RoleKind: "lehrkraft" | "admin" | "staff" (#2208).
	RoleKind string `json:"role_kind"`
}

func toMessageResponses(messages []*usersModels.StaffMessage) []MessageResponse {
	out := make([]MessageResponse, 0, len(messages))
	for _, m := range messages {
		out = append(out, MessageResponse{
			ID:              strconv.FormatInt(m.ID, 10),
			SenderAccountID: strconv.FormatInt(m.SenderAccountID, 10),
			SenderName:      m.SenderName,
			Body:            m.Body,
			CreatedAt:       m.CreatedAt.Format(time.RFC3339),
		})
	}
	return out
}

func toThreadDetail(d *service.ThreadDetail) ThreadDetailResponse {
	return ThreadDetailResponse{
		ThreadID:             strconv.FormatInt(d.ThreadID, 10),
		CounterpartAccountID: strconv.FormatInt(d.CounterpartAccountID, 10),
		CounterpartName:      d.CounterpartName,
		CounterpartRoleKind:  d.CounterpartRoleKind,
		Messages:             toMessageResponses(d.Messages),
	}
}

// ---------------------------------------------------------------------------
// Handlers
// ---------------------------------------------------------------------------

func (rs *Resource) listInbox(w http.ResponseWriter, r *http.Request) {
	rows, err := rs.Service.ListInbox(r.Context(), r.URL.Query().Get("only_unread") == "true")
	if err != nil {
		renderStaffMessagingError(w, r, err)
		return
	}

	actor := currentAccountID(r)
	out := make([]InboxThreadResponse, 0, len(rows))
	for _, row := range rows {
		item := InboxThreadResponse{
			ThreadID:             strconv.FormatInt(row.ThreadID, 10),
			CounterpartAccountID: strconv.FormatInt(row.CounterpartAccountID, 10),
			CounterpartName:      row.CounterpartName,
			CounterpartRoleKind:  row.CounterpartRoleKind,
			LastMessageBody:      row.LastMessageBody,
			UnreadCount:          row.UnreadCount,
			LastMessageMine:      row.LastSenderAccountID != nil && *row.LastSenderAccountID == actor,
		}
		if row.LastMessageAt != nil {
			item.LastMessageAt = row.LastMessageAt.Format(time.RFC3339)
		}
		out = append(out, item)
	}
	common.Respond(w, r, http.StatusOK, out, "Inbox retrieved")
}

func (rs *Resource) unreadCount(w http.ResponseWriter, r *http.Request) {
	count, err := rs.Service.UnreadMessageCount(r.Context())
	if err != nil {
		renderStaffMessagingError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, map[string]int{"unread_count": count}, "Unread count retrieved")
}

func (rs *Resource) listRecipients(w http.ResponseWriter, r *http.Request) {
	rows, err := rs.Service.ListMessageableStaff(r.Context())
	if err != nil {
		renderStaffMessagingError(w, r, err)
		return
	}
	out := make([]RecipientResponse, 0, len(rows))
	for _, row := range rows {
		out = append(out, RecipientResponse{
			AccountID: strconv.FormatInt(row.AccountID, 10),
			Name:      row.Name,
			RoleKind:  row.RoleKind,
		})
	}
	common.Respond(w, r, http.StatusOK, out, "Recipients retrieved")
}

func (rs *Resource) openThread(w http.ResponseWriter, r *http.Request) {
	var body struct {
		AccountID string `json:"account_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	accountID, err := strconv.ParseInt(body.AccountID, 10, 64)
	if err != nil || accountID <= 0 {
		common.RenderError(w, r, common.ErrorInvalidRequestMessage("Ungültige Empfänger-ID."))
		return
	}

	detail, err := rs.Service.OpenThread(r.Context(), accountID)
	if err != nil {
		renderStaffMessagingError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, toThreadDetail(detail), "Thread opened")
}

func (rs *Resource) getThread(w http.ResponseWriter, r *http.Request) {
	threadID, ok := common.ParsePositiveInt64IDWithError(w, r, "threadID", "invalid thread ID")
	if !ok {
		return
	}
	detail, err := rs.Service.GetThread(r.Context(), threadID)
	if err != nil {
		renderStaffMessagingError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, toThreadDetail(detail), "Thread retrieved")
}

func (rs *Resource) postMessage(w http.ResponseWriter, r *http.Request) {
	threadID, ok := common.ParsePositiveInt64IDWithError(w, r, "threadID", "invalid thread ID")
	if !ok {
		return
	}
	var body struct {
		Body string `json:"body"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	message, err := rs.Service.PostMessage(r.Context(), threadID, body.Body)
	if err != nil {
		renderStaffMessagingError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, toMessageResponses([]*usersModels.StaffMessage{message})[0], "Message sent")
}
