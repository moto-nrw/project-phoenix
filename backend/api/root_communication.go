package api

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/communication"
	communicationCompose "github.com/moto-nrw/project-phoenix/modules/communication/composition"
	parentStore "github.com/moto-nrw/project-phoenix/modules/communication/parentstore"
	staffStore "github.com/moto-nrw/project-phoenix/modules/communication/staffstore"
	deliveryModule "github.com/moto-nrw/project-phoenix/modules/delivery"
	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
	deliveryPolicy "github.com/moto-nrw/project-phoenix/modules/delivery/devices"
	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
	settingsValues "github.com/moto-nrw/project-phoenix/modules/settings"
	settingsCompose "github.com/moto-nrw/project-phoenix/modules/settings/compose"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
)

// buildParentEventEmitter builds the chat-pill emitter (#1803) that also
// wakes the guardians after enrollment and care writes that change a
// child's live care data (#1725), with the decision notices of the request
// flows (#1671).
func buildParentEventEmitter(
	db transactionruntime.Database, unit rootUnitOfWork, env RootEnvironment, transport MailTransport,
	settings settingsCompose.BatchSettingsService, messaging parentStore.MessagingRepositories, notifications DeliveryNotifications,
) *communicationCompose.ParentEventEmitter {
	return communicationCompose.NewParentEventEmitter(communicationCompose.ParentEventEmitterConfig{
		Runtime: NewCommunicationEventRuntime(db.Pool() != nil, unit), ThreadRepo: messaging.ParentMessageThread,
		MessageRepo: parentStore.NewParentMessageRepository(db.Pool()), Settings: settingsValues.NewParentEventValues(settings),
		Broadcaster: transport.Hub, Logger: env.Logger.With("service", "parent-events"),
		DecisionNotifier: deliveryCompose.NewRequestDecisionNotifier(notifications.Notifier, notifications.Preferences),
	})
}

// buildParentAnnouncements builds the staff-side parent broadcast news
// (#1669), whose cancellation notices the instance lifecycle publishes
// (#2601). File Storage hands it the attachment side at construction (#2890).
func buildParentAnnouncements(
	env RootEnvironment, settings settingsCompose.BatchSettingsService, outbox *deliveryCompose.EmailOutbox, module *deliveryModule.Module,
	notifications DeliveryNotifications, messaging parentStore.MessagingRepositories,
	attachments communication.ParentAnnouncementAttachmentPurger,
) (communication.ParentAnnouncementCapability, error) {
	return communicationCompose.NewParentAnnouncementsWithAttachments(communicationCompose.ParentAnnouncementConfig{
		Repo: messaging.ParentAnnouncement, Settings: settingsValues.NewParentAnnouncementValues(settings), Outbox: OutboxEnqueuer{Outbox: outbox},
		PushOutbox: deliveryCompose.NewDurablePushOutbox(module), Notifier: BindCommunicationNotificationSender(notifications.Notifier),
		ReminderNotifier: BindCommunicationDurableNotificationSender(notifications.Notifier), Preferences: notifications.Preferences,
		Deliveries: announcementDeliveryAdapter{module: module}, ParentsURL: env.Links.ParentsURL,
		Fingerprints: SecurityFingerprints{}, Logger: env.Logger.With("service", "announcement"),
	}, attachments, CommunicationRequestRuntime{}, deliveryPolicy.ParentDeliveryPolicy{})
}

// CommunicationMessaging are the parent messenger and the OGS-internal
// colleague chat (#2598), which shares the transport but none of its
// authorization.
type CommunicationMessaging struct {
	Messaging      communication.ParentMessagingCapability
	StaffMessaging communication.StaffMessagingRuntime
}

// buildCommunicationMessaging binds the parent messenger and the colleague
// chat over the person directory, the caller context, the settings, the
// realtime hub and the notification router.
func buildCommunicationMessaging(
	db transactionruntime.Database, env RootEnvironment, transport MailTransport, settings settingsCompose.BatchSettingsService,
	portal parentStore.MessagingRepositories, users peoplePorts.PersonDirectory, caller StaffStudentReads,
	notifications DeliveryNotifications, outbox *deliveryCompose.EmailOutbox, people PeopleRows,
	schools interface {
		FindSchoolName(context.Context, int64) (string, bool, error)
	},
) CommunicationMessaging {
	logger := env.Logger
	messaging := communicationCompose.NewParentMessaging(communicationCompose.ParentMessagingConfig{
		ThreadRepo: portal.ParentMessageThread, MessageRepo: parentStore.NewParentMessageRepository(db.Pool()),
		ReadRepo: portal.ParentMessageRead, Persons: BindParentMessagePeople(users), UserContext: caller,
		Session: CommunicationSession{}, Settings: settingsValues.NewParentMessagingValues(settings), Broadcaster: transport.Hub, DB: db.Pool(),
		Logger: logger.With("service", "messaging"), Notifier: BindCommunicationNotificationSender(notifications.Notifier), Preferences: notifications.Preferences,
		// E-Mail an den Sorgeberechtigten bei neuer OGS-Nachricht (#2307):
		// der Rueckfall fuer alle, die Push nicht eingerichtet haben.
		Outbox: OutboxEnqueuer{Outbox: outbox}, GuardianProfiles: BindParentMessageGuardianProfiles(people.GuardianProfiles), Schools: schools,
		LoginImages: settings, ParentsURL: env.Links.ParentsURL, Observe: env.Observe.Communication,
	}, CommunicationRequestRuntime{}, deliveryPolicy.ParentDeliveryPolicy{})
	staffMessaging := communicationCompose.NewStaffMessaging(communicationCompose.StaffMessagingConfig{
		ThreadRepo:  staffStore.NewStaffMessageThreadRepository(db.Pool()),
		MessageRepo: staffStore.NewStaffMessageRepository(db.Pool()), ReadRepo: portal.StaffMessageRead,
		Persons: BindStaffSenderNames(users), Session: CommunicationSession{}, Settings: settingsValues.NewStaffMessagingValues(settings), Broadcaster: transport.Hub,
		Logger: logger.With("service", "staffmessaging"), Notifier: BindCommunicationNotificationSender(notifications.Notifier),
		Preferences: notifications.Preferences, Observe: env.Observe.Communication,
	}, CommunicationRequestRuntime{}, deliveryPolicy.ParentDeliveryPolicy{})
	return CommunicationMessaging{Messaging: messaging, StaffMessaging: staffMessaging}
}
