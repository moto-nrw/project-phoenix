package announcement

import (
	"context"
	"log/slog"
)

// Runtime identifies the school of the ambient transaction.
type Runtime interface {
	TenantID(context.Context) int64
}

// NewService wires the staff announcement service.
func NewService(cfg ServiceConfig, runtime Runtime) Service {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &service{
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
		parentsURL:       cfg.ParentsURL,
		logger:           logger,
	}
}
