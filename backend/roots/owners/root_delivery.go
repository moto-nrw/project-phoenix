package owners

import (
	"fmt"
	"maps"
	"slices"

	deliveryContract "github.com/moto-ogs/project-phoenix/modules/delivery/contract"
	settingsValues "github.com/moto-ogs/project-phoenix/modules/settings"

	realtime "github.com/moto-ogs/project-phoenix/realtime"

	"github.com/moto-ogs/project-phoenix/modules/carecontract"
	careContractCompose "github.com/moto-ogs/project-phoenix/modules/carecontract/compose"
	"github.com/moto-ogs/project-phoenix/modules/communication"
	communicationCompose "github.com/moto-ogs/project-phoenix/modules/communication/composition"
	parentStore "github.com/moto-ogs/project-phoenix/modules/communication/parentstore"
	deliveryCompose "github.com/moto-ogs/project-phoenix/modules/delivery/compose"
	deliveryPolicy "github.com/moto-ogs/project-phoenix/modules/delivery/devices"
	email "github.com/moto-ogs/project-phoenix/modules/delivery/mail"
	deliveryModule "github.com/moto-ogs/project-phoenix/modules/delivery/runtime"
	enrollmentCompose "github.com/moto-ogs/project-phoenix/modules/enrollment/compose"
	"github.com/moto-ogs/project-phoenix/modules/identityaccess"
	"github.com/moto-ogs/project-phoenix/modules/parentbilling"
	parentBillingCompose "github.com/moto-ogs/project-phoenix/modules/parentbilling/compose"
	calendarService "github.com/moto-ogs/project-phoenix/modules/schoolcalendar/portal"
	"github.com/moto-ogs/project-phoenix/modules/schoolmembership"
	"github.com/moto-ogs/project-phoenix/modules/securityruntime"
	settingsCompose "github.com/moto-ogs/project-phoenix/modules/settings/compose"
	transactionruntime "github.com/moto-ogs/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-ogs/project-phoenix/modules/workforce"
	"github.com/moto-ogs/project-phoenix/workflows/parentportal"
)

// DeliveryNotifications are Delivery's notification side (#1845): the
// router over the realtime and web push channels, the consent-backed
// preferences, the staff recipients, the staff notices of a parent message
// and the absence notices.
type DeliveryNotifications struct {
	Notifier            deliveryCompose.Notifier
	Preferences         deliveryCompose.NotificationPreferenceService
	StaffParentMessages deliveryCompose.StaffParentMessageNotifier
	AbsenceNotifier     deliveryCompose.AbsenceNotifier
	// UnreportedAbsences tells the Schulverwaltung about children missing
	// without a report (#4791).
	UnreportedAbsences deliveryContract.UnreportedAbsenceNotifier
}

// BuildDeliveryNotifications binds the notification router over the realtime
// and web push channels, the consent-backed preferences, the staff
// recipients and the absence notices, every one over the root's unit of work.
// Consent rows back both the notification preferences and the opt-in "Neue
// Anmeldung" mail of the intake.
func BuildDeliveryNotifications(
	db transactionruntime.Database, unit rootUnitOfWork, env RootEnvironment, pushVAPID deliveryCompose.VAPIDConfig, hub *realtime.Hub, delivery DeliveryRuntime,
	settings settingsCompose.BatchSettingsService, consent communication.NotificationConsentCapability,
	identity identityaccess.Owner, people PeopleRows, structure StructureRows, workTime workforce.Capability,
	messaging parentStore.MessagingRepositories, assignments schoolmembership.TeachingAssignmentQuery,
) (DeliveryNotifications, error) {
	logger := env.Logger
	if !pushVAPID.Configured() {
		logger.Info("web push disabled: VAPID keys not configured (VAPID_PUBLIC_KEY / VAPID_PRIVATE_KEY / VAPID_SUBSCRIBER)")
	}
	sse, err := deliveryCompose.NewTenantSSEChannel(unit, hub, db.Pool(), people.StudentGuardians, logger.With("channel", "sse"), delivery.Runtime.Store)
	if err != nil {
		return DeliveryNotifications{}, err
	}
	push, err := deliveryCompose.NewTenantDurableWebPushChannel(unit, db.Pool(), deliveryCompose.NewPushSubscriptionRepository(db.Pool()),
		pushVAPID, deliveryCompose.NewDurablePushOutbox(DeliveryModuleOf(delivery)), logger.With("channel", "web_push"))
	if err != nil {
		return DeliveryNotifications{}, err
	}
	notifier := deliveryCompose.NewNotifier(settings, logger.With("service", "notifications"), deliveryCompose.NotificationDeliveryObserver(env.Observe.Delivery), sse, push)
	preferences, err := deliveryCompose.NewTenantPreferenceService(unit, consent, DeliveryPreferenceSettings(settings), db.Pool(), identity, securityruntime.HasPermission)
	if err != nil {
		return DeliveryNotifications{}, err
	}
	recipients := deliveryCompose.NewStaffRecipientResolver(preferences, deliveryStudentGroups{students: people.Students},
		structure.Groups, people.Staff, identity, settings,
		deliveryOnDutyStaff{presence: workTime})
	schoolClasses := deliveryContract.SchoolClassRecipients{
		Reach:   deliverySchoolClassReach{students: people.Students, assignments: assignments, staff: people.Staff},
		Consent: preferences, Subscribers: consent, Accounts: identity, Permissions: securityruntime.HasPermission,
		Office: deliverySchoolOffice{staff: people.Staff},
	}
	absences, err := deliveryCompose.NewTenantAbsenceNotifier(unit, notifier, recipients, db.Pool(), logger.With("producer", "absence_notifications"), schoolClasses)
	if err != nil {
		return DeliveryNotifications{}, err
	}
	unreported, ok := absences.(deliveryContract.UnreportedAbsenceNotifier)
	if !ok {
		return DeliveryNotifications{}, fmt.Errorf("absence notifier does not send the notice about children missing without a report")
	}
	return DeliveryNotifications{
		UnreportedAbsences: unreported,
		Notifier:           notifier,
		Preferences:        preferences,
		StaffParentMessages: deliveryCompose.NewStaffParentMessageNotifier(notifier, recipients,
			messaging.ParentMessageThread, BindParentMessageLeadership(people.Staff, identity), db.Pool(), logger.With("producer", "staff_parent_message_notifications")),
		AbsenceNotifier: absences,
	}, nil
}

// BuildDeliveryWorker composes the Worker that drains Delivery's outboxes:
// every outbox kind's renderer is known at startup, so the registry is
// complete before the worker claims anything. Enrollment keeps one renderer
// per kind so subjects and templates stay independent; calendar
// appointments share one renderer for all four kinds.
func BuildDeliveryWorker(
	db transactionruntime.Database, env RootEnvironment, mailSender email.Mailer, mailFrom email.Email, pushVAPID deliveryCompose.VAPIDConfig, mailIdentity email.ReplyToResolver,
	delivery DeliveryRuntime, appointments deliveryCompose.MailRenderer, identityRenderers map[string]deliveryCompose.MailRenderer, reminders CalendarReminderSource,
	modules settingsValues.ModuleBookingReader, documents parentportal.DocumentNotices, accounts HolidayAcceptanceAccounts,
) (*deliveryModule.Worker, error) {
	subscriptions := deliveryCompose.NewPushSubscriptionRepository(db.Pool())
	tenants := deliveryCompose.NewTenantTransactions(db.Pool())
	push := deliveryCompose.NewWebPushSender(deliveryContract.WebPushConfig{
		Subscriber: pushVAPID.Subscriber, PublicKey: pushVAPID.PublicKey, PrivateKey: pushVAPID.PrivateKey,
	}, deliveryCompose.NewExpiredPushSubscriptionCleaner(tenants, subscriptions))
	return buildDeliveryWorker(db, env, mailSender, mailFrom, mailIdentity, delivery, appointments, identityRenderers, reminders, modules, documents, push, accounts)
}

func buildDeliveryWorker(
	db transactionruntime.Database, env RootEnvironment, mailSender email.Mailer, mailFrom email.Email, mailIdentity email.ReplyToResolver,
	delivery DeliveryRuntime, appointments deliveryCompose.MailRenderer, identityRenderers map[string]deliveryCompose.MailRenderer, reminders CalendarReminderSource,
	modules settingsValues.ModuleBookingReader, documents parentportal.DocumentNotices, push deliveryContract.PushSender, accounts HolidayAcceptanceAccounts,
) (*deliveryModule.Worker, error) {
	defaultFrom := mailFrom
	renderers := map[string]deliveryCompose.MailRenderer{
		communication.MailKindParentAnnouncement: deliveryCompose.MailRendererFunc(BindCommunicationMailRenderer(communicationCompose.NewParentAnnouncementRenderer(communicationCompose.ParentAnnouncementEmailConfig{
			DefaultFrom: communication.MailAddress(defaultFrom),
		}))),
		communication.MailKindParentMessage:           deliveryCompose.MailRendererFunc(BindCommunicationMailRenderer(communicationCompose.NewParentMessageRenderer(communicationCompose.ParentMessageRendererConfig{DefaultFrom: communication.MailAddress(defaultFrom)}, deliveryPolicy.ParentDeliveryPolicy{}))),
		calendarService.EmailKindAppointmentPublished: appointments,
		calendarService.EmailKindAppointmentUpdated:   appointments,
		calendarService.EmailKindAppointmentCancelled: appointments,
		calendarService.EmailKindAppointmentReminder:  appointments,
		// The confirmation letter of a Träger, mailed with its PDF (#4670).
		parentbilling.MailKindBillingLetter: parentBillingLetterMailRenderer(parentBillingCompose.NewLetterMailRenderer(), defaultFrom),
		// The care contract of a Träger, mailed with its PDF (#4678).
		carecontract.MailKindContract: careContractMailRenderer(careContractCompose.NewContractMailRenderer(), defaultFrom),
		// The confirmation of a withdrawal or termination (#4682).
		carecontract.MailKindNotice: careContractNoticeMailRenderer(careContractCompose.NewNoticeMailRenderer(), defaultFrom),
	}
	enrollmentKinds := []string{}
	for kind, renderer := range enrollmentMailRenderers(enrollmentCompose.NewMailRenderers(newHolidayAcceptanceAccounts(accounts, env.Links.ParentsURL), env.Links.ParentsURL), defaultFrom) {
		renderers[kind] = renderer
		enrollmentKinds = append(enrollmentKinds, kind)
	}
	moduleGate, err := newDeliveryModuleGate(modules, []string{
		communication.MailKindParentAnnouncement, communication.MailKindParentMessage,
		calendarService.EmailKindAppointmentPublished, calendarService.EmailKindAppointmentUpdated,
		calendarService.EmailKindAppointmentCancelled, calendarService.EmailKindAppointmentReminder,
		identityaccess.MailKindGuardianInvitation, identityaccess.MailKindGuardianWelcome,
	}, enrollmentKinds, []string{TransactionalMailKind, identityaccess.MailKindStaffWelcome, parentbilling.MailKindBillingLetter, carecontract.MailKindContract, carecontract.MailKindNotice})
	if err != nil {
		return nil, err
	}
	if err := moduleGate.requireClassified(slices.Collect(maps.Keys(renderers))); err != nil {
		return nil, err
	}
	for kind, renderer := range identityRenderers {
		renderers[kind] = renderer
	}
	pushObjects, err := NewParentPushObjectAuthorization(db)
	if err != nil {
		return nil, fmt.Errorf("initialize push object authorization: %w", err)
	}
	pushSubscriptions := deliveryCompose.NewPushSubscriptionRepository(db.Pool())
	tenants := deliveryCompose.NewTenantTransactions(db.Pool())
	worker, err := deliveryCompose.NewWorker(delivery.Runtime, &deliveryCompose.MailPushProvider{
		Registry: deliveryCompose.NewEmailTemplateRegistry(renderers), Mailer: mailSender, MailIdentity: mailIdentity,
		Push:   push,
		Logger: env.Logger.With("service", "delivery"), Tenants: tenants,
		PushAuthorized: deliveryCompose.NewPushAuthorizationChecker(tenants, pushSubscriptions),
		PushObjects:    pushObjects,
		DeferredPush:   deferredPushes(reminderRecovery(db, reminders), documents),
		Deliverable:    moduleGate.deliverable,
	})
	if err != nil {
		return nil, fmt.Errorf("initialize delivery module: %w", err)
	}
	return worker, nil
}
