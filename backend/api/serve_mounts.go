package api

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	requestFeedCompose "github.com/moto-nrw/project-phoenix/modules/careplan/requestfeed/compose"
	requestFeedHTTP "github.com/moto-nrw/project-phoenix/modules/careplan/requestfeed/http"
	communicationModule "github.com/moto-nrw/project-phoenix/modules/communication"
	announcementAPI "github.com/moto-nrw/project-phoenix/modules/communication/http/parentannouncements"
	messagingAPI "github.com/moto-nrw/project-phoenix/modules/communication/http/parentmessages"
	platformAnnouncementsHTTP "github.com/moto-nrw/project-phoenix/modules/communication/http/platformannouncements"
	sseAPI "github.com/moto-nrw/project-phoenix/modules/delivery/http/sse"
	nativeUploads "github.com/moto-nrw/project-phoenix/modules/delivery/objects"
	"github.com/moto-nrw/project-phoenix/modules/documentrendering/lists"
	"github.com/moto-nrw/project-phoenix/modules/emergencysnapshot"
	emergencyAPI "github.com/moto-nrw/project-phoenix/modules/emergencysnapshot/http"
	feedbackModule "github.com/moto-nrw/project-phoenix/modules/feedback"
	filestorageModule "github.com/moto-nrw/project-phoenix/modules/filestorage"
	filestoreAPI "github.com/moto-nrw/project-phoenix/modules/filestorage/http/files"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	meAPI "github.com/moto-nrw/project-phoenix/modules/identityaccess/inbound/me"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
	mealplanModule "github.com/moto-nrw/project-phoenix/modules/mealplan"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	birthdaysAPI "github.com/moto-nrw/project-phoenix/modules/peopledirectory/inbound/birthdays"
	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
	schoolSetupCompose "github.com/moto-nrw/project-phoenix/modules/schoolsetup/compose"
	settingsCompose "github.com/moto-nrw/project-phoenix/modules/settings/compose"
	statisticsAPI "github.com/moto-nrw/project-phoenix/modules/statistics/http"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-nrw/project-phoenix/realtime"
	owners "github.com/moto-nrw/project-phoenix/roots/owners"
	"github.com/moto-nrw/project-phoenix/workflows/parentportal"
	reminder "github.com/moto-nrw/project-phoenix/workflows/reminderdelivery"
	remindersHTTP "github.com/moto-nrw/project-phoenix/workflows/reminderdelivery/http"
	"github.com/spf13/viper"
)

// mountRequestFeed serves the change-request feed: the public feed behind its
// token and the staff's feed administration under /api.
func mountRequestFeed(root, tenant chi.Router, db transactionruntime.Database, logger *slog.Logger, frontendURL string) error {
	requestFeed, err := requestFeedCompose.New(requestFeedCompose.Dependencies{
		DB: db.Pool(), Feed: newParentRequestFeed, FrontendURL: frontendURL, Now: time.Now,
		NewToken: sessiontoken.NewOpaqueCapabilityToken, HashToken: sessiontoken.OpaqueCapabilityFingerprint,
	})
	if err != nil {
		return err
	}
	resource := requestFeedHTTP.NewResource(requestFeed, requestFeedHTTP.Runtime{
		Protected: func(router chi.Router, register func(chi.Router, requestFeedHTTP.Middleware)) {
			ProtectedTenantRoutes(router, register)
		},
		CurrentTenantID:  func(r *http.Request) int64 { return sessiontoken.ClaimsFromCtx(r.Context()).TenantID },
		CurrentAccountID: func(r *http.Request) int64 { return int64(sessiontoken.ClaimsFromCtx(r.Context()).ID) },
		Logger:           logger.With("handler", "request-feed"),
	})
	root.Mount("/public/request-feed", resource.PublicRouter())
	tenant.Mount("/students/change-requests/rss-feed", resource.TenantRouter())
	return nil
}

// mountModuleRoutes mounts the route groups modules serve themselves.
func mountModuleRoutes(tenant chi.Router, settings settingsCompose.SettingsService) error {
	schoolSetup, err := newSchoolSetupRoute(schoolSetupCompose.Dependencies{
		Settings: SchoolSetupSettings{Settings: settings},
	})
	if err != nil {
		return err
	}
	staffOnboarding, err := newStaffOnboardingRoute()
	if err != nil {
		return err
	}
	for _, module := range []moduleRoute{schoolSetup, staffOnboarding} {
		tenant.Mount(module.pattern, module.router)
	}
	return nil
}

// mountPeopleRoutes serves the people directory, guardians, statistics,
// birthdays, the caller's own context, the emergency snapshot, feedback and
// the meal plan under /api.
func mountPeopleRoutes(
	tenant chi.Router, db transactionruntime.Database, logger *slog.Logger, reports owners.PresenceReports, renderer *lists.RendererService,
	persons *peopleCompose.Directory, guardians peoplePorts.GuardianService, identity identityaccess.Owner, caller *owners.CallerRows,
	feedback feedbackModule.Capability, mealPlan mealplanModule.Capability, birthdays birthdaysAPI.Service,
	emergency emergencysnapshot.Query, settings settingsCompose.SettingsService,
) {
	tenant.Mount("/statistics", statisticsAPI.NewResource(reports.Statistics, statisticsHTTPRuntime(renderer), logger.With("handler", "statistics")).Router(ProtectedTenantRoutes))
	tenant.Mount("/guardians", newGuardiansResource(persons, NewGuardianDirectoryRuntime(guardians, renderer, identity, caller.HasCurrentStaff, peopleCompose.NewLegacyRepositoryRuntime(db.Pool())), viper.GetString("app_env"), logger.With("handler", "guardians")).Router())
	tenant.Mount("/feedback", newFeedbackResource(feedback).Router())
	tenant.Mount("/meal-plan", newMealPlanResource(mealPlan, newMealPlanExportRenderer()).Router())
	tenant.Mount("/users", newUsersResource(persons, identity.ListAccountEmails, func(ctx context.Context, tagID string) (bool, error) {
		_, _, found, err := identity.LookupRFIDCard(ctx, tagID)
		return found, err
	}).Router())
	// Birthday display and staff birthday list (#1542).
	tenant.Mount("/birthdays", birthdaysAPI.NewResource(birthdays, renderer, caller, logger.With("handler", "birthdays")).Router(ProtectedTenantRoutes))
	tenant.Mount("/me", meAPI.NewResource(caller.Caller(), caller, TenantSessionChain, nativeUploads.UploadFiles{}, settings).Router())
	tenant.Mount("/emergency", emergencyAPI.NewResource(emergency).Router(ProtectedTenantRoutes))
}

// mountCommunicationRoutes serves parent messages, announcements and surveys,
// staff reminders, the PWA usage report and the platform announcements under /api.
func mountCommunicationRoutes(
	tenant chi.Router, chat owners.CommunicationMessaging, announcements communicationModule.ParentAnnouncementCapability,
	reminders reminder.Capability, devices owners.DeliveryDevices, communication communicationModule.Capability,
) error {
	surveys, err := communicationModule.SurveysOf(announcements)
	if err != nil {
		return err
	}
	tenant.Mount("/messages", messagingAPI.NewResource(chat.Messaging, chat.GroupThreads).Router(ProtectedTenantRoutes))
	tenant.Mount("/parent-announcements", announcementAPI.NewResource(announcements, newDeclarationReports()).Router(ProtectedTenantRoutes))
	// Umfragen an Eltern oder Mitarbeitende (#4781).
	tenant.Mount("/surveys", announcementAPI.NewSurveyResource(surveys, newDeclarationReports()).Router(ProtectedTenantRoutes))
	// Visual-only staff reminders (#1457).
	tenant.Mount("/reminders", remindersHTTP.NewResource(reminders, ReminderHTTPRuntime()).Router())
	// PWA standalone-usage reporting (#2189).
	tenant.Mount("/pwa", pwaUsageRouter(devices.PWAUsage))
	// User-facing platform announcements.
	tenant.Mount("/platform", platformAnnouncementsHTTP.NewResource(platformAnnouncementsHTTP.ResourceConfig{
		AnnouncementsService: communication,
		Runtime:              newPlatformRuntime(),
	}).Router())
	return nil
}

// mountDeliveryRoutes serves the live streams and the file storage: the staff
// routes under /api and the portal routes at the root. The portal routes sit
// at the root because /parent is a catch-all mount; ParentMiddleware and
// SchoolMiddleware authenticate them.
func mountDeliveryRoutes(root, tenant chi.Router, logger *slog.Logger, fileStore filestorageModule.Capability, parent *parentportal.Portal, hub *realtime.Hub, caller *owners.CallerRows, identity identityaccess.Owner) error {
	files := filestoreAPI.NewResource(fileStore, logger.With("handler", "filestore"))
	attachmentReads, err := filestorageModule.TenantReads(fileStore)
	if err != nil {
		return err
	}
	guardianAttachments, err := owners.NewParentGuardianAttachments(parent, attachmentReads)
	if err != nil {
		return err
	}
	sse, err := sseAPI.NewResourceWithSchoolAccess(hub, caller, NewSSEHTTPRuntime(), identity, logger.With("handler", "sse"))
	if err != nil {
		return err
	}
	tenant.Mount("/files", files.Router(ProtectedTenantRoutes))
	// Anhänge an Elternmitteilungen (#2890). Eigener Pfad statt einer
	// Route unter /parent-announcements: die Bytes gehören der
	// Dateiablage, die Mitteilung steuert nur den Empfängerkreis bei.
	tenant.Mount("/announcement-attachments", files.AnnouncementAttachmentRouter(ProtectedTenantRoutes))
	tenant.Mount("/sse", NewSSEHTTPRouter(sse))
	// Parent-portal SSE stream: only whitelisted triggers (parent_message)
	// for the tenants of the guardian's children.
	root.Mount("/parent-sse", newParentSSERouter(sse))
	// Anhänge, die Eltern zu einer Mitteilung herunterladen (#2890). Der
	// Empfängerkreis der Mitteilung entscheidet im Elternportal-Workflow;
	// wer nicht dazugehört, bekommt 404 (#4825).
	root.Mount("/parent-news-attachments", files.ParentAnnouncementAttachmentRouter(ProtectedParentGroup, guardianAttachments))
	// School-portal SSE stream (#2208): account-addressed triggers only
	// (Team-Chat).
	root.Mount("/school-sse", newSchoolSSERouter(sse))
	return nil
}

// newMealPlanExportRenderer keeps the export adapter construction in the API
// composition layer so both production and route tests receive a real renderer.
func newMealPlanExportRenderer() SimpleListRenderer {
	return NewSimpleListRenderer()
}
