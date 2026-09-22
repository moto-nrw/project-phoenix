package httpadapter

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
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
)

// Tagesinformationen (#2180) — HTTP-Fläche der Hinweise fürs Team:
// interne Hinweise der Leitung an das Team. Eingehängt unter
// /api/staff-notices (OGS-Portal) und /school/staff-notices (moto schule).
//
// Zwei Rechte, zwei Zwecke: LESEN darf im OGS-Portal jede Person mit
// users:read (das hält jede Betreuungskraft), in moto schule jede mit
// staff_notices:read (das hält die Lehrkraft-Rolle, ohne dass users:read und
// damit das Kinderverzeichnis mit aufgeht); SCHREIBEN nur ein Admin, nur im
// OGS-Portal. Der Zuschnitt steht bewusst hier an der Route und nicht nur im
// Frontend — ein Baustein der Startseite, den man ohne Recht trotzdem abrufen
// könnte, wäre kein Zuschnitt, sondern eine Kulisse.
//
// Die Zielgruppe (#2208) entscheidet die Route, nicht die Person: wer über
// /api liest, liest als Betreuung, wer über /school liest, als Lehrkraft. Ein
// Konto mit beiden Rollen sieht in jedem Portal den Anteil, der dorthin gehört.

// StaffNoticeResource ist die HTTP-Ressource der Tagesinformationen.
type StaffNoticeResource struct {
	Service   timetable.StaffNotices
	accountID func(context.Context) int64
	db        *bun.DB
}

// NewStaffNoticeResource verdrahtet die Ressource. accountID löst das handelnde
// Konto der Anfrage auf (die Session-Claims); die Wurzel reicht dafür eine
// Funktion über ihre Identitäts-Middleware herein.
func NewStaffNoticeResource(service timetable.StaffNotices, accountID func(context.Context) int64, db *bun.DB) *StaffNoticeResource {
	if service == nil || accountID == nil || db == nil {
		panic("staff notice resource: service, identity and db are required")
	}
	return &StaffNoticeResource{Service: service, accountID: accountID, db: db}
}

// Router liefert den Router unter /api/staff-notices: die Sicht der Betreuung
// plus die Verwaltung der Leitung.
func (rs *StaffNoticeResource) Router() chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))

	common.ProtectedTenantGroup(r, rs.db, func(r chi.Router, withTx common.Middleware) {
		read := common.RequiresPermission(permissions.UsersRead)
		// Schreiben ist adminexklusiv: die Zielgruppe wählt die Leitung beim
		// Anlegen, sie ist kein Recht, das jemand anderes halten könnte.
		write := common.RequiresPermission(permissions.AdminWildcard)

		r.With(read, withTx).Get("/today", rs.todayFor(timetable.StaffNoticeAudienceStaff))
		r.With(read, withTx).Post("/{noticeId}/acknowledge", rs.acknowledgeFor(timetable.StaffNoticeAudienceStaff))

		r.With(write, withTx).Get("/", rs.list)
		r.With(write, withTx).Post("/", rs.create)
		r.With(write, withTx).Put("/{noticeId}", rs.update)
		r.With(write, withTx).Delete("/{noticeId}", rs.remove)
		r.With(write, withTx).Get("/{noticeId}/acknowledgements", rs.acknowledgements)
	})

	return r
}

// SchoolRouter liefert den Router unter /school/staff-notices (#2208): die
// Sicht einer Lehrkraft in moto schule — lesen und zur Kenntnis nehmen, mehr
// nicht. Gleiche Handler, andere Leserart, eigenes Recht.
func (rs *StaffNoticeResource) SchoolRouter() chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))

	common.ProtectedSchoolGroup(r, rs.db, func(r chi.Router, withTx common.Middleware) {
		read := common.RequiresPermission(permissions.StaffNoticesRead)

		r.With(read, withTx).Get("/today", rs.todayFor(timetable.StaffNoticeAudienceLehrkraft))
		r.With(read, withTx).Post("/{noticeId}/acknowledge", rs.acknowledgeFor(timetable.StaffNoticeAudienceLehrkraft))
	})

	return r
}

// --- Wire-Format (int64-Ids als String, wie im Frontend üblich) ---

type noticeRequest struct {
	Title    string `json:"title"`
	Body     string `json:"body"`
	Priority string `json:"priority"`
	// Audience: "all" | "staff" | "lehrkraft" (#2208); leer = "all".
	Audience                string  `json:"audience"`
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
	Audience                string  `json:"audience"`
	ValidFrom               string  `json:"valid_from"`
	ValidUntil              *string `json:"valid_until,omitempty"`
	Weekdays                []int16 `json:"weekdays"`
	WeekPattern             int     `json:"week_pattern"`
	RequiresAcknowledgement bool    `json:"requires_acknowledgement"`
	Active                  bool    `json:"active"`
	AcknowledgedAt          *string `json:"acknowledged_at,omitempty"`
	AcknowledgedCount       *int    `json:"acknowledged_count,omitempty"`
}

// noticeFields is the wire-relevant projection of a Tagesinformation plus
// the per-account acknowledgement extras. ValidFrom and ValidUntil are
// calendar days as YYYY-MM-DD; an empty ValidUntil means unbefristet and is
// left out of the response.
type noticeFields struct {
	ID                      int64
	Title                   string
	Body                    string
	Priority                string
	Audience                string
	ValidFrom               string
	ValidUntil              string
	Weekdays                []int16
	WeekPattern             int
	RequiresAcknowledgement bool
	Active                  bool
	AcknowledgedAt          *time.Time
	AcknowledgedCount       int
}

// noticeFieldsOf projects a notice of the public contract onto the wire
// fields; the acknowledgement extras are added by the caller that has them.
func noticeFieldsOf(notice timetable.StaffNotice) noticeFields {
	return noticeFields{
		ID:                      notice.ID,
		Title:                   notice.Title,
		Body:                    notice.Body,
		Priority:                notice.Priority,
		Audience:                notice.Audience,
		ValidFrom:               notice.ValidFrom,
		ValidUntil:              notice.ValidUntil,
		Weekdays:                notice.Weekdays,
		WeekPattern:             notice.WeekPattern,
		RequiresAcknowledgement: notice.RequiresAcknowledgement,
		Active:                  notice.Active,
	}
}

// noticeViewFields adds the reader's own acknowledgement and the leadership
// count to the projection.
func noticeViewFields(view timetable.StaffNoticeView) noticeFields {
	fields := noticeFieldsOf(view.StaffNotice)
	fields.AcknowledgedAt = view.AcknowledgedAt
	fields.AcknowledgedCount = view.AcknowledgedCount
	return fields
}

func toNoticeResponse(view noticeFields, includeAcknowledgedCount bool) noticeResponse {
	out := noticeResponse{
		ID:                      strconv.FormatInt(view.ID, 10),
		Title:                   view.Title,
		Body:                    view.Body,
		Priority:                view.Priority,
		Audience:                view.Audience,
		ValidFrom:               view.ValidFrom,
		Weekdays:                view.Weekdays,
		WeekPattern:             view.WeekPattern,
		RequiresAcknowledgement: view.RequiresAcknowledgement,
		Active:                  view.Active,
	}
	if includeAcknowledgedCount {
		count := view.AcknowledgedCount
		out.AcknowledgedCount = &count
	}
	if view.ValidUntil != "" {
		until := view.ValidUntil
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

// --- Handler ---

// todayFor liefert den Handler der heutigen Hinweise für eine Leserart. Die
// Leserart ist an die Route gebunden, nicht an die Anfrage: keine Person kann
// sich per Parameter in ein anderes Portal lesen.
func (rs *StaffNoticeResource) todayFor(reader string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		views, err := rs.Service.StaffNoticesOn(ctx, rs.noticeAccountID(ctx), timezone.TodayDate().String(), reader)
		if err != nil {
			common.RenderError(w, r, common.ErrorInternalServer(err))
			return
		}
		out := make([]noticeResponse, 0, len(views))
		for _, view := range views {
			out = append(out, toNoticeResponse(noticeViewFields(view), false))
		}
		render.JSON(w, r, map[string]any{"data": out})
	}
}

func (rs *StaffNoticeResource) list(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	// Die Leitung sieht auch abgeschaltete Hinweise: sonst wäre ein
	// deaktivierter Hinweis unauffindbar und müsste neu getippt werden.
	views, err := rs.Service.ListStaffNotices(ctx, rs.noticeAccountID(ctx), true)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}
	out := make([]noticeResponse, 0, len(views))
	for _, view := range views {
		out = append(out, toNoticeResponse(noticeViewFields(view), true))
	}
	render.JSON(w, r, map[string]any{"data": out})
}

func (rs *StaffNoticeResource) create(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	in, err := decodeNoticeInput(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	notice, err := rs.Service.CreateStaffNotice(ctx, rs.noticeAccountID(ctx), in)
	if err != nil {
		renderNoticeServiceError(w, r, err)
		return
	}
	render.Status(r, http.StatusCreated)
	render.JSON(w, r, map[string]any{"data": toNoticeResponse(noticeFieldsOf(notice), true)})
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
	notice, err := rs.Service.UpdateStaffNotice(ctx, id, in)
	if err != nil {
		renderNoticeServiceError(w, r, err)
		return
	}
	render.JSON(w, r, map[string]any{"data": toNoticeResponse(noticeFieldsOf(notice), true)})
}

func (rs *StaffNoticeResource) remove(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, err := noticeIDFromURL(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	if err := rs.Service.DeleteStaffNotice(ctx, id); err != nil {
		renderNoticeServiceError(w, r, err)
		return
	}
	render.JSON(w, r, map[string]any{"status": "ok"})
}

// acknowledgeFor liefert den Kenntnisnahme-Handler für eine Leserart; ein
// Hinweis, der dieses Portal nicht erreicht, ist dort auch nicht bestätigbar.
func (rs *StaffNoticeResource) acknowledgeFor(reader string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		id, err := noticeIDFromURL(r)
		if err != nil {
			common.RenderError(w, r, common.ErrorInvalidRequest(err))
			return
		}
		if err := rs.Service.AcknowledgeStaffNotice(ctx, id, rs.noticeAccountID(ctx), reader); err != nil {
			renderNoticeServiceError(w, r, err)
			return
		}
		render.JSON(w, r, map[string]any{"status": "ok"})
	}
}

// acknowledgerResponse ist eine Zeile der Bestätigungsliste (#2208).
type acknowledgerResponse struct {
	AccountID      string `json:"account_id"`
	Name           string `json:"name"`
	AcknowledgedAt string `json:"acknowledged_at"`
}

// acknowledgements gibt der Leitung, wer den Hinweis bestätigt hat — Namen,
// nicht nur die Zahl. Adminexklusiv wie die Verwaltung selbst.
func (rs *StaffNoticeResource) acknowledgements(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, err := noticeIDFromURL(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	rows, err := rs.Service.StaffNoticeAcknowledgers(ctx, id)
	if err != nil {
		renderNoticeServiceError(w, r, err)
		return
	}
	out := make([]acknowledgerResponse, 0, len(rows))
	for _, row := range rows {
		out = append(out, acknowledgerResponse{
			AccountID:      strconv.FormatInt(row.AccountID, 10),
			Name:           row.Name,
			AcknowledgedAt: row.AcknowledgedAt.Format(time.RFC3339),
		})
	}
	render.JSON(w, r, map[string]any{"data": out})
}

// --- Hilfen ---

func (rs *StaffNoticeResource) noticeAccountID(ctx context.Context) int64 {
	return rs.accountID(ctx)
}

func noticeIDFromURL(r *http.Request) (int64, error) {
	raw := chi.URLParam(r, "noticeId")
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		return 0, errors.New("invalid notice id")
	}
	return id, nil
}

func decodeNoticeInput(r *http.Request) (timetable.StaffNoticeInput, error) {
	var req noticeRequest
	if err := render.DecodeJSON(r.Body, &req); err != nil {
		return timetable.StaffNoticeInput{}, errors.New("invalid request body")
	}

	validFrom, err := timezone.ParseDate(req.ValidFrom)
	if err != nil {
		return timetable.StaffNoticeInput{}, errors.New("valid_from must be a date (YYYY-MM-DD)")
	}

	in := timetable.StaffNoticeInput{
		Title:                   req.Title,
		Body:                    req.Body,
		Priority:                req.Priority,
		Audience:                req.Audience,
		ValidFrom:               validFrom.String(),
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
			return timetable.StaffNoticeInput{}, errors.New("valid_until must be a date (YYYY-MM-DD)")
		}
		in.ValidUntil = until.String()
	}
	return in, nil
}

func renderNoticeServiceError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, timetable.ErrStaffNoticeNotFound):
		common.RenderError(w, r, common.ErrorNotFound(err))
	case errors.Is(err, timetable.ErrStaffNoticeInvalid):
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
	default:
		common.RenderError(w, r, common.ErrorInternalServer(err))
	}
}
