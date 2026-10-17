package api

import (
	classdayHTTP "github.com/moto-nrw/project-phoenix/modules/classday/http"
	notificationsHTTP "github.com/moto-nrw/project-phoenix/modules/delivery/http/notifications"
	"github.com/moto-nrw/project-phoenix/modules/schoolportal"
	timetableHTTP "github.com/moto-nrw/project-phoenix/modules/timetable/http"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// NewSchoolPortalHTTP binds native HTTP resources to the portal's route ports.
// Normalize optional native pointers before interface conversion so a missing
// timetable or notification surface stays unmounted.
func NewSchoolPortalHTTP(
	auth schoolportal.AuthRuntime,
	mfa schoolportal.MFARuntime,
	resets schoolportal.PasswordResetRuntime,
	classDay *classdayHTTP.Resource,
	timetable *timetableHTTP.Resource,
	staffMessaging schoolportal.StaffMessagingRouter,
	staffNotices schoolportal.StaffNoticesRouter,
	notifications *notificationsHTTP.Resource,
) *schoolportal.Resource {
	var supervision schoolportal.SupervisionsRouter
	if timetable != nil {
		supervision = timetable
	}
	var notificationRoutes schoolportal.NotificationRouter
	if notifications != nil {
		notificationRoutes = notifications
	}
	return schoolportal.NewResource(auth, mfa, resets, classDay, supervision, staffMessaging, staffNotices, notificationRoutes, tenant.WithTenantID)
}
