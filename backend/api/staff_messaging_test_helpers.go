//go:build testhooks

package api

import (
	"log/slog"

	"github.com/moto-nrw/project-phoenix/modules/communication"
	communicationCompose "github.com/moto-nrw/project-phoenix/modules/communication/composition"
	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
	deliveryPolicy "github.com/moto-nrw/project-phoenix/modules/delivery/devices"
	settingsValues "github.com/moto-nrw/project-phoenix/modules/settings"
	settingsCompose "github.com/moto-nrw/project-phoenix/modules/settings/compose"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-nrw/project-phoenix/tenant"
)

type StaffMessagingTestModule struct {
	StaffMessaging communication.StaffMessagingRuntime
	Settings       settingsCompose.SettingsService
}

func NewStaffMessagingTestModule(db transactionruntime.Database, unit tenant.UnitOfWork) (StaffMessagingTestModule, error) {
	repos, err := NewStaffMessagingTestRepositories(db, MustNewMembershipTestRepositories(db))
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
		Persons: BindStaffSenderNames(people.Users), Settings: settingsValues.NewStaffMessagingValues(settings.Settings), Broadcaster: deliveryCompose.NewRealtimeHub(slog.Default()),
		Logger: slog.Default(), Notifier: BindCommunicationNotificationSender(delivery.Notifications),
		Preferences: delivery.NotificationPreferences, Observe: func(communicationCompose.Observation) {},
	}, CommunicationRequestRuntime{}, deliveryPolicy.ParentDeliveryPolicy{})
	return StaffMessagingTestModule{StaffMessaging: messaging, Settings: settings.Settings}, nil
}
