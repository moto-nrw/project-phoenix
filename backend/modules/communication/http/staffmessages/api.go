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

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/communication"
)

// Resource is the internal messaging HTTP resource.
type Resource struct {
	Service communication.StaffMessagingCapability
}

// NewResource wires the internal messaging resource.
func NewResource(svc communication.StaffMessagingCapability) *Resource {
	return &Resource{Service: svc}
}

// Router returns the chi router scoped to /staff-messages.
//
// No permission gate beyond authentication, deliberately: every authenticated
// member of the school may write to every other member, and the conversations
// a caller can reach are decided by participation, enforced in the service and
// by RLS. Adding a permission here would suggest a granularity the feature does
// not have.
func (rs *Resource) Router(protectedTenantRoutes func(chi.Router, func(chi.Router, common.Middleware))) chi.Router {
	r := chi.NewRouter()
	protectedTenantRoutes(r, func(r chi.Router, withTx common.Middleware) {
		rs.registerRoutes(r, withTx)
		rs.registerGroupAdminRoutes(r, withTx)
	})
	return r
}

// SchoolRouter is the school-portal mantle of the same surface (#2208).
// Lehrkräfte take part in group chats (#4776) but neither start nor manage
// them, so the school portal gets the shared routes and leaving only:
// identical handlers and authorization (participation + active staff row),
// gated to school-scope tokens by ProtectedSchoolRoutes. A Lehrkraft holds a
// users.staff row (services/auth/school_identity.go), so the staffJoin that
// decides who is addressable admits them without a special case.
func (rs *Resource) SchoolRouter(protectedSchoolRoutes func(chi.Router, func(chi.Router, common.Middleware))) chi.Router {
	r := chi.NewRouter()
	protectedSchoolRoutes(r, rs.registerRoutes)
	return r
}

func (rs *Resource) registerRoutes(r chi.Router, withTx common.Middleware) {
	r.With(withTx).Get("/", rs.listInbox)
	r.With(withTx).Get("/unread-count", rs.unreadCount)
	r.With(withTx).Get("/recipients", rs.listRecipients)
	r.With(withTx).Post("/threads/open", rs.openThread)
	r.With(withTx).Get("/threads/{threadID}", rs.getThread)
	r.With(withTx).Post("/threads/{threadID}", rs.postMessage)
	r.With(withTx).Post("/threads/{threadID}/leave", rs.leaveGroup)
}

// registerGroupAdminRoutes mounts starting and managing group chats (#4776).
// The service decides who may: the creator or the leadership manage, a
// Lehrkraft never starts a group.
func (rs *Resource) registerGroupAdminRoutes(r chi.Router, withTx common.Middleware) {
	r.With(withTx).Post("/groups", rs.createGroup)
	r.With(withTx).Put("/threads/{threadID}/name", rs.renameGroup)
	r.With(withTx).Post("/threads/{threadID}/members", rs.addGroupMembers)
	r.With(withTx).Delete("/threads/{threadID}/members/{accountID}", rs.removeGroupMember)
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
	// Kind: "direct" | "group" (#4776). A group row carries group_name and
	// member_count; its counterpart fields are empty.
	Kind           string `json:"kind"`
	GroupName      string `json:"group_name,omitempty"`
	MemberCount    int    `json:"member_count"`
	LastSenderName string `json:"last_sender_name,omitempty"`
}

// MessageResponse is one chat message.
type MessageResponse struct {
	ID              string `json:"id"`
	SenderAccountID string `json:"sender_account_id"`
	SenderName      string `json:"sender_name"`
	Body            string `json:"body"`
	CreatedAt       string `json:"created_at"`
	// Group chats only (#4776): who of the members read the message and how
	// many members could have.
	ReadByAccountIDs []string `json:"read_by_account_ids,omitempty"`
	ReaderCount      int      `json:"reader_count"`
}

// ThreadDetailResponse is the chat window payload.
type ThreadDetailResponse struct {
	ThreadID             string            `json:"thread_id"`
	CounterpartAccountID string            `json:"counterpart_account_id"`
	CounterpartName      string            `json:"counterpart_name"`
	CounterpartRoleKind  string            `json:"counterpart_role_kind"`
	Messages             []MessageResponse `json:"messages"`
	// Group chats (#4776).
	Kind      string           `json:"kind"`
	GroupName string           `json:"group_name,omitempty"`
	CanManage bool             `json:"can_manage"`
	Members   []MemberResponse `json:"members"`
}

// MemberResponse is one person in a group chat.
type MemberResponse struct {
	AccountID string `json:"account_id"`
	Name      string `json:"name"`
	RoleKind  string `json:"role_kind"`
}

// RecipientResponse is one addressable colleague.
type RecipientResponse struct {
	AccountID string `json:"account_id"`
	Name      string `json:"name"`
	// RoleKind: "lehrkraft" | "admin" | "staff" (#2208).
	RoleKind string `json:"role_kind"`
}

func toMessageResponses(messages []communication.StaffMessage) []MessageResponse {
	out := make([]MessageResponse, 0, len(messages))
	for _, m := range messages {
		out = append(out, MessageResponse{
			ID:               strconv.FormatInt(m.ID, 10),
			SenderAccountID:  strconv.FormatInt(m.SenderAccountID, 10),
			SenderName:       m.SenderName,
			Body:             m.Body,
			CreatedAt:        m.CreatedAt.Format(time.RFC3339),
			ReadByAccountIDs: formatIDs(m.ReadByAccountIDs),
			ReaderCount:      m.ReaderCount,
		})
	}
	return out
}

func formatIDs(ids []int64) []string {
	if ids == nil {
		return nil
	}
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		out = append(out, strconv.FormatInt(id, 10))
	}
	return out
}

func toThreadDetail(d *communication.StaffMessageThread) ThreadDetailResponse {
	return ThreadDetailResponse{
		ThreadID:             strconv.FormatInt(d.ThreadID, 10),
		CounterpartAccountID: strconv.FormatInt(d.CounterpartAccountID, 10),
		CounterpartName:      d.CounterpartName,
		CounterpartRoleKind:  d.CounterpartRoleKind,
		Messages:             toMessageResponses(d.Messages),
		Kind:                 d.Kind,
		GroupName:            d.GroupName,
		CanManage:            d.CanManage,
		Members:              toMemberResponses(d.Members),
	}
}

func toMemberResponses(members []communication.StaffThreadMember) []MemberResponse {
	out := make([]MemberResponse, 0, len(members))
	for _, m := range members {
		out = append(out, MemberResponse{AccountID: strconv.FormatInt(m.AccountID, 10), Name: m.Name, RoleKind: m.RoleKind})
	}
	return out
}

// ---------------------------------------------------------------------------
// Handlers
// ---------------------------------------------------------------------------

func (rs *Resource) listInbox(w http.ResponseWriter, r *http.Request) {
	rows, err := rs.Service.ListStaffMessageInbox(r.Context(), r.URL.Query().Get("only_unread") == "true")
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
			Kind:                 row.Kind,
			GroupName:            row.GroupName,
			MemberCount:          row.MemberCount,
			LastSenderName:       row.LastSenderName,
		}
		if row.CounterpartAccountID == 0 {
			item.CounterpartAccountID = ""
		}
		if row.LastMessageAt != nil {
			item.LastMessageAt = row.LastMessageAt.Format(time.RFC3339)
		}
		out = append(out, item)
	}
	common.Respond(w, r, http.StatusOK, out, "Inbox retrieved")
}

func (rs *Resource) unreadCount(w http.ResponseWriter, r *http.Request) {
	count, err := rs.Service.CountUnreadStaffMessages(r.Context())
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

	detail, err := rs.Service.OpenStaffMessageThread(r.Context(), accountID)
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
	detail, err := rs.Service.GetStaffMessageThread(r.Context(), threadID)
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

	message, err := rs.Service.PostStaffMessage(r.Context(), threadID, body.Body)
	if err != nil {
		renderStaffMessagingError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, toMessageResponses([]communication.StaffMessage{*message})[0], "Message sent")
}
