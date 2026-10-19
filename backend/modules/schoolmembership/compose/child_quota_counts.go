package compose

import (
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// NewChildQuotaCounts composes the cross-school Kontingentzahl of the
// operator school overview (#3568). It reads on the caller's administrative
// transaction and counts on today's Berlin calendar day, the day every
// membership write is checked on.
func NewChildQuotaCounts() schoolmembership.ChildQuotaCounts {
	return NewChildQuotaCountsWithClock(postgres.BerlinToday)
}

// NewChildQuotaCountsWithClock binds the count day at construction.
func NewChildQuotaCountsWithClock(today func() string) schoolmembership.ChildQuotaCounts {
	return postgres.NewChildQuotaCounts(today, tenant.TransactionFromContext, tenant.IsAdminTx)
}
