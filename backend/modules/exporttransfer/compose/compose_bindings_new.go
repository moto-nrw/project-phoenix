package compose

import (
	context "context"

	exporttransfer "github.com/moto-nrw/project-phoenix/modules/exporttransfer"
	postgres "github.com/moto-nrw/project-phoenix/modules/exporttransfer/internal/adapters/postgres"
	sftp "github.com/moto-nrw/project-phoenix/modules/exporttransfer/internal/adapters/sftp"
	application "github.com/moto-nrw/project-phoenix/modules/exporttransfer/internal/application"
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

type retainedEngine struct{ Service *application.Service }

func (e retainedEngine) Status(ctx context.Context) (exporttransfer.Status, error) {
	return application.StatusBinding(ctx, e.Service, newStatus)
}
func (e retainedEngine) Transfer(ctx context.Context, request exporttransfer.Request) (exporttransfer.Outcome, error) {
	return application.TransferBinding(ctx, e.Service, domain.Request(request), newOutcome)
}

func newStatus(enabled, ready bool, host string, port int, directory string, missing []string) exporttransfer.Status {
	return exporttransfer.Status{Enabled: enabled, Ready: ready, Host: host, Port: port, RemoteDirectory: directory, MissingSettings: missing}
}
func newOutcome(success bool, filename string, size int64, host, directory, reason string) exporttransfer.Outcome {
	return exporttransfer.Outcome{Transferred: success, Filename: filename, ByteSize: size, TargetHost: host, TargetDirectory: directory, Reason: reason}
}
