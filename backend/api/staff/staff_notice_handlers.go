// Tagesinformationen (#2180) — HTTP-Fläche der Hinweise fürs Team:
// interne Hinweise der Leitung an das Team. Eingehängt unter
// /api/staff-notices.
//
// Zwei Rechte, zwei Zwecke: LESEN darf jede Person mit users:read (das hält
// jede Betreuungskraft), SCHREIBEN nur ein Admin. Der Zuschnitt steht bewusst
// hier an der Route und nicht nur im Frontend — ein Baustein der Startseite,
// den man ohne Recht trotzdem abrufen könnte, wäre kein Zuschnitt, sondern
// eine Kulisse.
package staff

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
)

// StaffNoticeResource ist die HTTP-Ressource der Tagesinformationen.
type StaffNoticeResource struct {
	Service scheduleSvc.StaffNoticeService
	db      *bun.DB
}

// NewStaffNoticeResource verdrahtet die Ressource.
func NewStaffNoticeResource(service scheduleSvc.StaffNoticeService, db *bun.DB) *StaffNoticeResource {
	return &StaffNoticeResource{Service: service, db: db}
}

// Router liefert den Router unter /staff-notices.
func (rs *StaffNoticeResource) Router() chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))

	common.ProtectedTenantGroup(r, rs.db, func(r chi.Router, withTx common.Middleware) {
		read := common.RequiresPermission(permissions.UsersRead)
		// Schreiben ist adminexklusiv: ein Hinweis erreicht die ganze
		// Einrichtung, es gibt also keine Zielgruppe, an der sich ein feinerer
		// Zuschnitt festmachen ließe.
		write := common.RequiresPermission(permissions.AdminWildcard)

		r.With(read, withTx).Get("/today", rs.today)
		r.With(read, withTx).Post("/{noticeId}/acknowledge", rs.acknowledge)

		r.With(write, withTx).Get("/", rs.list)
		r.With(write, withTx).Post("/", rs.create)
		r.With(write, withTx).Put("/{noticeId}", rs.update)
		r.With(write, withTx).Delete("/{noticeId}", rs.remove)
	})

	return r
}

// --- Wire-Format (int64-Ids als String, wie im Frontend üblich) ---

type noticeRequest struct {
	Title                   string  `json:"title"`
	Body                    string  `json:"body"`
	Priority                string  `json:"priority"`
	ValidFrom               string  `json:"valid_from"`
	ValidUntil              *string `json:"valid_until,omitempty"`
	Weekdays                []int16 `json:"weekdays"`
	WeekPattern             int     `json:"week_pattern"`
	RequiresAcknowledgement bool    `json:"requires_acknowledgement"`
	Active                  *bool   `json:"active,omitempty"`
}

type noticeResponse struct {
	ID                      string  `json:"id"`
	Title                   string  `json:"title"`
	Body                    string  `json:"body"`
	Priority                string  `json:"priority"`
	ValidFrom               string  `json:"valid_from"`
	ValidUntil              *string `json:"valid_until,omitempty"`
	Weekdays                []int16 `json:"weekdays"`
	WeekPattern             int     `json:"week_pattern"`
	RequiresAcknowledgement bool    `json:"requires_acknowledgement"`
	Active                  bool    `json:"active"`
	AcknowledgedAt          *string `json:"acknowledged_at,omitempty"`
	AcknowledgedCount       *int    `json:"acknowledged_count,omitempty"`
}

func toNoticeResponse(view *usersModels.StaffNoticeView, includeAcknowledgedCount bool) noticeResponse {
	out := noticeResponse{
		ID:                      strconv.FormatInt(view.ID, 10),
		Title:                   view.Title,
		Body:                    view.Body,
		Priority:                view.Priority,
		ValidFrom:               view.ValidFrom.String(),
		Weekdays:                view.Weekdays,
		WeekPattern:             view.WeekPattern,
		RequiresAcknowledgement: view.RequiresAcknowledgement,
		Active:                  view.Active,
	}
	if includeAcknowledgedCount {
		count := view.AcknowledgedCount
		out.AcknowledgedCount = &count
	}
	if view.ValidUntil != nil {
		until := view.ValidUntil.String()
		out.ValidUntil = &until
	}
	if view.AcknowledgedAt != nil {
		at := view.AcknowledgedAt.Format(time.RFC3339)
		out.AcknowledgedAt = &at
	}
	if out.Weekdays == nil {
		out.Weekdays = []int16{}
	}
	return out
}

func toNoticeResponses(views []*usersModels.StaffNoticeView, includeAcknowledgedCount bool) []noticeResponse {
	out := make([]noticeResponse, 0, len(views))
	for _, view := range views {
		out = append(out, toNoticeResponse(view, includeAcknowledgedCount))
	}
	return out
}

// --- Handler ---

func (rs *StaffNoticeResource) today(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	views, err := rs.Service.Today(ctx, noticeAccountID(ctx), timezone.TodayDate())
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}
	render.JSON(w, r, map[string]any{"data": toNoticeResponses(views, false)})
}

func (rs *StaffNoticeResource) list(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	// Die Leitung sieht auch abgeschaltete Hinweise: sonst wäre ein
	// deaktivierter Hinweis unauffindbar und müsste neu getippt werden.
	views, err := rs.Service.List(ctx, noticeAccountID(ctx), true)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}
	render.JSON(w, r, map[string]any{"data": toNoticeResponses(views, true)})
}

func (rs *StaffNoticeResource) create(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	in, err := decodeNoticeInput(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	notice, err := rs.Service.Create(ctx, noticeAccountID(ctx), in)
	if err != nil {
		renderNoticeServiceError(w, r, err)
		return
	}
	render.Status(r, http.StatusCreated)
	render.JSON(w, r, map[string]any{
		"data": toNoticeResponse(&usersModels.StaffNoticeView{StaffNotice: notice}, true),
	})
}

func (rs *StaffNoticeResource) update(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, err := noticeIDFromURL(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	in, err := decodeNoticeInput(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	notice, err := rs.Service.Update(ctx, id, in)
	if err != nil {
		renderNoticeServiceError(w, r, err)
		return
	}
	render.JSON(w, r, map[string]any{
		"data": toNoticeResponse(&usersModels.StaffNoticeView{StaffNotice: notice}, true),
	})
}

func (rs *StaffNoticeResource) remove(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, err := noticeIDFromURL(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	if err := rs.Service.Delete(ctx, id); err != nil {
		renderNoticeServiceError(w, r, err)
		return
	}
	render.JSON(w, r, map[string]any{"status": "ok"})
}

func (rs *StaffNoticeResource) acknowledge(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, err := noticeIDFromURL(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	if err := rs.Service.Acknowledge(ctx, id, noticeAccountID(ctx)); err != nil {
		renderNoticeServiceError(w, r, err)
		return
	}
	render.JSON(w, r, map[string]any{"status": "ok"})
}

// --- Hilfen ---

func noticeAccountID(ctx context.Context) int64 {
	return int64(jwt.ClaimsFromCtx(ctx).ID)
}

func noticeIDFromURL(r *http.Request) (int64, error) {
	raw := chi.URLParam(r, "noticeId")
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		return 0, errors.New("invalid notice id")
	}
	return id, nil
}

func decodeNoticeInput(r *http.Request) (scheduleSvc.StaffNoticeInput, error) {
	var req noticeRequest
	if err := render.DecodeJSON(r.Body, &req); err != nil {
		return scheduleSvc.StaffNoticeInput{}, errors.New("invalid request body")
	}

	validFrom, err := timezone.ParseDate(req.ValidFrom)
	if err != nil {
		return scheduleSvc.StaffNoticeInput{}, errors.New("valid_from must be a date (YYYY-MM-DD)")
	}

	in := scheduleSvc.StaffNoticeInput{
		Title:                   req.Title,
		Body:                    req.Body,
		Priority:                req.Priority,
		ValidFrom:               validFrom,
		Weekdays:                req.Weekdays,
		WeekPattern:             req.WeekPattern,
		RequiresAcknowledgement: req.RequiresAcknowledgement,
		Active:                  true,
	}
	if req.Active != nil {
		in.Active = *req.Active
	}
	if req.ValidUntil != nil && *req.ValidUntil != "" {
		until, err := timezone.ParseDate(*req.ValidUntil)
		if err != nil {
			return scheduleSvc.StaffNoticeInput{}, errors.New("valid_until must be a date (YYYY-MM-DD)")
		}
		in.ValidUntil = &until
	}
	return in, nil
}

func renderNoticeServiceError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, scheduleSvc.ErrStaffNoticeNotFound):
		common.RenderError(w, r, common.ErrorNotFound(err))
	case errors.Is(err, scheduleSvc.ErrStaffNoticeInvalid):
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
	default:
		common.RenderError(w, r, common.ErrorInternalServer(err))
	}
}
