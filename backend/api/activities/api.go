package activities

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	activitiesSvc "github.com/moto-nrw/project-phoenix/services/activities"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
	usercontextSvc "github.com/moto-nrw/project-phoenix/services/usercontext"
	usersSvc "github.com/moto-nrw/project-phoenix/services/users"
	"github.com/uptrace/bun"
)

// Resource defines the activities API resource
type Resource struct {
	ActivityService    activitiesSvc.ActivityService
	ScheduleService    scheduleSvc.Service
	UserService        usersSvc.PersonService
	UserContextService usercontextSvc.UserContextService
	db                 *bun.DB
}

// NewResource creates a new activities resource
func NewResource(activityService activitiesSvc.ActivityService, scheduleService scheduleSvc.Service, userService usersSvc.PersonService, userContextService usercontextSvc.UserContextService, db *bun.DB) *Resource {
	return &Resource{
		ActivityService:    activityService,
		ScheduleService:    scheduleService,
		UserService:        userService,
		UserContextService: userContextService,
		db:                 db,
	}
}

// Router returns a configured router for activity endpoints
func (rs *Resource) Router() chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))

	// Protected routes that require authentication and permissions
	common.ProtectedTenantGroup(r, rs.db, func(r chi.Router, withTx common.Middleware) {

		// Basic Activity Group operations (Read) - All authenticated users can read
		r.With(withTx).Get("/", rs.listActivities)
		r.With(withTx).Get("/{id}", rs.getActivity)
		r.With(withTx).Get("/categories", rs.listCategories)

		// Category Stammdaten (#2131) — admin-only. Every activities:* write
		// permission is also held by the plain `user` role, so these use the
		// dedicated activities:manage_categories permission instead.
		r.With(authorize.RequiresPermission(permissions.ActivitiesManageCategories), withTx).
			Post("/categories", common.BindAction(
				func() *CategoryRequest { return &CategoryRequest{} },
				http.StatusCreated, rs.createCategory, categoryErrorRenderer, "Category created successfully"))
		r.With(authorize.RequiresPermission(permissions.ActivitiesManageCategories), withTx).
			Put("/categories/{categoryId}", rs.updateCategory)
		r.With(authorize.RequiresPermission(permissions.ActivitiesManageCategories), withTx).
			Delete("/categories/{categoryId}", common.IDFetch(
				categoryIDParam, msgInvalidCategoryID, rs.archiveCategory, categoryErrorRenderer, "Category archived successfully"))
		r.With(authorize.RequiresPermission(permissions.ActivitiesManageCategories), withTx).
			Post("/categories/{categoryId}/restore", common.IDFetch(
				categoryIDParam, msgInvalidCategoryID, rs.restoreCategory, categoryErrorRenderer, "Category restored successfully"))
		r.With(withTx).Get("/timespans", rs.getTimespans)

		// Basic Activity Group operations (Write) - All authenticated users can create/update/delete
		r.With(withTx).Post("/", rs.createActivity)
		r.With(withTx).Post("/quick-create", rs.quickCreateActivity)
		r.With(withTx).Put("/{id}", rs.updateActivity)
		r.With(withTx).Delete("/{id}", rs.deleteActivity)

		// Schedule Management - All authenticated users can manage schedules
		r.With(withTx).Get("/{id}/schedules", rs.getActivitySchedules)
		r.With(withTx).Get(routeScheduleByID, rs.getActivitySchedule)
		r.With(withTx).Get("/schedules/available", rs.getAvailableTimeSlots)
		r.With(withTx).Post("/{id}/schedules", rs.createActivitySchedule)
		r.With(withTx).Put(routeScheduleByID, rs.updateActivitySchedule)
		r.With(withTx).Delete(routeScheduleByID, rs.deleteActivitySchedule)

		// Supervisor Assignment - All authenticated users can manage supervisors
		r.With(withTx).Get("/{id}/supervisors", rs.getActivitySupervisors)
		r.With(withTx).Get("/supervisors/available", rs.getAvailableSupervisors)
		r.With(withTx).Post("/{id}/supervisors", rs.assignSupervisor)
		r.With(withTx).Put("/{id}/supervisors/{supervisorId}", rs.updateSupervisorRole)
		r.With(withTx).Delete("/{id}/supervisors/{supervisorId}", rs.removeSupervisor)

		// Student Enrollment - All authenticated users can manage enrollments
		r.With(withTx).Get("/{id}/students", rs.getActivityStudents)
		r.With(withTx).Get("/students/{studentId}", rs.getStudentEnrollments)
		r.With(withTx).Get("/students/{studentId}/available", rs.getAvailableActivities)
		r.With(withTx).Post("/{id}/students/{studentId}", rs.enrollStudent)
		r.With(withTx).Delete("/{id}/students/{studentId}", rs.unenrollStudent)
		r.With(withTx).Put("/{id}/students", rs.updateGroupEnrollments)
	})

	return r
}
