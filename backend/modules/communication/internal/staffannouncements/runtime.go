package announcement

import (
	"context"
	"log/slog"

	deliveryPorts "github.com/moto-nrw/project-phoenix/modules/communication/internal/application"
)

// Runtime identifies the school of the ambient transaction.
type Runtime interface {
	TenantID(context.Context) int64
}

// NewService wires the staff announcement service.
func NewService(cfg ServiceConfig, runtime Runtime, policy deliveryPorts.ParentDeliveryPolicy) Service {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &service{
		deliveryPolicy:   policy,
		runtime:          runtime,
		repo:             cfg.Repo,
		settings:         cfg.Settings,
		outbox:           cfg.Outbox,
		pushOutbox:       cfg.PushOutbox,
		notifier:         cfg.Notifier,
		reminderNotifier: cfg.ReminderNotifier,
		preferences:      cfg.Preferences,
		deliveries:       cfg.Deliveries,
		fingerprints:     cfg.Fingerprints,
		attachments:      cfg.Attachments,
		parentsURL:       cfg.ParentsURL,
		logger:           logger,
	}
}
