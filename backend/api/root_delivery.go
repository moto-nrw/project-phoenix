package api

import (
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/communication"
	communicationCompose "github.com/moto-nrw/project-phoenix/modules/communication/composition"
	parentStore "github.com/moto-nrw/project-phoenix/modules/communication/parentstore"
	deliveryModule "github.com/moto-nrw/project-phoenix/modules/delivery"
	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
	"github.com/moto-nrw/project-phoenix/modules/delivery/compose/email"
	deliveryPolicy "github.com/moto-nrw/project-phoenix/modules/delivery/devices"
	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	calendarService "github.com/moto-nrw/project-phoenix/modules/schoolcalendar/portal"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
	settingsCompose "github.com/moto-nrw/project-phoenix/modules/settings/compose"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	workforceCompose "github.com/moto-nrw/project-phoenix/modules/workforce/compose"
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
}

// buildDeliveryNotifications binds the notification router over the realtime
// and web push channels, the consent-backed preferences, the staff
// recipients and the absence notices, every one over the root's unit of work.
// Consent rows back both the notification preferences and the opt-in "Neue
// Anmeldung" mail of the intake.
func buildDeliveryNotifications(
	db transactionruntime.Database, unit rootUnitOfWork, env RootEnvironment, transport MailTransport, module *deliveryModule.Module,
	settings settingsCompose.BatchSettingsService, consent communication.NotificationConsentCapability,
	identity *identityaccess.Module, people PeopleRows, structure StructureRows, workTime workforce.Capability,
	threads parentStore.ParentMessageThreadRepository,
) (DeliveryNotifications, error) {
	logger := env.Logger
	if !transport.VAPID.Configured() {
		logger.Info("web push disabled: VAPID keys not configured (VAPID_PUBLIC_KEY / VAPID_PRIVATE_KEY / VAPID_SUBSCRIBER)")
	}
	sse, err := deliveryCompose.NewTenantSSEChannel(unit, transport.Hub, db.Pool(), people.StudentGuardians, logger.With("channel", "sse"))
	if err != nil {
		return DeliveryNotifications{}, err
	}
	push, err := deliveryCompose.NewTenantDurableWebPushChannel(unit, db.Pool(), deliveryCompose.NewPushSubscriptionRepository(db.Pool()),
		transport.VAPID, deliveryCompose.NewDurablePushOutbox(module), logger.With("channel", "web_push"))
	if err != nil {
		return DeliveryNotifications{}, err
	}
	notifier := deliveryCompose.NewNotifier(settings, logger.With("service", "notifications"), deliveryCompose.NotificationDeliveryObserver(env.Observe.Delivery), sse, push)
	preferences, err := deliveryCompose.NewTenantPreferenceService(unit, consent, settings, db.Pool(), identity, securityruntime.HasPermission)
	if err != nil {
		return DeliveryNotifications{}, err
	}
	recipients := deliveryCompose.NewStaffRecipientResolver(preferences, deliveryStudentGroups{students: people.Students},
		structure.Groups, people.Staff, identity, settings,
		deliveryOnDutyStaff{presence: workforceCompose.WorkPresence(workTime)})
	absences, err := deliveryCompose.NewTenantAbsenceNotifier(unit, notifier, recipients, db.Pool(), logger.With("producer", "absence_notifications"))
	if err != nil {
		return DeliveryNotifications{}, err
	}
	return DeliveryNotifications{
		Notifier:    notifier,
		Preferences: preferences,
		StaffParentMessages: deliveryCompose.NewStaffParentMessageNotifier(notifier, recipients,
			threads, db.Pool(), logger.With("producer", "staff_parent_message_notifications")),
		AbsenceNotifier: absences,
	}, nil
}

// buildDeliveryWorker composes the Worker that drains Delivery's outboxes:
// every outbox kind's renderer is known at startup, so the registry is
// complete before the worker claims anything. Enrollment keeps one renderer
// per kind so subjects and templates stay independent; calendar
// appointments share one renderer for all four kinds.
func buildDeliveryWorker(
	db transactionruntime.Database, env RootEnvironment, transport MailTransport, mailIdentity email.ReplyToResolver,
	delivery DeliveryRuntime, appointments deliveryCompose.MailRenderer, identityRenderers map[string]deliveryCompose.MailRenderer,
) (*deliveryModule.Worker, error) {
	defaultFrom := transport.DefaultFrom
	renderers := map[string]deliveryCompose.MailRenderer{
		communication.MailKindParentAnnouncement: deliveryCompose.MailRendererFunc(BindCommunicationMailRenderer(communicationCompose.NewParentAnnouncementRenderer(communicationCompose.ParentAnnouncementEmailConfig{
			DefaultFrom: communication.MailAddress(defaultFrom),
		}))),
		communication.MailKindParentMessage:           deliveryCompose.MailRendererFunc(BindCommunicationMailRenderer(communicationCompose.NewParentMessageRenderer(communicationCompose.ParentMessageRendererConfig{DefaultFrom: communication.MailAddress(defaultFrom)}, deliveryPolicy.ParentDeliveryPolicy{}))),
		calendarService.EmailKindAppointmentPublished: appointments,
		calendarService.EmailKindAppointmentUpdated:   appointments,
		calendarService.EmailKindAppointmentCancelled: appointments,
		calendarService.EmailKindAppointmentReminder:  appointments,
	}
	for kind, renderer := range enrollmentMailRenderers(enrollmentCompose.NewMailRenderers(), defaultFrom) {
		renderers[kind] = renderer
	}
	for kind, renderer := range identityRenderers {
		renderers[kind] = renderer
	}
	pushSubscriptions := deliveryCompose.NewPushSubscriptionRepository(db.Pool())
	tenants := deliveryCompose.NewTenantTransactions(db.Pool())
	vapid := transport.VAPID
	worker, err := deliveryCompose.NewWorker(delivery.Runtime, &deliveryCompose.MailPushProvider{
		Registry: deliveryCompose.NewEmailTemplateRegistry(renderers), Mailer: transport.Mailer, MailIdentity: mailIdentity,
		Push: deliveryCompose.NewWebPushSender(deliveryModule.WebPushConfig{
			Subscriber: vapid.Subscriber, PublicKey: vapid.PublicKey, PrivateKey: vapid.PrivateKey,
		}, deliveryCompose.NewExpiredPushSubscriptionCleaner(tenants, pushSubscriptions)),
		Logger: env.Logger.With("service", "delivery"), Tenants: tenants,
		PushAuthorized: deliveryCompose.NewPushAuthorizationChecker(tenants, pushSubscriptions),
	})
	if err != nil {
		return nil, fmt.Errorf("initialize delivery module: %w", err)
	}
	return worker, nil
}
