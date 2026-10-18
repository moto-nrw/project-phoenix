package httpadapter

import (
	"context"
	"net/http"

	activityContract "github.com/moto-nrw/project-phoenix/modules/timetable/activitycontract"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	timetableModule "github.com/moto-nrw/project-phoenix/modules/timetable"
)

// CallerContext is the slice of the Identity & Access caller context the
// activity routes read. found is false, without an error, for a caller who
// is no staff member.
type CallerContext interface {
	HasCurrentStaff(ctx context.Context) (bool, error)
	CurrentStaffID(ctx context.Context) (staffID int64, found bool, err error)
	CurrentPersonName(ctx context.Context) (firstName, lastName string, err error)
}

// SupervisorDirectory is the People Directory read port of the supervisor
// routes: teachers by specialization and the staff behind them.
type SupervisorDirectory interface {
	GetTeachersBySpecialization(ctx context.Context, specialization string) ([]*timetableModule.SupervisorTeacher, error)
	GetStaffWithPersonByIDs(ctx context.Context, ids []int64) (map[int64]*timetableModule.StaffMember, error)
	ListStaffWithPerson(ctx context.Context) ([]*timetableModule.StaffMember, error)
}

// Resource defines the activities API resource
type Resource struct {
	ActivityService activityContract.ActivityService
	// Timeframes serves the time-span reads from the Timetable owner.
	Timeframes         timetableModule.TimeframeQuery
	UserService        SupervisorDirectory
	UserContextService CallerContext
	transactions       Transactions
}

// NewResource creates a new activities resource
func NewResource(activityService activityContract.ActivityService, timeframes timetableModule.TimeframeQuery, userService SupervisorDirectory, userContextService CallerContext, transactions Transactions) *Resource {
	return &Resource{
		ActivityService:    activityService,
		Timeframes:         timeframes,
		UserService:        userService,
		UserContextService: userContextService,
		transactions:       transactions,
	}
}

// Router returns a configured router for activity endpoints
func (rs *Resource) Router(protectedTenantRoutes func(chi.Router, func(chi.Router, common.Middleware))) chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))

	// Protected routes that require authentication and permissions
	protectedTenantRoutes(r, func(r chi.Router, withTx common.Middleware) {

		// Activity reads include creator and supervisor names.
		r.With(common.RequiresAnyPermission(permissions.ActivitiesList, permissions.ActivitiesRead), withTx).Get("/", rs.listActivities)
		r.With(common.RequiresPermission(permissions.ActivitiesRead), withTx).Get("/{id}", rs.getActivity)
		r.With(withTx).Get("/categories", rs.listCategories)

		rs.mountCategories(r, withTx)
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

		// Supervisor Assignment
		r.With(common.RequiresPermission(permissions.ActivitiesRead), withTx).
			Get("/{id}/supervisors", rs.getActivitySupervisors)
		r.With(common.RequiresPermission(permissions.ActivitiesAssign), withTx).
			Get("/supervisors/available", rs.getAvailableSupervisors)
		r.With(common.RequiresPermission(permissions.ActivitiesAssign), withTx, rs.requireActivityModification).
			Post("/{id}/supervisors", rs.assignSupervisor)
		r.With(common.RequiresPermission(permissions.ActivitiesAssign), withTx, rs.requireActivityModification).
			Put("/{id}/supervisors/{supervisorId}", rs.updateSupervisorRole)
		r.With(common.RequiresPermission(permissions.ActivitiesAssign), withTx, rs.requireActivityModification).
			Delete("/{id}/supervisors/{supervisorId}", rs.removeSupervisor)

		// Student Enrollment
		r.With(common.RequiresPermission(permissions.ActivitiesRead), withTx).
			Get("/{id}/students", rs.getActivityStudents)
		r.With(common.RequiresPermission(permissions.ActivitiesRead), withTx).
			Get("/students/{studentId}", rs.getStudentEnrollments)
		r.With(common.RequiresPermission(permissions.ActivitiesRead), withTx).
			Get("/students/{studentId}/available", rs.getAvailableActivities)
		r.With(common.RequiresPermission(permissions.ActivitiesEnroll), withTx, rs.requireActivityModification).
			Post("/{id}/students/{studentId}", rs.enrollStudent)
		r.With(common.RequiresPermission(permissions.ActivitiesEnroll), withTx, rs.requireActivityModification).
			Delete("/{id}/students/{studentId}", rs.unenrollStudent)
		r.With(common.RequiresPermission(permissions.ActivitiesEnroll), withTx, rs.requireActivityModification).
			Put("/{id}/students", rs.updateGroupEnrollments)
	})

	return r
}

func (rs *Resource) mountCategories(r chi.Router, withTx common.Middleware) {
	// Category Stammdaten (#2131) — admin-only. Every activities:* write
	// permission is also held by the plain `user` role, so these use the
	// dedicated activities:manage_categories permission instead.
	r.With(common.RequiresPermission(permissions.ActivitiesManageCategories), withTx).
		Post("/categories", common.BindAction(
			func() *CategoryRequest { return &CategoryRequest{} },
			http.StatusCreated, rs.createCategory, categoryErrorRenderer, "Category created successfully"))
	r.With(common.RequiresPermission(permissions.ActivitiesManageCategories), withTx).
		Put("/categories/{categoryId}", rs.updateCategory)
	r.With(common.RequiresPermission(permissions.ActivitiesManageCategories), withTx).
		Delete("/categories/{categoryId}", common.IDFetch(
			categoryIDParam, msgInvalidCategoryID, rs.archiveCategory, categoryErrorRenderer, "Category archived successfully"))
	r.With(common.RequiresPermission(permissions.ActivitiesManageCategories), withTx).
		Post("/categories/{categoryId}/restore", common.IDFetch(
			categoryIDParam, msgInvalidCategoryID, rs.restoreCategory, categoryErrorRenderer, "Category restored successfully"))
}
