package studentpresencehttp

import (
	slog "log/slog"
	time "time"

	chi "github.com/go-chi/chi/v5"
	render "github.com/go-chi/render"
	common "github.com/moto-nrw/project-phoenix/api/common"
	permissions "github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	peopleModule "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	settingsFacts "github.com/moto-nrw/project-phoenix/modules/settings"
	studentpresence "github.com/moto-nrw/project-phoenix/modules/studentpresence"
	realtime "github.com/moto-nrw/project-phoenix/realtime"
)

type ResourceConfig struct {
	AbsenceNotifier         studentpresence.AbsenceNotifier
	ActiveService           StudentPresence
	Broadcaster             realtime.Broadcaster
	Logger                  *slog.Logger
	Now                     func() time.Time
	ParentEventEmitter      GuardianWake
	PeopleDirectory         peopleModule.Capability
	Persons                 PersonRecords
	PrivacyConsents         PrivacyConsentCapability
	SettingsService         TenantSettings
	StudentStatusDayService studentpresence.StatusDays
	UserContextService      CallerContext
}

type Resource struct{ ResourceConfig }

func NewResource(cfg ResourceConfig) *Resource {
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	return &Resource{ResourceConfig: cfg}
}
func (rs *Resource) Router(protected func(chi.Router, func(chi.Router, common.Middleware))) chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))
	protected(r, rs.Register)
	return r
}
func (rs *Resource) Register(r chi.Router, withTx common.Middleware) {
	r.With(common.RequiresAnyPermission(permissions.UsersUpdate, permissions.UsersAbsence), withTx).Post("/status-days/bulk", rs.bulkCreateStudentStatusDays)
	r.With(common.RequiresAnyPermission(permissions.UsersUpdate, permissions.UsersAbsence), withTx).Post("/{id}/status-days", rs.createStudentStatusDays)
	r.With(common.RequiresAnyPermission(permissions.UsersUpdate, permissions.UsersAbsence), withTx).Delete("/{id}/status-days/{statusDayId}", rs.deleteStudentStatusDay)
	r.With(common.RequiresPermission(permissions.UsersRead), withTx).Get("/{id}/privacy-consent", rs.getStudentPrivacyConsent)
	r.With(common.RequiresPermission(permissions.UsersUpdate), withTx).Put("/{id}/privacy-consent", rs.updateStudentPrivacyConsent)
	r.With(common.RequiresPermission(permissions.UsersCheckin), withTx, common.RequireWebAttendanceEnabled(settingsFacts.WebAttendanceEnabledReader(rs.SettingsService))).Post("/{id}/school-checkin", rs.schoolCheckinHandler)
	r.With(common.RequiresPermission(permissions.UsersCheckin), withTx, common.RequireWebAttendanceEnabled(settingsFacts.WebAttendanceEnabledReader(rs.SettingsService))).Post("/school-checkin/batch", rs.schoolCheckinBatchHandler)
}
