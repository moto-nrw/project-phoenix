//go:build testhooks

package api

import (
	"log/slog"

	owners "github.com/moto-nrw/project-phoenix/roots/owners"

	"github.com/moto-nrw/project-phoenix/modules/communication"
	communicationCompose "github.com/moto-nrw/project-phoenix/modules/communication/composition"
	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
	deliveryPolicy "github.com/moto-nrw/project-phoenix/modules/delivery/devices"
	settingsValues "github.com/moto-nrw/project-phoenix/modules/settings"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// StaffMessagingTestModule is the Communication owner's staff messaging.
type StaffMessagingTestModule struct {
	StaffMessaging communication.StaffMessagingRuntime
}

// NewStaffMessagingTestModule composes the staff messaging over db and
// returns it with the tenant settings it reads.
func NewStaffMessagingTestModule(db transactionruntime.Database, unit tenant.UnitOfWork) (StaffMessagingTestModule, owners.SettingsTestModule, error) {
	repos, err := NewStaffMessagingTestRepositories(db, owners.MustNewMembershipTestRepositories(db))
	if err != nil {
		return StaffMessagingTestModule{}, owners.SettingsTestModule{}, err
	}
	people, err := NewRFIDTestPeople(db)
	if err != nil {
		return StaffMessagingTestModule{}, owners.SettingsTestModule{}, err
	}
	settings, err := owners.NewSettingsTestModule(db, unit)
	if err != nil {
		return StaffMessagingTestModule{}, owners.SettingsTestModule{}, err
	}
	delivery, err := NewDeliveryTestModule(db, unit)
	if err != nil {
		return StaffMessagingTestModule{}, owners.SettingsTestModule{}, err
	}
	messaging := communicationCompose.NewStaffMessaging(communicationCompose.StaffMessagingConfig{
		ThreadRepo: repos.Thread, MessageRepo: repos.Message, ReadRepo: repos.Read,
		Persons: owners.BindStaffSenderNames(people), Session: CommunicationSession{}, Settings: settingsValues.NewStaffMessagingValues(settings.Settings), Broadcaster: deliveryCompose.NewRealtimeHub(slog.Default()),
		Logger: slog.Default(), Notifier: owners.BindCommunicationNotificationSender(delivery.Notifications),
		Preferences: delivery.NotificationPreferences, Observe: func(communicationCompose.Observation) {},
	}, CommunicationRequestRuntime{}, deliveryPolicy.ParentDeliveryPolicy{})
	return StaffMessagingTestModule{StaffMessaging: messaging}, settings, nil
}
