// Package careplanhttp serves Care Plan's operator review before a school
// switches to booking-led care (#4072): which children would be left without
// a care day and which completions the switch would plan. The operator router
// mounts it below the school's settings; the root lends it the operator
// surface's response envelope and error bodies and the tenant transaction of
// the school in the path, so the wire format stays unchanged.
package careplanhttp

import (
	"context"
	"errors"
	"net/http"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// BookingAuthority is the Care Plan capability the review reads.
type BookingAuthority interface {
	PreviewBookingAuthorityImpact(ctx context.Context, on calendar.Date) (*careplan.BookingAuthorityImpact, error)
}

// Surface is what the operator router lends the review.
type Surface struct {
	InvalidRequest func(err error) render.Renderer
	Internal       func(message string, cause error) render.Renderer
	RenderError    func(w http.ResponseWriter, r *http.Request, renderer render.Renderer)
	Respond        func(w http.ResponseWriter, r *http.Request, status int, data any, message string)
}

// BookingAuthorityImpactConfig holds the review's dependencies. All are
// required.
type BookingAuthorityImpactConfig struct {
	Authority BookingAuthority
	// WithinSchool runs fn in the tenant transaction of the school.
	WithinSchool func(ctx context.Context, schoolID int64, fn func(context.Context) error) error
	Surface      Surface
}

type bookingAuthorityImpact struct {
	authority    BookingAuthority
	withinSchool func(ctx context.Context, schoolID int64, fn func(context.Context) error) error
	surface      Surface
}

// NewBookingAuthorityImpact returns the GET handler of
// /schools/{id}/settings/booking-authority-impact. The preview is advisory:
// the write path evaluates the impact again under the booking-write lock, so
// a stale preview cannot bypass the guard.
func NewBookingAuthorityImpact(cfg BookingAuthorityImpactConfig) http.Handler {
	surface := cfg.Surface
	if cfg.Authority == nil || cfg.WithinSchool == nil || surface.InvalidRequest == nil ||
		surface.Internal == nil || surface.RenderError == nil || surface.Respond == nil {
		panic("booking authority impact: all dependencies are required")
	}
	return &bookingAuthorityImpact{authority: cfg.Authority, withinSchool: cfg.WithinSchool, surface: surface}
}

func (h *bookingAuthorityImpact) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	schoolID, err := common.ParseIDParam(r, "id")
	if err != nil {
		h.surface.RenderError(w, r, h.surface.InvalidRequest(errors.New("invalid school ID")))
		return
	}
	var impact *careplan.BookingAuthorityImpact
	err = h.withinSchool(r.Context(), schoolID, func(ctx context.Context) error {
		var previewErr error
		impact, previewErr = h.authority.PreviewBookingAuthorityImpact(ctx, calendar.TodayDate())
		return previewErr
	})
	if err != nil {
		h.surface.RenderError(w, r, h.surface.Internal("Failed to review booking authority impact", err))
		return
	}
	h.surface.Respond(w, r, http.StatusOK, impact, "Booking authority impact retrieved successfully")
}
