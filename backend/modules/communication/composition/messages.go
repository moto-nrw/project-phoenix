package compose

import (
	"context"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/modules/communication"
	"github.com/moto-nrw/project-phoenix/modules/communication/internal/adapters/attachmentpostgres"
	"github.com/moto-nrw/project-phoenix/modules/communication/internal/adapters/parentpostgres"
	communicationPostgres "github.com/moto-nrw/project-phoenix/modules/communication/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/communication/internal/adapters/staffpostgres"
	deliveryPorts "github.com/moto-nrw/project-phoenix/modules/communication/internal/application"
	parentMessages "github.com/moto-nrw/project-phoenix/modules/communication/internal/parentmessages"
	staffMessages "github.com/moto-nrw/project-phoenix/modules/communication/internal/staffmessages"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/uptrace/bun"
)

type ParentMessagingConfig struct {
	ThreadRepo  deliveryPorts.ParentMessageThreadRepositorySource
	MessageRepo deliveryPorts.ParentMessageRepositorySource
	ReadRepo    deliveryPorts.ParentMessageReadRepositorySource
	Persons     parentMessages.PersonLookup
	// UserContext is the caller: the student read rules gate the read scope,
	// and its groups serve the "own groups" count scope (#3673).
	UserContext      parentMessages.Caller
	Session          deliveryPorts.RequestSession
	Settings         parentMessages.Settings
	Broadcaster      realtime.Broadcaster
	DB               *bun.DB
	Logger           *slog.Logger
	Notifier         func(context.Context, communication.NotificationEvent) error
	Preferences      deliveryPorts.NotificationPreferences
	Outbox           deliveryPorts.MailEnqueuer
	GuardianProfiles parentMessages.GuardianProfileFinder
	Schools          parentMessages.SchoolFinder
	LoginImages      parentMessages.LoginImageResolver
	ParentsURL       string
	Observe          func(Observation)
	GroupAudience    communication.ParentGroupAudienceReads
}

// NewParentMessaging also returns the OGS side of the group threads (#4784),
// the photos and files of both (#6685) and the team confirmation (#6486).
func NewParentMessaging(cfg ParentMessagingConfig, runtime parentMessages.Runtime, policy deliveryPorts.ParentDeliveryPolicy) (communication.ParentMessagingCapability, communication.ParentGroupThreadCapability, communication.MessageAttachmentCapability, communication.ParentMessageTeamHandling) {
	service := parentMessages.NewService(parentMessages.Config{
		Attachments: messageAttachmentStore(cfg.DB),
		ThreadRepo:  parentMessages.NewThreadRepository(cfg.ThreadRepo), MessageRepo: parentMessages.NewMessageRepository(cfg.MessageRepo), ReadRepo: parentMessages.NewReadRepository(cfg.ReadRepo),
		Persons: cfg.Persons, UserContext: cfg.UserContext, Session: cfg.Session, Settings: cfg.Settings,
		Broadcaster: cfg.Broadcaster, Logger: cfg.Logger, Notifier: deliveryPorts.NewNotificationSender(cfg.Notifier), Preferences: cfg.Preferences, Outbox: cfg.Outbox,
		GuardianProfiles: cfg.GuardianProfiles, Schools: cfg.Schools, LoginImages: cfg.LoginImages, ParentsURL: cfg.ParentsURL,
		CountPreferences: newCountPreferences(cfg.DB), Groups: cfg.UserContext,
		GroupThreads: parentpostgres.NewGroupThreadStore(parentDatabase(cfg.DB)), GroupAudience: cfg.GroupAudience, Leaders: cfg.Persons,
	}, runtime, policy)
	return deliveryPorts.NewParentMessagingFacade(service, parentMessages.NativeFacadeRows(), parentMessages.NativeFacadeErrors(), communicationPostgres.NewMessageObservation(cfg.Observe)), service, service, service
}

// messageAttachmentStore is the attachment store over the ambient tenant
// transaction; without a database its calls fail and uploads are refused.
func messageAttachmentStore(db *bun.DB) *attachmentpostgres.Store {
	return attachmentpostgres.New(attachmentpostgres.Database(staffpostgres.NewAmbientDatabase(db)))
}

// NewPortalMessageAttachments is the guardian's side of the photos and files
// of messages (#6685).
func NewPortalMessageAttachments(db *bun.DB) communication.PortalMessageAttachments {
	return deliveryPorts.PortalMessageAttachments{Store: messageAttachmentStore(db)}
}

// NewMessageAttachmentCleanup removes the objects of deleted attachments.
func NewMessageAttachmentCleanup(db *bun.DB, logger *slog.Logger) communication.MessageAttachmentCleanup {
	return deliveryPorts.MessageAttachmentSweep{Store: messageAttachmentStore(db), Logger: logger}
}

// newCountPreferences builds the store behind the personal count scope
// (#3673) over users.parent_message_count_preferences.
func newCountPreferences(db *bun.DB) *parentpostgres.CountPreferenceStore {
	return parentpostgres.NewCountPreferenceStore(parentDatabase(db))
}

type ParentMessageRendererConfig communication.MailRendererConfig

func NewParentMessageRenderer(cfg ParentMessageRendererConfig, policy deliveryPorts.LocaleNormalizer) communication.MailRenderer {
	return communication.MailRenderer(parentMessages.NewParentMessageRenderer(parentMessages.ParentMessageRendererConfig(cfg), policy))
}

type StaffMessagingConfig struct {
	ThreadRepo  deliveryPorts.StaffConversationSource
	MessageRepo deliveryPorts.StaffMessageSource
	ReadRepo    deliveryPorts.StaffReadSource
	Persons     staffMessages.PersonLookup
	Session     deliveryPorts.RequestAccount
	Settings    staffMessages.Settings
	Broadcaster realtime.Broadcaster
	Logger      *slog.Logger
	Notifier    func(context.Context, communication.NotificationEvent) error
	Preferences deliveryPorts.NotificationOptIns
	Observe     func(Observation)
	// DB backs the photos and files of messages (#6685); nil refuses uploads.
	DB *bun.DB
}

// NewStaffMessaging also returns the photos and files of the team chat (#6685).
func NewStaffMessaging(cfg StaffMessagingConfig, runtime staffMessages.Runtime, policy deliveryPorts.NotificationErrors) (communication.StaffMessagingRuntime, communication.MessageAttachmentCapability) {
	service := staffMessages.NewService(staffMessages.Config{
		Attachments: messageAttachmentStore(cfg.DB),
		ThreadRepo:  staffMessages.NewThreadRepository(cfg.ThreadRepo), Groups: cfg.ThreadRepo, MessageRepo: staffMessages.NewMessageRepository(cfg.MessageRepo), ReadRepo: staffMessages.NewReadRepository(cfg.ReadRepo),
		Persons: cfg.Persons, Session: cfg.Session, Settings: cfg.Settings, Broadcaster: cfg.Broadcaster,
		Logger: cfg.Logger, Notifier: deliveryPorts.NewNotificationSender(cfg.Notifier), Preferences: cfg.Preferences,
	}, runtime, policy)
	return deliveryPorts.NewStaffMessagingFacade(service, staffMessages.NativeFacadeRows(), staffMessages.NativeFacadeErrors(), communicationPostgres.NewMessageObservation(cfg.Observe)), service
}
