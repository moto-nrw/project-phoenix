package compose

import (
	context "context"

	postgres "github.com/moto-nrw/project-phoenix/modules/exporttransfer/internal/adapters/postgres"
	sftp "github.com/moto-nrw/project-phoenix/modules/exporttransfer/internal/adapters/sftp"
	domain "github.com/moto-nrw/project-phoenix/modules/exporttransfer/internal/domain"
	tenant "github.com/moto-nrw/project-phoenix/tenant"
)

// database yields the caller's tenant transaction for the journal. The entry
// is written on the SAME transaction as the request, so it cannot end up
// describing a state the database never reached.
func retainedDatabase() postgres.Database {
	return postgres.NewScopedDatabase(tenant.TransactionFromContext, tenant.TenantFromContext)
}

type retainedUploader struct{ Client *sftp.Client }

func (u retainedUploader) Prepare(ctx context.Context, target domain.Target, filename string, data []byte) (func() error, func() error, error) {
	return sftp.PreparePortUpload(ctx, u.Client, sftp.Target(target), filename, data)
}
