package api

import (
	"fmt"
	"log/slog"

	owners "github.com/moto-nrw/project-phoenix/roots/owners"

	identityaccessCompose "github.com/moto-nrw/project-phoenix/modules/identityaccess/compose"

	auditCompose "github.com/moto-nrw/project-phoenix/modules/auditlog/compose"
	auditModels "github.com/moto-nrw/project-phoenix/modules/auditlog/records"
	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
)

// NewAuthCleanupService composes the expired session sweep and the password
// reset maintenance the cleanup CLI runs (#3251, #3332). The sweep never
// signs or verifies a session, so it is composed without a signer.
func NewAuthCleanupService(db transactionruntime.Database, logger *slog.Logger, command auditModels.Command) (*identityaccessCompose.SessionMaintenance, error) {
	// The sweep resolves the school and person facts through their owners
	// and clears the push subscriptions through Delivery (#3251, #4013).
	schools, err := owners.NewOrganizationTenancy(db)
	if err != nil {
		return nil, fmt.Errorf("auth cleanup: compose organization tenancy: %w", err)
	}
	events := auditCompose.MustNewPersistence(auditCompose.PersistenceDependencies{DB: db.Pool()}).WithRoutedWrites(command).AuthEvent
	return identityaccessCompose.NewSessionMaintenance(identityaccessCompose.SessionMaintenanceDependencies{
		DB: db.Pool(), Runtime: db.UnitOfWork(),
		Schools: owners.SchoolDirectory{Schools: schools}, Persons: owners.PersonDirectory{Persons: owners.NewPersonRepository(db)},
		Audit: owners.AuthAudit{Command: command, Events: events}, Push: owners.PushSubscriptionCleanup{Subscriptions: deliveryCompose.NewPushSubscriptionRepository(db.Pool())},
		// The cleanup root only removes spent links and stale windows; it
		// never issues a link.
		ResetExpiry: cleanupResetExpiry, Logger: logger,
	})
}
