package compose

import (
	"errors"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/modules/exporttransfer"
	"github.com/moto-nrw/project-phoenix/modules/exporttransfer/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/exporttransfer/internal/adapters/sftp"
	"github.com/moto-nrw/project-phoenix/modules/exporttransfer/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/exporttransfer/internal/ports"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// NewTraegerTransfers builds the Träger scope (ADR 0217) with the
// production transport: public destinations only, as for schools.
func NewTraegerTransfers(cipher exporttransfer.BankDataCipher, logger *slog.Logger) (exporttransfer.TraegerTransfers, error) {
	return newTraegerModule(cipher, logger)
}

// newTraegerModule is the single wiring path; the integration test appends
// the loopback policy only.
func newTraegerModule(cipher exporttransfer.BankDataCipher, logger *slog.Logger, transportOptions ...sftp.Option) (exporttransfer.TraegerTransfers, error) {
	if cipher == nil {
		return nil, errors.New("export transfer compose: the bank cipher is required for the Träger scope")
	}
	store := postgres.NewTraegerStore(traegerDatabase())
	return application.NewTraegerService(store, store, retainedUploader{Client: sftp.New(transportOptions...)}, cipher,
		ports.TraegerRuntime{Detached: retainedNewDetachedContext(), WithinTraeger: tenant.WithinTraeger}, logger), nil
}

// NewRecipientRekey builds the maintenance-only rekey of the recipient
// password; never bind it to HTTP.
func NewRecipientRekey(rewrapper exporttransfer.BankDataRewrapper) exporttransfer.RecipientRekey {
	return application.RecipientRekey{Recipients: postgres.NewTraegerStore(traegerDatabase()), Rewrapper: rewrapper}
}

func traegerDatabase() postgres.TraegerDatabase {
	return postgres.NewTraegerDatabase(tenant.TransactionFromContext, tenant.TraegerTransactionFromContext)
}
