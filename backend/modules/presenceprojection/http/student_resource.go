package presenceprojectionhttp

import (
	slog "log/slog"
	time "time"

	chi "github.com/go-chi/chi/v5"
	common "github.com/moto-nrw/project-phoenix/api/common"
	permissions "github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	careplan "github.com/moto-nrw/project-phoenix/modules/careplan"
	excusedrequests "github.com/moto-nrw/project-phoenix/modules/careplan/excusedrequests"
	lists "github.com/moto-nrw/project-phoenix/modules/documentrendering/lists"
	peopleModule "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	studentpresence "github.com/moto-nrw/project-phoenix/modules/studentpresence"
)

type ResourceConfig struct {
	AbsenceOverview         studentpresence.StatusDayOverviews
	ActiveEnrollments       ActiveEnrollments
	ActiveService           StudentPresence
	ArrivalScheduleService  careplan.ArrivalScheduleService
	CareDayService          careplan.CareDayQuery
	CareLifecycleService    careplan.CareLifecycle
	ClassListEntries        ClassListEntryReader
	CompanionService        careplan.StudentCompanions
	ExcusedRequestService   excusedrequests.Service
	InstanceService         PlannedStudents
	ListExportService       lists.Renderer
	Logger                  *slog.Logger
	Now                     func() time.Time
	OfferingDayRoster       careplan.OfferingDayRoster
	PeopleDirectory         peopleModule.Capability
	Persons                 PersonRecords
	PickupScheduleService   careplan.PickupScheduleService
	SchoolGroups            SchoolGroups
	SchoolService           SchoolDirectory
	SettingsService         TenantSettings
	StudentConsents         StudentConsentCapability
	StudentHistoryService   studentpresence.StudentHistory
	StudentStatusDayService studentpresence.StatusDays
	UserContextService      CallerContext
	CompareSchoolClasses    func(string, string) int
	CompareGermanNames      func(string, string, string, string) int
}

type Resource struct{ ResourceConfig }

func NewResource(cfg ResourceConfig) *Resource {
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	return &Resource{ResourceConfig: cfg}
}

func (rs *Resource) Register(r chi.Router, withTx common.Middleware) {
	if rs.SettingsService == nil {
		panic("presenceprojection HTTP: tenant settings are required")
	}
	// The child list and detail reads also open for students:read, the
	// view-only child access of the role guest (ADR 0090). Exports, the day
	// log, histories and every write stay behind users:read and above.
	childRead := common.RequiresAnyPermission(permissions.UsersRead, permissions.StudentsRead)
	r.With(childRead, withTx).Get("/", rs.listStudents)
	r.With(common.RequiresPermission(permissions.UsersRead), withTx).Post("/export", rs.exportStudents)
	r.With(common.RequiresPermission(permissions.UsersRead), withTx).Get("/care-offerings", rs.listCareOfferingOptions)
	r.With(childRead, withTx).Get("/{id}", rs.getStudent)
	r.With(childRead, withTx).Get("/{id}/current-visit", rs.getStudentCurrentVisit)
	r.With(common.RequiresPermission(permissions.UsersRead), withTx).Get("/{id}/visit-history", rs.getStudentVisitHistory)
	r.With(common.RequiresPermission(permissions.UsersRead), withTx).Get("/day-log", rs.getStudentsDayLog)
	r.With(common.RequiresPermission(permissions.UsersRead), withTx).Get("/day-log/export", rs.exportStudentsDayLog)
	r.With(common.RequiresPermission(permissions.UsersRead), withTx).Get("/{id}/attendance-history", rs.getStudentAttendanceHistory)
	r.With(common.RequiresPermission(permissions.UsersRead), withTx).Get("/{id}/attendance-history/export", rs.exportStudentAttendanceHistory)
	r.With(common.RequiresPermission(permissions.UsersRead), withTx).Get("/{id}/status-days", rs.getStudentStatusDays)
	r.With(childRead, withTx).Get("/status-days", rs.getStudentStatusDaysOverview)
}
