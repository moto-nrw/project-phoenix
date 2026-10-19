package compose

import (
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/communication"
	"github.com/moto-nrw/project-phoenix/modules/communication/internal/adapters/parentpostgres"
	"github.com/moto-nrw/project-phoenix/modules/communication/internal/adapters/postgres"
	"github.com/uptrace/bun"
)

// NotificationConsentConfig wires Communication's consent store. Observe is
// optional; when set, every operation reports its duration and error under a
// stable name.
type NotificationConsentConfig struct {
	DB      *bun.DB
	Observe func(Observation)
}

// NewNotificationConsent builds the owner capability over
// users.notification_preferences.
func NewNotificationConsent(cfg NotificationConsentConfig) (communication.NotificationConsentCapability, error) {
	if cfg.DB == nil {
		return nil, errors.New("communication compose: notification consent requires a database")
	}
	return parentpostgres.NewNotificationConsentStore(parentDatabase(cfg.DB), postgres.NewMessageObservation(cfg.Observe)), nil
}
