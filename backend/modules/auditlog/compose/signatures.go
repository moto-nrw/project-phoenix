package compose

import (
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/auditlog"
	"github.com/moto-nrw/project-phoenix/modules/auditlog/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// NewSignatureLedger composes the signature ledger (#4785). Like the
// guardian change trail it holds no database handle: every append and read
// runs in the caller's ambient tenant transaction and is refused without one.
// fingerprint is the SHA-256 content identity the root binds from Security
// Runtime.
func NewSignatureLedger(fingerprint func([]byte) string) (auditlog.SignatureLedger, error) {
	if fingerprint == nil {
		return nil, errors.New("audit signatures: fingerprint is required")
	}
	return postgres.NewSignatures(postgres.NewAmbientTransaction("audit signatures", tenant.TenantFromContext, tenant.TransactionFromContext), fingerprint), nil
}
