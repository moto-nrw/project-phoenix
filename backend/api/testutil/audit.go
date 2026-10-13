package testutil

import (
	"github.com/moto-nrw/project-phoenix/services"
	"github.com/uptrace/bun"
)

// NewTestAuditStore composes the Audit append store over the test database
// for root tests that hand it to a cleanup or service composition as their
// Audit command. The Audit composition builds it (ADR 0049).
func NewTestAuditStore(db *bun.DB) services.AuditCommand {
	return services.NewTestAuditStore(db)
}
