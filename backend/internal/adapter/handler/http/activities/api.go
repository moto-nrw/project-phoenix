package activities

import (
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/internal/adapter/middleware/jwt"
	activitiesSvc "github.com/moto-nrw/project-phoenix/internal/core/service/activities"
	scheduleSvc "github.com/moto-nrw/project-phoenix/internal/core/service/schedule"
	usercontextSvc "github.com/moto-nrw/project-phoenix/internal/core/service/usercontext"
	usersSvc "github.com/moto-nrw/project-phoenix/internal/core/service/users"
)

const (
	routeScheduleByID         = "/{id}/schedules/{scheduleId}"
	msgActivityCreatedSuccess = "Activity created successfully"
	msgActivityUpdatedSuccess = "Activity updated successfully"
)

// Resource defines the activities API resource
// It aggregates all dependencies required for activity endpoints.
type Resource struct {
	ActivityService    activitiesSvc.ActivityService
	ScheduleService    scheduleSvc.Service
	UserService        usersSvc.PersonService
	UserContextService usercontextSvc.UserContextService
}

// NewResource creates a new activities resource
func NewResource(activityService activitiesSvc.ActivityService, scheduleService scheduleSvc.Service, userService usersSvc.PersonService, userContextService usercontextSvc.UserContextService) *Resource {
	return &Resource{
		ActivityService:    activityService,
		ScheduleService:    scheduleService,
		UserService:        userService,
		UserContextService: userContextService,
	}
}

// Router returns a configured router for activity endpoints
func (rs *Resource) Router() chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))

	// Create JWT auth instance for middleware
	tokenAuth := jwt.MustTokenAuth()

	// Protected routes that require authentication and permissions
	r.Group(func(r chi.Router) {
		r.Use(tokenAuth.Verifier())
		r.Use(jwt.Authenticator)

		// Basic Activity Group operations (Read) - All authenticated users can read
		r.Get("/", rs.listActivities)
		r.Get("/{id}", rs.getActivity)
		r.Get("/categories", rs.listCategories)
		r.Get("/timespans", rs.getTimespans)

		// Basic Activity Group operations (Write) - All authenticated users can create/update/delete
		r.Post("/", rs.createActivity)
		r.Post("/quick-create", rs.quickCreateActivity)
		r.Put("/{id}", rs.updateActivity)
		r.Delete("/{id}", rs.deleteActivity)

		// Schedule Management - All authenticated users can manage schedules
		r.Get("/{id}/schedules", rs.getActivitySchedules)
		r.Get(routeScheduleByID, rs.getActivitySchedule)
		r.Get("/schedules/available", rs.getAvailableTimeSlots)
		r.Post("/{id}/schedules", rs.createActivitySchedule)
		r.Put(routeScheduleByID, rs.updateActivitySchedule)
		r.Delete(routeScheduleByID, rs.deleteActivitySchedule)

		// Supervisor Assignment - All authenticated users can manage supervisors
		r.Get("/{id}/supervisors", rs.getActivitySupervisors)
		r.Get("/supervisors/available", rs.getAvailableSupervisors)
		r.Post("/{id}/supervisors", rs.assignSupervisor)
		r.Put("/{id}/supervisors/{supervisorId}", rs.updateSupervisorRole)
		r.Delete("/{id}/supervisors/{supervisorId}", rs.removeSupervisor)

		// Student Enrollment - All authenticated users can manage enrollments
		r.Get("/{id}/students", rs.getActivityStudents)
		r.Get("/students/{studentId}", rs.getStudentEnrollments)
		r.Get("/students/{studentId}/available", rs.getAvailableActivities)
		r.Post("/{id}/students/{studentId}", rs.enrollStudent)
		r.Delete("/{id}/students/{studentId}", rs.unenrollStudent)
		r.Put("/{id}/students", rs.updateGroupEnrollments)
	})

	return r
}
