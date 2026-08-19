// Package announcement is the staff-side (tenant portal) HTTP surface for
// parent announcements (Elternmitteilungen, #1669): authoring, publishing and
// read/ack stats for broadcast news to guardians. Mounted at
// /api/parent-announcements. The parent-portal feed lives in api/parent.
package announcement

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
	announcementService "github.com/moto-nrw/project-phoenix/services/announcement"
)

// Resource is the staff parent-announcement HTTP resource.
type Resource struct {
	Service announcementService.Service
	db      *bun.DB
}

// NewResource wires the staff announcement resource.
func NewResource(service announcementService.Service, db *bun.DB) *Resource {
	return &Resource{Service: service, db: db}
}

// Router returns the chi router scoped to /parent-announcements.
func (rs *Resource) Router() chi.Router {
	r := chi.NewRouter()

	common.ProtectedTenantGroup(r, rs.db, func(r chi.Router, withTx common.Middleware) {

		// Authoring parent broadcasts is ADMIN-ONLY in v1 (#1669 product
		// decision, 2026-07-02: "nur Admins"). The route is the only audience
		// gate — the service accepts school_all, arbitrary class/group/student
		// targets and any announcement id without checking the caller's own
		// scope — so a non-admin holder of communications:announce could
		// otherwise broadcast school-wide and read/unpublish/delete other
		// authors' announcements. Requiring admin:* here closes that hole
		// structurally; per-target scoping for a delegated announcer role (e.g.
		// a group lead limited to their own group) is deliberately deferred.
		// The parent-news feature flag is still re-checked in the service.
		announce := authorize.RequiresPermission(permissions.AdminWildcard)
		r.With(announce, withTx).Get("/", rs.list)
		r.With(announce, withTx).Post("/", rs.create)
		r.With(announce, withTx).Get("/{announcementId}", rs.get)
		r.With(announce, withTx).Put("/{announcementId}", rs.update)
		r.With(announce, withTx).Delete("/{announcementId}", rs.delete)
		r.With(announce, withTx).Post("/{announcementId}/publish", rs.publish)
		r.With(announce, withTx).Post("/{announcementId}/unpublish", rs.unpublish)
		r.With(announce, withTx).Get("/{announcementId}/stats", rs.stats)
		r.With(announce, withTx).Get("/{announcementId}/recipients", rs.recipients)
		// Poll (Umfrage, #1371): evaluation and the manual reminder to the
		// guardians of children that have not answered.
		r.With(announce, withTx).Get("/{announcementId}/poll-results", rs.pollResults)
		r.With(announce, withTx).Get("/{announcementId}/poll-children", rs.pollChildren)
		r.With(announce, withTx).Post("/{announcementId}/remind", rs.remindUnanswered)
	})

	return r
}

// --- wire shapes (int64 ids stringified per the frontend convention) ---

type targetRequest struct {
	TargetType string  `json:"target_type"`
	RefID      *string `json:"ref_id,omitempty"`
	RefText    *string `json:"ref_text,omitempty"`
}

type announcementRequest struct {
	Title                   string          `json:"title"`
	Body                    string          `json:"body"`
	Priority                string          `json:"priority"`
	LinkURL                 *string         `json:"link_url,omitempty"`
	RequiresAcknowledgement bool            `json:"requires_acknowledgement"`
	SendEmail               bool            `json:"send_email"`
	ExpiresAt               *time.Time      `json:"expires_at,omitempty"`
	Targets                 []targetRequest `json:"targets"`
	// Poll fields (#1371). response_type "" or "none" keeps the announcement a
	// plain Mitteilung; the choice types require options.
	ResponseType     string     `json:"response_type,omitempty"`
	ResponseDeadline *time.Time `json:"response_deadline,omitempty"`
	Options          []string   `json:"options,omitempty"`
	// delivery_mode "letter" is the Elternbrief (#2384); email_audience widens
	// the mail beyond the portal audience. Both default server-side when empty,
	// so pre-#2384 clients keep working unchanged.
	DeliveryMode  string `json:"delivery_mode,omitempty"`
	EmailAudience string `json:"email_audience,omitempty"`
}

// optionResponse is one answer option of a poll.
type optionResponse struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

type targetResponse struct {
	TargetType string  `json:"target_type"`
	RefID      *string `json:"ref_id,omitempty"`
	RefText    *string `json:"ref_text,omitempty"`
}

type announcementResponse struct {
	ID                      string           `json:"id"`
	Title                   string           `json:"title"`
	Body                    string           `json:"body"`
	Priority                string           `json:"priority"`
	LinkURL                 *string          `json:"link_url,omitempty"`
	RequiresAcknowledgement bool             `json:"requires_acknowledgement"`
	SendEmail               bool             `json:"send_email"`
	Status                  string           `json:"status"`
	PublishedAt             *time.Time       `json:"published_at,omitempty"`
	ExpiresAt               *time.Time       `json:"expires_at,omitempty"`
	Active                  bool             `json:"active"`
	CreatedAt               time.Time        `json:"created_at"`
	UpdatedAt               time.Time        `json:"updated_at"`
	Targets                 []targetResponse `json:"targets"`
	ResponseType            string           `json:"response_type"`
	ResponseDeadline        *time.Time       `json:"response_deadline,omitempty"`
	Options                 []optionResponse `json:"options"`
	DeliveryMode            string           `json:"delivery_mode"`
	EmailAudience           string           `json:"email_audience"`
}

type statsResponse struct {
	TargetCount       int `json:"target_count"`
	ReadCount         int `json:"read_count"`
	AcknowledgedCount int `json:"acknowledged_count"`
}

func announcementStatus(a *usersModels.ParentAnnouncement) string {
	if a.PublishedAt == nil {
		return "draft"
	}
	if a.ExpiresAt != nil && !a.ExpiresAt.After(time.Now()) {
		return "expired"
	}
	return "published"
}

func toTargetResponses(targets []*usersModels.ParentAnnouncementTarget) []targetResponse {
	out := make([]targetResponse, 0, len(targets))
	for _, t := range targets {
		resp := targetResponse{TargetType: t.TargetType, RefText: t.TargetRefText}
		if t.TargetRefID != nil {
			id := strconv.FormatInt(*t.TargetRefID, 10)
			resp.RefID = &id
		}
		out = append(out, resp)
	}
	return out
}

func toOptionResponses(options []*usersModels.ParentAnnouncementOption) []optionResponse {
	out := make([]optionResponse, 0, len(options))
	for _, o := range options {
		out = append(out, optionResponse{
			ID:    strconv.FormatInt(o.ID, 10),
			Label: o.Label,
		})
	}
	return out
}

func toAnnouncementResponse(a *usersModels.ParentAnnouncement) announcementResponse {
	return announcementResponse{
		ID:                      strconv.FormatInt(a.ID, 10),
		Title:                   a.Title,
		Body:                    a.Body,
		Priority:                a.Priority,
		LinkURL:                 a.LinkURL,
		RequiresAcknowledgement: a.RequiresAcknowledgement,
		SendEmail:               a.SendEmail,
		Status:                  announcementStatus(a),
		PublishedAt:             a.PublishedAt,
		ExpiresAt:               a.ExpiresAt,
		Active:                  a.Active,
		CreatedAt:               a.CreatedAt,
		UpdatedAt:               a.UpdatedAt,
		Targets:                 toTargetResponses(a.Targets),
		ResponseType:            a.ResponseType,
		ResponseDeadline:        a.ResponseDeadline,
		Options:                 toOptionResponses(a.Options),
		DeliveryMode:            a.DeliveryMode,
		EmailAudience:           a.EmailAudience,
	}
}

// toInput maps the wire request to the service input, parsing stringified
// entity ids. An unparseable ref_id is a client error.
func toInput(req announcementRequest) (announcementService.Input, error) {
	in := announcementService.Input{
		Title:                   req.Title,
		Body:                    req.Body,
		Priority:                req.Priority,
		LinkURL:                 req.LinkURL,
		RequiresAcknowledgement: req.RequiresAcknowledgement,
		SendEmail:               req.SendEmail,
		ExpiresAt:               req.ExpiresAt,
		ResponseType:            req.ResponseType,
		ResponseDeadline:        req.ResponseDeadline,
		Options:                 req.Options,
		DeliveryMode:            req.DeliveryMode,
		EmailAudience:           req.EmailAudience,
		Targets:                 make([]announcementService.TargetInput, 0, len(req.Targets)),
	}
	for _, t := range req.Targets {
		target := announcementService.TargetInput{TargetType: t.TargetType, RefText: t.RefText}
		if t.RefID != nil && *t.RefID != "" {
			id, err := strconv.ParseInt(*t.RefID, 10, 64)
			if err != nil || id <= 0 {
				return announcementService.Input{}, errors.New("invalid target ref_id")
			}
			target.RefID = &id
		}
		in.Targets = append(in.Targets, target)
	}
	return in, nil
}

func (rs *Resource) list(w http.ResponseWriter, r *http.Request) {
	includeInactive := r.URL.Query().Get("include_inactive") == "true"
	rows, err := rs.Service.List(r.Context(), includeInactive)
	if err != nil {
		renderAnnouncementError(w, r, err)
		return
	}
	out := make([]announcementResponse, 0, len(rows))
	for _, a := range rows {
		out = append(out, toAnnouncementResponse(a))
	}
	common.Respond(w, r, http.StatusOK, out, "Announcements retrieved")
}

func (rs *Resource) get(w http.ResponseWriter, r *http.Request) {
	id, ok := parseAnnouncementID(w, r)
	if !ok {
		return
	}
	a, err := rs.Service.Get(r.Context(), id)
	if err != nil {
		renderAnnouncementError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, toAnnouncementResponse(a), "Announcement retrieved")
}

func (rs *Resource) create(w http.ResponseWriter, r *http.Request) {
	in, ok := decodeInput(w, r)
	if !ok {
		return
	}
	claims := jwt.ClaimsFromCtx(r.Context())
	a, err := rs.Service.Create(r.Context(), int64(claims.ID), in)
	if err != nil {
		renderAnnouncementError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusCreated, toAnnouncementResponse(a), "Announcement created")
}

func (rs *Resource) update(w http.ResponseWriter, r *http.Request) {
	id, ok := parseAnnouncementID(w, r)
	if !ok {
		return
	}
	in, ok := decodeInput(w, r)
	if !ok {
		return
	}
	a, err := rs.Service.Update(r.Context(), id, in)
	if err != nil {
		renderAnnouncementError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, toAnnouncementResponse(a), "Announcement updated")
}

func (rs *Resource) delete(w http.ResponseWriter, r *http.Request) {
	id, ok := parseAnnouncementID(w, r)
	if !ok {
		return
	}
	if err := rs.Service.Delete(r.Context(), id); err != nil {
		renderAnnouncementError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, map[string]bool{"deleted": true}, "Announcement deleted")
}

func (rs *Resource) publish(w http.ResponseWriter, r *http.Request) {
	id, ok := parseAnnouncementID(w, r)
	if !ok {
		return
	}
	a, err := rs.Service.Publish(r.Context(), id)
	if err != nil {
		renderAnnouncementError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, toAnnouncementResponse(a), "Announcement published")
}

func (rs *Resource) unpublish(w http.ResponseWriter, r *http.Request) {
	id, ok := parseAnnouncementID(w, r)
	if !ok {
		return
	}
	a, err := rs.Service.Unpublish(r.Context(), id)
	if err != nil {
		renderAnnouncementError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, toAnnouncementResponse(a), "Announcement unpublished")
}

func (rs *Resource) stats(w http.ResponseWriter, r *http.Request) {
	id, ok := parseAnnouncementID(w, r)
	if !ok {
		return
	}
	s, err := rs.Service.Stats(r.Context(), id)
	if err != nil {
		renderAnnouncementError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, statsResponse{
		TargetCount:       s.TargetCount,
		ReadCount:         s.ReadCount,
		AcknowledgedCount: s.AcknowledgedCount,
	}, "Stats retrieved")
}

// recipientResponse is one guardian account in the live audience with their
// read/ack state. status is derived server-side so the client never has to
// re-implement the precedence (acknowledged > read > pending).
type recipientResponse struct {
	AccountID      string     `json:"account_id"`
	FirstName      string     `json:"first_name"`
	LastName       string     `json:"last_name"`
	Status         string     `json:"status"`
	ReadAt         *time.Time `json:"read_at,omitempty"`
	AcknowledgedAt *time.Time `json:"acknowledged_at,omitempty"`
}

func (rs *Resource) recipients(w http.ResponseWriter, r *http.Request) {
	id, ok := parseAnnouncementID(w, r)
	if !ok {
		return
	}
	list, err := rs.Service.Recipients(r.Context(), id)
	if err != nil {
		renderAnnouncementError(w, r, err)
		return
	}
	out := make([]recipientResponse, 0, len(list))
	for _, rcpt := range list {
		status := "pending"
		switch {
		case rcpt.AcknowledgedAt != nil:
			status = "acknowledged"
		case rcpt.ReadAt != nil:
			status = "read"
		}
		out = append(out, recipientResponse{
			AccountID:      strconv.FormatInt(rcpt.AccountID, 10),
			FirstName:      rcpt.FirstName,
			LastName:       rcpt.LastName,
			Status:         status,
			ReadAt:         rcpt.ReadAt,
			AcknowledgedAt: rcpt.AcknowledgedAt,
		})
	}
	common.Respond(w, r, http.StatusOK, out, "Recipients retrieved")
}

// --- poll (Umfrage) ---

type pollOptionResultResponse struct {
	OptionID string `json:"option_id"`
	Label    string `json:"label"`
	Count    int    `json:"count"`
}

type pollResultsResponse struct {
	ChildCount       int                        `json:"child_count"`
	TargetChildCount int                        `json:"target_child_count"`
	AnsweredCount    int                        `json:"answered_count"`
	Options          []pollOptionResultResponse `json:"options"`
}

// pollChildResponse is one reached child with the answer given for them and
// whether any guardian may currently respond. answer_labels is empty while an
// answer is still open.
type pollChildResponse struct {
	StudentID    string     `json:"student_id"`
	FirstName    string     `json:"first_name"`
	LastName     string     `json:"last_name"`
	SchoolClass  string     `json:"school_class"`
	AnswerLabels []string   `json:"answer_labels"`
	RespondedAt  *time.Time `json:"responded_at,omitempty"`
	CanAnswer    bool       `json:"can_answer"`
}

func (rs *Resource) pollResults(w http.ResponseWriter, r *http.Request) {
	id, ok := parseAnnouncementID(w, r)
	if !ok {
		return
	}
	results, err := rs.Service.PollResults(r.Context(), id)
	if err != nil {
		renderAnnouncementError(w, r, err)
		return
	}
	out := pollResultsResponse{
		ChildCount:       results.ChildCount,
		TargetChildCount: results.TargetChildCount,
		AnsweredCount:    results.AnsweredCount,
		Options:          make([]pollOptionResultResponse, 0, len(results.Options)),
	}
	for _, o := range results.Options {
		out.Options = append(out.Options, pollOptionResultResponse{
			OptionID: strconv.FormatInt(o.OptionID, 10),
			Label:    o.Label,
			Count:    o.Count,
		})
	}
	common.Respond(w, r, http.StatusOK, out, "Poll results retrieved")
}

func (rs *Resource) pollChildren(w http.ResponseWriter, r *http.Request) {
	id, ok := parseAnnouncementID(w, r)
	if !ok {
		return
	}
	rows, err := rs.Service.PollChildren(r.Context(), id)
	if err != nil {
		renderAnnouncementError(w, r, err)
		return
	}
	out := make([]pollChildResponse, 0, len(rows))
	for _, c := range rows {
		labels := c.AnswerLabels
		if labels == nil {
			labels = []string{}
		}
		out = append(out, pollChildResponse{
			StudentID:    strconv.FormatInt(c.StudentID, 10),
			FirstName:    c.FirstName,
			LastName:     c.LastName,
			SchoolClass:  c.SchoolClass,
			AnswerLabels: labels,
			RespondedAt:  c.RespondedAt,
			CanAnswer:    c.CanAnswer,
		})
	}
	common.Respond(w, r, http.StatusOK, out, "Poll children retrieved")
}

func (rs *Resource) remindUnanswered(w http.ResponseWriter, r *http.Request) {
	id, ok := parseAnnouncementID(w, r)
	if !ok {
		return
	}
	count, err := rs.Service.RemindUnanswered(r.Context(), id)
	if err != nil {
		renderAnnouncementError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, map[string]int{"reminded_count": count}, "Reminder sent")
}

func decodeInput(w http.ResponseWriter, r *http.Request) (announcementService.Input, bool) {
	var req announcementRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid request body")))
		return announcementService.Input{}, false
	}
	in, err := toInput(req)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return announcementService.Input{}, false
	}
	return in, true
}

func parseAnnouncementID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	return common.ParsePositiveInt64IDWithError(w, r, "announcementId", "invalid announcement ID")
}

// renderAnnouncementError maps service sentinels to HTTP status codes.
func renderAnnouncementError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, announcementService.ErrNotFound):
		common.RenderError(w, r, common.ErrorNotFound(err))
	case errors.Is(err, announcementService.ErrNewsDisabled):
		common.RenderError(w, r, common.ErrorForbiddenWithCode(err, "parent_news_disabled"))
	case errors.Is(err, announcementService.ErrPublishedImmutable):
		common.RenderError(w, r, common.ErrorConflictWithCode(err, "announcement_published_immutable"))
	case errors.Is(err, announcementService.ErrNotAPoll):
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
	case errors.Is(err, announcementService.ErrPollNotOpen):
		common.RenderError(w, r, common.ErrorConflictWithCode(err, "poll_not_open"))
	case errors.Is(err, announcementService.ErrValidation):
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
	default:
		common.RenderError(w, r, common.ErrorInternalServerWrap("announcement request failed", err))
	}
}
