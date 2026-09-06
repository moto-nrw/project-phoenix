package services

import (
	"log/slog"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/modules/communication"
	communicationCompose "github.com/moto-nrw/project-phoenix/modules/communication/composition"
	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
	"github.com/moto-nrw/project-phoenix/services/config"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

type StaffMessagingTestModule struct {
	StaffMessaging communication.StaffMessagingRuntime
	Settings       config.SettingsService
}

func NewStaffMessagingTestModule(db *bun.DB, unit tenant.UnitOfWork) (StaffMessagingTestModule, error) {
	repos, err := repositories.NewStaffMessagingTestRepositories(db)
	if err != nil {
		return StaffMessagingTestModule{}, err
	}
	people, err := NewRFIDTestModule(db)
	if err != nil {
		return StaffMessagingTestModule{}, err
	}
	settings, err := NewSettingsTestModule(db, unit)
	if err != nil {
		return StaffMessagingTestModule{}, err
	}
	delivery, err := NewDeliveryTestModule(db, unit)
	if err != nil {
		return StaffMessagingTestModule{}, err
	}
	messaging := communicationCompose.NewStaffMessaging(communicationCompose.StaffMessagingConfig{
		ThreadRepo: repos.Thread, MessageRepo: repos.Message, ReadRepo: repos.Read,
		Persons: people.Users, Settings: settings.Settings, Broadcaster: deliveryCompose.NewRealtimeHub(slog.Default()),
		DB: db, Logger: slog.Default(), Notifier: delivery.Notifications,
		Preferences: delivery.NotificationPreferences, Observe: func(communicationCompose.Observation) {},
	})
	return StaffMessagingTestModule{StaffMessaging: messaging, Settings: settings.Settings}, nil
}
