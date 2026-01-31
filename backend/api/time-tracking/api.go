package timetracking

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	activeSvc "github.com/moto-nrw/project-phoenix/services/active"
	usersSvc "github.com/moto-nrw/project-phoenix/services/users"
)

// Resource defines the time-tracking API resource
type Resource struct {
	WorkSessionService activeSvc.WorkSessionService
	PersonService      usersSvc.PersonService
}

// NewResource creates a new time-tracking resource
func NewResource(workSessionService activeSvc.WorkSessionService, personService usersSvc.PersonService) *Resource {
	return &Resource{
		WorkSessionService: workSessionService,
		PersonService:      personService,
	}
}

// Router returns a configured router for time-tracking endpoints
func (rs *Resource) Router() chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))

	// Create JWT auth instance for middleware
	tokenAuth, _ := jwt.NewTokenAuth()

	// Protected routes that require authentication and permissions
	r.Group(func(r chi.Router) {
		r.Use(tokenAuth.Verifier())
		r.Use(jwt.Authenticator)

		// All time-tracking endpoints require TimeTrackingOwn permission
		r.With(authorize.RequiresPermission(permissions.TimeTrackingOwn)).Post("/check-in", rs.checkIn)
		r.With(authorize.RequiresPermission(permissions.TimeTrackingOwn)).Post("/check-out", rs.checkOut)
		r.With(authorize.RequiresPermission(permissions.TimeTrackingOwn)).Get("/current", rs.getCurrent)
		r.With(authorize.RequiresPermission(permissions.TimeTrackingOwn)).Get("/history", rs.getHistory)
		r.With(authorize.RequiresPermission(permissions.TimeTrackingOwn)).Put("/{id}", rs.updateSession)

		// Break management
		r.With(authorize.RequiresPermission(permissions.TimeTrackingOwn)).Post("/break/start", rs.startBreak)
		r.With(authorize.RequiresPermission(permissions.TimeTrackingOwn)).Post("/break/end", rs.endBreak)
		r.With(authorize.RequiresPermission(permissions.TimeTrackingOwn)).Get("/breaks/{sessionId}", rs.getBreaks)

		// Presence map - for internal use by staff page
		r.With(authorize.RequiresPermission(permissions.UsersRead)).Get("/presence-map", rs.getPresenceMap)
	})

	return r
}

// CheckInRequest represents a check-in request
type CheckInRequest struct {
	Status string `json:"status"` // "present" or "home_office"
}

// Bind validates the check-in request
func (req *CheckInRequest) Bind(_ *http.Request) error {
	if req.Status != "present" && req.Status != "home_office" {
		return errors.New("status must be 'present' or 'home_office'")
	}
	return nil
}

// getStaffIDFromClaims resolves JWT account ID to staff ID through PersonService
func (rs *Resource) getStaffIDFromClaims(ctx context.Context, claims jwt.AppClaims) (int64, error) {
	if claims.ID == 0 {
		return 0, errors.New("invalid token")
	}

	person, err := rs.PersonService.FindByAccountID(ctx, int64(claims.ID))
	if err != nil {
		return 0, errors.New("person not found for account")
	}

	staff, err := rs.PersonService.StaffRepository().FindByPersonID(ctx, person.ID)
	if err != nil {
		return 0, errors.New("staff record not found")
	}

	return staff.ID, nil
}

// checkIn handles POST /api/time-tracking/check-in
func (rs *Resource) checkIn(w http.ResponseWriter, r *http.Request) {
	// Parse request
	req := &CheckInRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(err))
		return
	}

	// Get staff ID from JWT claims
	userClaims := jwt.ClaimsFromCtx(r.Context())
	staffID, err := rs.getStaffIDFromClaims(r.Context(), userClaims)
	if err != nil {
		common.RenderError(w, r, ErrorUnauthorized(err))
		return
	}

	// Call service to check in
	session, err := rs.WorkSessionService.CheckIn(r.Context(), staffID, req.Status)
	if err != nil {
		common.RenderError(w, r, classifyServiceError(err))
		return
	}

	common.Respond(w, r, http.StatusOK, session, "Check-in successful")
}

// checkOut handles POST /api/time-tracking/check-out
func (rs *Resource) checkOut(w http.ResponseWriter, r *http.Request) {
	// Get staff ID from JWT claims
	userClaims := jwt.ClaimsFromCtx(r.Context())
	staffID, err := rs.getStaffIDFromClaims(r.Context(), userClaims)
	if err != nil {
		common.RenderError(w, r, ErrorUnauthorized(err))
		return
	}

	// Call service to check out
	session, err := rs.WorkSessionService.CheckOut(r.Context(), staffID)
	if err != nil {
		common.RenderError(w, r, classifyServiceError(err))
		return
	}

	common.Respond(w, r, http.StatusOK, session, "Check-out successful")
}

// getCurrent handles GET /api/time-tracking/current
func (rs *Resource) getCurrent(w http.ResponseWriter, r *http.Request) {
	// Get staff ID from JWT claims
	userClaims := jwt.ClaimsFromCtx(r.Context())
	staffID, err := rs.getStaffIDFromClaims(r.Context(), userClaims)
	if err != nil {
		common.RenderError(w, r, ErrorUnauthorized(err))
		return
	}

	// Get current session
	session, err := rs.WorkSessionService.GetCurrentSession(r.Context(), staffID)
	if err != nil {
		common.RenderError(w, r, ErrorInternalServer(err))
		return
	}

	// Return null if no active session (not an error)
	common.Respond(w, r, http.StatusOK, session, "Current session retrieved successfully")
}

// getHistory handles GET /api/time-tracking/history?from=2026-01-01&to=2026-01-31
func (rs *Resource) getHistory(w http.ResponseWriter, r *http.Request) {
	// Get staff ID from JWT claims
	userClaims := jwt.ClaimsFromCtx(r.Context())
	staffID, err := rs.getStaffIDFromClaims(r.Context(), userClaims)
	if err != nil {
		common.RenderError(w, r, ErrorUnauthorized(err))
		return
	}

	// Parse date range from query params
	fromStr := r.URL.Query().Get("from")
	toStr := r.URL.Query().Get("to")

	if fromStr == "" || toStr == "" {
		common.RenderError(w, r, ErrorInvalidRequest(errors.New("from and to query parameters are required")))
		return
	}

	from, err := time.Parse(common.DateFormatISO, fromStr)
	if err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(errors.New("invalid from date format, expected YYYY-MM-DD")))
		return
	}

	to, err := time.Parse(common.DateFormatISO, toStr)
	if err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(errors.New("invalid to date format, expected YYYY-MM-DD")))
		return
	}

	// Get history
	sessions, err := rs.WorkSessionService.GetHistory(r.Context(), staffID, from, to)
	if err != nil {
		common.RenderError(w, r, ErrorInternalServer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, sessions, "Session history retrieved successfully")
}

// updateSession handles PUT /api/time-tracking/{id}
func (rs *Resource) updateSession(w http.ResponseWriter, r *http.Request) {
	// Get staff ID from JWT claims
	userClaims := jwt.ClaimsFromCtx(r.Context())
	staffID, err := rs.getStaffIDFromClaims(r.Context(), userClaims)
	if err != nil {
		common.RenderError(w, r, ErrorUnauthorized(err))
		return
	}

	// Parse session ID from URL
	idStr := chi.URLParam(r, "id")
	sessionID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(errors.New("invalid session ID")))
		return
	}

	// Parse update request
	var updates activeSvc.SessionUpdateRequest
	if err := render.DecodeJSON(r.Body, &updates); err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(err))
		return
	}

	// Call service to update session
	session, err := rs.WorkSessionService.UpdateSession(r.Context(), staffID, sessionID, updates)
	if err != nil {
		common.RenderError(w, r, classifyServiceError(err))
		return
	}

	common.Respond(w, r, http.StatusOK, session, "Session updated successfully")
}

// startBreak handles POST /api/time-tracking/break/start
func (rs *Resource) startBreak(w http.ResponseWriter, r *http.Request) {
	// Get staff ID from JWT claims
	userClaims := jwt.ClaimsFromCtx(r.Context())
	staffID, err := rs.getStaffIDFromClaims(r.Context(), userClaims)
	if err != nil {
		common.RenderError(w, r, ErrorUnauthorized(err))
		return
	}

	// Call service to start break
	brk, err := rs.WorkSessionService.StartBreak(r.Context(), staffID)
	if err != nil {
		common.RenderError(w, r, classifyServiceError(err))
		return
	}

	common.Respond(w, r, http.StatusOK, brk, "Break started")
}

// endBreak handles POST /api/time-tracking/break/end
func (rs *Resource) endBreak(w http.ResponseWriter, r *http.Request) {
	// Get staff ID from JWT claims
	userClaims := jwt.ClaimsFromCtx(r.Context())
	staffID, err := rs.getStaffIDFromClaims(r.Context(), userClaims)
	if err != nil {
		common.RenderError(w, r, ErrorUnauthorized(err))
		return
	}

	// Call service to end break
	session, err := rs.WorkSessionService.EndBreak(r.Context(), staffID)
	if err != nil {
		common.RenderError(w, r, classifyServiceError(err))
		return
	}

	common.Respond(w, r, http.StatusOK, session, "Break ended")
}

// getBreaks handles GET /api/time-tracking/breaks/{sessionId}
func (rs *Resource) getBreaks(w http.ResponseWriter, r *http.Request) {
	// Parse session ID from URL
	idStr := chi.URLParam(r, "sessionId")
	sessionID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(errors.New("invalid session ID")))
		return
	}

	// Get breaks
	breaks, err := rs.WorkSessionService.GetSessionBreaks(r.Context(), sessionID)
	if err != nil {
		common.RenderError(w, r, ErrorInternalServer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, breaks, "Breaks retrieved successfully")
}

// getPresenceMap handles GET /api/time-tracking/presence-map
func (rs *Resource) getPresenceMap(w http.ResponseWriter, r *http.Request) {
	// Get today's presence map
	presenceMap, err := rs.WorkSessionService.GetTodayPresenceMap(r.Context())
	if err != nil {
		common.RenderError(w, r, ErrorInternalServer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, presenceMap, "Presence map retrieved successfully")
}
