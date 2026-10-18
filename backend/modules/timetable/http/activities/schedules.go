package httpadapter

import (
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
)

// SchedulesResource defines the schedules API resource
type SchedulesResource struct {
	Dateframes     Dateframes
	Timeframes     Timeframes
	TimeframeGuard TimeframeChangeGuard
	transactions   Transactions
}

// NewSchedulesResource creates a new schedules resource
func NewSchedulesResource(dateframes Dateframes, timeframes Timeframes, guard TimeframeChangeGuard, transactions Transactions) *SchedulesResource {
	return &SchedulesResource{
		Dateframes:     dateframes,
		Timeframes:     timeframes,
		TimeframeGuard: guard,
		transactions:   transactions,
	}
}

// Router returns a configured router for schedule endpoints
func (rs *SchedulesResource) Router(protectedTenantRoutes func(chi.Router, func(chi.Router, common.Middleware))) chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))

	// Protected routes that require authentication and permissions
	protectedTenantRoutes(r, func(r chi.Router, withTx common.Middleware) {

		// Current dateframe endpoint - requires schedules:read permission
		r.With(common.RequiresPermission(permissions.SchedulesRead), withTx).Get("/current-dateframe", rs.getCurrentDateframe)

		// Dateframe endpoints
		r.Route("/dateframes", func(r chi.Router) {
			r.With(common.RequiresPermission(permissions.ActivitiesRead), withTx).Get("/", rs.listDateframes)
			r.With(common.RequiresPermission(permissions.ActivitiesRead), withTx).Get("/{id}", rs.getDateframe)
			r.With(common.RequiresPermission(permissions.ActivitiesCreate), withTx).Post("/", rs.createDateframe)
			r.With(common.RequiresPermission(permissions.ActivitiesUpdate), withTx).Put("/{id}", rs.updateDateframe)
			r.With(common.RequiresPermission(permissions.ActivitiesDelete), withTx).Delete("/{id}", rs.deleteDateframe)

			// Special dateframe queries
			r.With(common.RequiresPermission(permissions.ActivitiesRead), withTx).Get("/by-date", rs.getDateframesByDate)
			r.With(common.RequiresPermission(permissions.ActivitiesRead), withTx).Get("/overlapping", rs.getOverlappingDateframes)
		})

		// Timeframe endpoints
		r.Route("/timeframes", func(r chi.Router) {
			r.With(common.RequiresPermission(permissions.ActivitiesRead), withTx).Get("/", rs.listTimeframes)
			r.With(common.RequiresPermission(permissions.ActivitiesRead), withTx).Get("/{id}", rs.getTimeframe)
			r.With(common.RequiresPermission(permissions.ActivitiesCreate), withTx).Post("/", rs.createTimeframe)
			r.With(common.RequiresPermission(permissions.ActivitiesUpdate), withTx).Put("/{id}", rs.updateTimeframe)
			r.With(common.RequiresPermission(permissions.ActivitiesDelete), withTx).Delete("/{id}", rs.deleteTimeframe)

			// Special timeframe queries
			r.With(common.RequiresPermission(permissions.ActivitiesRead), withTx).Get("/active", rs.getActiveTimeframes)
			r.With(common.RequiresPermission(permissions.ActivitiesRead), withTx).Get("/by-range", rs.getTimeframesByRange)
		})

		// Recurrence rule endpoints
		r.Route("/recurrence-rules", func(r chi.Router) {
			r.With(common.RequiresPermission(permissions.ActivitiesRead), withTx).Get("/", rs.listRecurrenceRules)
			r.With(common.RequiresPermission(permissions.ActivitiesRead), withTx).Get("/{id}", rs.getRecurrenceRule)
			r.With(common.RequiresPermission(permissions.ActivitiesCreate), withTx).Post("/", rs.createRecurrenceRule)
			r.With(common.RequiresPermission(permissions.ActivitiesUpdate), withTx).Put("/{id}", rs.updateRecurrenceRule)
			r.With(common.RequiresPermission(permissions.ActivitiesDelete), withTx).Delete("/{id}", rs.deleteRecurrenceRule)

			// Special recurrence rule queries and operations
			r.With(common.RequiresPermission(permissions.ActivitiesRead), withTx).Get("/by-frequency", rs.getRecurrenceRulesByFrequency)
			r.With(common.RequiresPermission(permissions.ActivitiesRead), withTx).Get("/by-weekday", rs.getRecurrenceRulesByWeekday)
			r.With(common.RequiresPermission(permissions.ActivitiesRead), withTx).Post("/{id}/generate-events", rs.generateEvents)
		})

		// Advanced scheduling operations
		r.With(common.RequiresPermission(permissions.ActivitiesRead), withTx).Post("/check-conflict", rs.checkConflict)
		r.With(common.RequiresPermission(permissions.ActivitiesRead), withTx).Post("/find-available-slots", rs.findAvailableSlots)
	})

	return r
}

// Request and Response structures

// DateframeRequest represents a dateframe creation/update request
