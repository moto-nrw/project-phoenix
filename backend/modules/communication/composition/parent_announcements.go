package compose

import (
	"context"
	"errors"
	"log/slog"

	records "github.com/moto-nrw/project-phoenix/modules/communication/internal/staffannouncements"

	"github.com/moto-nrw/project-phoenix/modules/communication"
	deliveryPorts "github.com/moto-nrw/project-phoenix/modules/communication/internal/application"
)

// ParentAnnouncementConfig is the compatibility composition seam for the
// users.parent_* persistence that moves in #2675. Callers receive only the
// Communication public capability.
type ParentAnnouncementOutbox interface{ deliveryPorts.MailOutbox }

type ParentAnnouncementPushOutbox interface {
	CancelPendingByRelatedEntity(context.Context, string, int64, string) (int64, error)
}

type ParentAnnouncementConfig struct {
	Repo             deliveryPorts.StaffAnnouncementSource
	Settings         deliveryPorts.Settings
	Outbox           ParentAnnouncementOutbox
	PushOutbox       ParentAnnouncementPushOutbox
	Notifier         func(context.Context, communication.NotificationEvent) error
	ReminderNotifier func(context.Context, communication.NotificationEvent) (int, error)
	Preferences      deliveryPorts.NotificationPreferences
	Deliveries       deliveryPorts.ParentAnnouncementDeliveryRecorder
	Fingerprints     deliveryPorts.Fingerprints
	ParentsURL       string
	Logger           *slog.Logger
}

type ParentAnnouncementEmailConfig communication.MailRendererConfig

// NewParentAnnouncementsWithAttachments composes the parent announcements
// over the file side of their attachments, which File Storage builds from
// its database and object store before it builds its attachment routes
// over these announcements (#4080). A missing file side is a construction
// error.
func NewParentAnnouncementsWithAttachments(cfg ParentAnnouncementConfig, attachments communication.ParentAnnouncementAttachmentPurger, runtime deliveryPorts.Runtime, policy deliveryPorts.ParentDeliveryPolicy) (communication.ParentAnnouncementCapability, error) {
	if attachments == nil {
		return nil, errors.New("parent announcements compose: the attachment purger is required")
	}
	var deliveries deliveryPorts.DeliveryRecorder
	if cfg.Deliveries != nil {
		deliveries = deliveryPorts.NewParentAnnouncementDeliveryAdapter(cfg.Deliveries)
	}
	return deliveryPorts.NewParentAnnouncementFacade(deliveryPorts.NewService(deliveryPorts.ServiceConfig{
		Repo: records.NewRepository(cfg.Repo), Settings: cfg.Settings, Outbox: cfg.Outbox, PushOutbox: cfg.PushOutbox, Notifier: deliveryPorts.NewNotificationSender(cfg.Notifier),
		ReminderNotifier: deliveryPorts.NewDurableNotificationSender(cfg.ReminderNotifier),
		Preferences:      cfg.Preferences, Deliveries: deliveries,
		ParentsURL: cfg.ParentsURL, Logger: cfg.Logger, Fingerprints: cfg.Fingerprints, Attachments: attachments,
	}, runtime, policy)), nil
}

func NewParentAnnouncementRenderer(cfg ParentAnnouncementEmailConfig) communication.MailRenderer {
	return communication.MailRenderer(deliveryPorts.NewAnnouncementRenderer(deliveryPorts.EmailConfig(cfg)))
}
