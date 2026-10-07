// Package birthdays is the HTTP adapter of the birthday display (#1542, migrated
// under #2706): it serves the dashboard birthday list, the personal opt-out
// behind it and the staff Geburtstagsliste export.
//
// It is a resource of its own rather than another route group on students or
// staff because it deliberately spans both populations and is governed by the
// school's birthday settings — mounting it under either domain would have made
// one of the two look like the owner of a rule that belongs to neither. The
// birthday facts come from the People Directory birthday capability;
// the printed list renders through the Document Rendering renderer.
package birthdays

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/modules/documentrendering/lists"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/legacy/jwt"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// Resource is the birthdays API resource.
type Resource struct {
	BirthdayService    peopledirectory.Birthdays
	ListExportService  lists.Renderer
	UserContextService common.StudentAccessSource
	logger             *slog.Logger
}

// NewResource creates the birthdays resource. The birthday settings are
// applied inside the birthday service, so the resource takes no settings
// dependency of its own.
func NewResource(
	birthdayService peopledirectory.Birthdays,
	listExportService lists.Renderer,
	userContextService common.StudentAccessSource,
	logger *slog.Logger,
) *Resource {
	return &Resource{
		BirthdayService:    birthdayService,
		ListExportService:  listExportService,
		UserContextService: userContextService,
		logger:             logger,
	}
}

// Router returns the configured router.
func (rs *Resource) Router() chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))

	common.ProtectedTenantRoutes(r, func(r chi.Router, withTx common.Middleware) {
		// users:read is the permission that already grants the child directory
		// this list is a narrow slice of, and the handler additionally applies
		// the caller's student data scope so the list never reaches past that
		// directory's own boundary. Staff entries are gated by the school
		// setting and the personal opt-out, both applied in the service, so
		// this route never widens what a colleague may see.
		r.With(common.RequiresPermission(permissions.UsersRead), withTx).Get("/", rs.getOverview)

		// The opt-out is self-service: it acts on the caller's own staff row,
		// resolved from the JWT, so it needs no permission beyond being
		// authenticated staff of this tenant.
		r.With(withTx).Get("/opt-out", rs.getOptOut)
		r.With(withTx).Put("/opt-out", rs.updateOptOut)

		// The staff Geburtstagsliste reveals full birth dates, so it is gated
		// on the permissions that already open the Stammdaten those dates come
		// from — never on users:read, which every colleague holds.
		r.With(common.RequiresAnyPermission(permissions.UsersUpdate, permissions.TimeTrackingManage), withTx).
			Post("/staff-export", rs.exportStaffBirthdays)
	})

	return r
}

type celebrationResponse struct {
	Kind        string `json:"kind"`
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	GroupName   string `json:"group_name,omitempty"`
	SchoolClass string `json:"school_class,omitempty"`
	Date        string `json:"date"`
	Age         int    `json:"age,omitempty"`
	IsToday     bool   `json:"is_today"`
}

type overviewResponse struct {
	Enabled           bool                  `json:"enabled"`
	IncludeStaff      bool                  `json:"include_staff"`
	Today             string                `json:"today"`
	WeekStart         string                `json:"week_start"`
	WeekEnd           string                `json:"week_end"`
	EarliestWeekStart string                `json:"earliest_week_start"`
	LatestWeekStart   string                `json:"latest_week_start"`
	Celebrations      []celebrationResponse `json:"celebrations"`
}

type optOutResponse struct {
	OptOut bool `json:"opt_out"`
}

type optOutRequest struct {
	OptOut *bool `json:"opt_out"`
}

func (rs *Resource) getOverview(w http.ResponseWriter, r *http.Request) {
	// The same policy every other child list uses: admin wildcard or a
	// verified staff record (#2329).
	access := common.DetermineStudentAccess(r, rs.UserContextService)

	// week_start picks the week to show (any day of it; the service moves it
	// to its Monday). Without it the current week is shown (#3777).
	var weekOf *calendar.Date
	if raw := r.URL.Query().Get("week_start"); raw != "" {
		parsed, err := calendar.ParseDate(raw)
		if err != nil {
			common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid week_start, expected YYYY-MM-DD")))
			return
		}
		weekOf = &parsed
	}

	overview, err := rs.BirthdayService.Overview(r.Context(), access, weekOf)
	if errors.Is(err, peopledirectory.ErrBirthdayWeekOutOfRange) {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("week_start is outside the allowed range")))
		return
	}
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	celebrations := make([]celebrationResponse, 0, len(overview.Celebrations))
	for _, celebration := range overview.Celebrations {
		celebrations = append(celebrations, celebrationResponse{
			Kind:        string(celebration.Kind),
			ID:          celebration.ID,
			Name:        celebration.Name,
			GroupName:   celebration.GroupName,
			SchoolClass: celebration.SchoolClass,
			Date:        celebration.Date.String(),
			Age:         celebration.Age,
			IsToday:     celebration.IsToday,
		})
	}

	common.Respond(w, r, http.StatusOK, overviewResponse{
		Enabled:           overview.Enabled,
		IncludeStaff:      overview.IncludeStaff,
		Today:             overview.Today.String(),
		WeekStart:         overview.WeekStart.String(),
		WeekEnd:           overview.WeekEnd.String(),
		EarliestWeekStart: overview.EarliestWeekStart.String(),
		LatestWeekStart:   overview.LatestWeekStart.String(),
		Celebrations:      celebrations,
	}, "Birthdays retrieved successfully")
}

func (rs *Resource) getOptOut(w http.ResponseWriter, r *http.Request) {
	accountID, ok := rs.callerAccountID(w, r)
	if !ok {
		return
	}

	optOut, err := rs.BirthdayService.GetOptOut(r.Context(), accountID)
	if err != nil {
		rs.renderServiceError(w, r, err)
		return
	}

	common.Respond(w, r, http.StatusOK, optOutResponse{OptOut: optOut}, "Birthday display preference retrieved successfully")
}

func (rs *Resource) updateOptOut(w http.ResponseWriter, r *http.Request) {
	accountID, ok := rs.callerAccountID(w, r)
	if !ok {
		return
	}

	var req optOutRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid request body")))
		return
	}
	if req.OptOut == nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("opt_out is required")))
		return
	}

	if err := rs.BirthdayService.SetOptOut(r.Context(), accountID, *req.OptOut); err != nil {
		rs.renderServiceError(w, r, err)
		return
	}

	common.Respond(w, r, http.StatusOK, optOutResponse{OptOut: *req.OptOut}, "Birthday display preference updated successfully")
}

// callerAccountID resolves the acting account from the JWT. A request without
// one cannot be answered — the opt-out has no meaning without an owner.
func (rs *Resource) callerAccountID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	accountID := jwt.ActorAccountIDFromCtx(r.Context())
	if accountID == nil {
		common.RenderError(w, r, common.ErrorUnauthorized(errors.New("no account in token")))
		return 0, false
	}
	return *accountID, true
}

func (rs *Resource) renderServiceError(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, peopledirectory.ErrStaffNotFound) {
		// The settings card hides itself on this code: the opt-out only
		// applies to accounts with a staff record (#2517).
		common.RenderError(w, r, common.ErrorNotFoundWithCode(errors.New("no staff record for this account"), common.CodeWorkforceStaffProfileMissing))
		return
	}
	common.RenderError(w, r, common.ErrorInternalServer(err))
}
