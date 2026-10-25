package owners

import (
	"fmt"

	realtime "github.com/moto-nrw/project-phoenix/realtime"

	eventContract "github.com/moto-nrw/project-phoenix/modules/communication/eventcontract"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	carePlanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	"github.com/moto-nrw/project-phoenix/modules/communication"
	communicationCompose "github.com/moto-nrw/project-phoenix/modules/communication/composition"
	parentStore "github.com/moto-nrw/project-phoenix/modules/communication/parentstore"
	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	filestorageModule "github.com/moto-nrw/project-phoenix/modules/filestorage"
	filestorageCompose "github.com/moto-nrw/project-phoenix/modules/filestorage/compose"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	mealplanModule "github.com/moto-nrw/project-phoenix/modules/mealplan"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
	settingsCompose "github.com/moto-nrw/project-phoenix/modules/settings/compose"
	studentDirectoryView "github.com/moto-nrw/project-phoenix/modules/studentdirectoryview/compose"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-nrw/project-phoenix/workflows/parentportal"

	parentportalcompose "github.com/moto-nrw/project-phoenix/workflows/parentportal/compose"
)

// BuildAttachmentPurger composes File Storage's side of the announcement
// attachments over the database and the uploads object store alone, so the
// parent announcements receive it at construction.
func BuildAttachmentPurger(db transactionruntime.Database, env RootEnvironment, objects filestorageCompose.ObjectBackend) (filestorageModule.AttachmentPurger, error) {
	return filestorageCompose.NewAttachmentPurger(filestorageCompose.AttachmentPurgerDependencies{
		DB: db.Pool(), Objects: objects, Observe: env.Observe.FileStorage, Now: env.Now,
	})
}

// BuildParentPortal binds the parents portal workflow over the owners, the
// rows and the services it reads and writes through.
func BuildParentPortal(
	db transactionruntime.Database, env RootEnvironment, hub *realtime.Hub, settings settingsCompose.BatchSettingsService,
	people PeopleRows, portal carePlanCompose.ParentPortalRepositories, messaging parentStore.MessagingRepositories,
	presence PresenceOwner, carePlanRows CarePlanRows, carePlan careplan.Capability, persons peopledirectory.Capability,
	mealPlan mealplanModule.Capability, schedules CarePlanSchedules, requests CarePlanRequests, services PeopleServices,
	emitter eventContract.Emitter, notifications DeliveryNotifications, identity identityaccess.Owner,
	enrollment EnrollmentRows, notices EnrollmentNotices,
) (*parentportal.Portal, error) {
	profiles, err := carePlanCompose.NewStudentProfiles(db.Pool(), studentDirectoryView.NewReader(), func(carePlanCompose.Observation) {})
	if err != nil {
		return nil, fmt.Errorf("compose parent portal care profiles: %w", err)
	}
	messages := parentStore.NewParentMessageRepository(db.Pool())
	photos := services.StudentPhotos
	children, err := ParentChildReadsForBookedSchools(NewParentChildReads(carePlanCompose.NewParentChildQueries(portal.ParentChild, securityruntime.GuardianPermissionGranted)), settings)
	if err != nil {
		return nil, fmt.Errorf("compose parent portal children: %w", err)
	}
	return parentportalcompose.New(parentportalcompose.Dependencies{
		GuardianChanges: ParentGuardianChanges(),
		Signatures:      ParentDeclarationSignatures{Ledger: SignatureLedger()},
		DeparturePolicy: ParentDeparturePolicy(),
		GuardianPolicy:  NewParentGuardianPolicy(),
		Fingerprints:    SecurityFingerprints{},
		Declarations:    ParentDeclarationPolicy(),
		Locales:         ParentPortalLocales(),
		Runtime:         ParentPortalRuntime{},
		CareProfiles:    profiles, ChildRepo: children, EnrollablePhaseRepo: NewParentPhaseReads(carePlanCompose.NewParentPhaseQueries(portal.ParentEnrollablePhase)),
		EnrollmentSettings: settings, EnrollmentRequestRepo: NewParentEnrollmentReads(carePlanCompose.NewParentEnrollmentQueries(portal.ParentEnrollmentRequest)),
		GuardianProfileRepo: NewParentGuardianProfileReads(people.GuardianProfiles), Attendance: BindParentAttendance(presence.Presence), StatusDayRepo: carePlanRows.StatusDays,
		MealPlan: mealPlan, StudentRepo: NewParentStudentReads(people.Students), CarePlan: NewParentCarePlanCommands(carePlan, schedules.AutoExcusal), DataRequests: NewParentDataRequestCommands(carePlan), People: persons,
		Settings: BindParentSettings(settings), StudentUpdates: BindParentStudentUpdates(hub),
		PersonRepo: NewParentPersonReads(people.Persons), ChangeRequestRepo: NewParentDataRequestReads(carePlan),
		CareRequestRepo: NewParentCareRequestReads(carePlan), ExcusedRequestRepo: NewParentExcusedRequestReads(carePlanRows.ExcusedRequestRows),
		OfferingChangeRequestRepo: NewParentOfferingChangeReads(enrollmentCompose.NewOfferingChangeRecords(carePlan)),
		FamilyProtectionEvents:    BindParentFamilyProtectionReads(peopleCompose.NewFamilyProtectionEventRepository(db.Pool())),
		ParentRequestShares:       BindParentRequestShareRecords(peopleCompose.NewParentRequestShareEventRepository(db.Pool())),
		ParentRequestEvents:       NewParentRequestEvents(peopleCompose.NewParentRequestEventRepository(db.Pool())), StudentAudit: NewParentStudentAudit(services.StudentAudit),
		MessageThreadRepo: BindParentMessageThreadAccess(messaging.ParentMessageThread), MessageRepo: BindParentMessageReads(messages), MessageReadRepo: BindParentMessageInboxReads(messaging.ParentMessageRead),
		GroupThreads: BindParentGroupThreads(parentStore.NewGroupThreadPortal(db.Pool())),
		Conversations: BindParentConversationCore(communicationCompose.NewParentConversationCore(communicationCompose.ParentConversationConfig{
			ThreadRepo: messaging.ParentMessageThread, MessageRepo: messages, ReadRepo: messaging.ParentMessageRead,
			Broadcaster: hub, Logger: env.Logger.With("service", "parent"),
		})),
		ParentMessageNotifier: bindParentMessageNotifier(notifications.StaffParentMessages), ArrivalSchedules: NewParentArrivalSchedules(schedules.Arrivals),
		PickupSchedules: NewParentPickupSchedules(schedules.Pickups), CareRequests: requests.CareRequests, ExcusedRequests: NewParentExcusedAbsenceRequests(requests.Excused),
		Emitter: BindParentChildEvents(emitter), MessagingEnabled: BindParentMessagingGate(settings, env.Logger.With("service", "parent")), AnnouncementRepo: BindParentAnnouncementAccess(messaging.ParentAnnouncement),
		GuardianInvites: NewParentGuardianAccess(identity), GuardianInvitations: NewGuardianInvitationReads(identity),
		StudentGuardianRepo: NewParentStudentGuardianReads(people.StudentGuardians), GuardianPhoneRepo: NewParentGuardianPhoneReads(peopleCompose.NewGuardianPhoneNumberRepository(db.Pool())),
		StudentConsents: NewParentStudentConsents(services.StudentConsents), StudentPhotos: photos,
		AbsenceNotifier: bindParentAbsenceNotifier(notifications.AbsenceNotifier), CarePeriods: parentportal.NewCarePeriodReads(enrollment.ParentCarePeriods.CarePeriods),
		OfferingHistory: parentportal.NewOfferingHistoryReads(enrollment.ParentOfferingHistory.OfferingHistory), CareOfferingRepo: NewParentCareOfferingReads(notices.CareOfferings),
		OfferingChanges: NewParentOfferingChangeRequests(requests.OfferingChanges), Logger: env.Logger.With("service", "parent"), Now: env.Now,

		CarePlanPolicy: NewParentCarePlanPolicy(),
	}), nil
}

// BuildFileStore binds school file storage (#2596) and the attachments of
// Elternmitteilungen (#2890) through the public File Storage capability
// (#2707). Die Datei gehört der Dateiablage, der Empfängerkreis der
// Mitteilung: the announcements received the attachment side at
// construction and the store reads the announcements. The guardian audience
// stays with the parents portal, whose attachment entry reads through this
// store (#4825).
func BuildFileStore(
	db transactionruntime.Database, env RootEnvironment, objects filestorageCompose.ObjectBackend, identity identityaccess.Owner,
	persons peopledirectory.Capability, settings settingsCompose.SettingsService, audit AuditOwner,
	announcements communication.ParentAnnouncementCapability,
) (filestorageModule.Capability, error) {
	return filestorageCompose.New(filestorageCompose.Dependencies{
		DB: db.Pool(), Objects: objects, Identity: identity, People: persons,
		Settings: FileStorageSettings{Service: settings}, Events: FileStorageEvents{Repo: audit.Persistence.FileEvent},
		HasPermission: securityruntime.HasPermission, Announcements: announcements,
		Observe: env.Observe.FileStorage, Logger: env.Logger.With("service", "filestore"), Now: env.Now,
	})
}
