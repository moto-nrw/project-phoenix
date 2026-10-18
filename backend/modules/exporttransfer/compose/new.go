// Package compose wires the Export Transfer capability.
package compose

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/modules/exporttransfer"
	"github.com/moto-nrw/project-phoenix/modules/exporttransfer/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/exporttransfer/internal/adapters/sftp"
	"github.com/moto-nrw/project-phoenix/modules/exporttransfer/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/exporttransfer/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/exporttransfer/internal/ports"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// Settings is the per-school configuration this capability reads. The caller
// supplies the resolvers, so the module never learns which settings system is
// behind them.
type Settings struct {
	Enabled            func(context.Context) (bool, error)
	Host               func(context.Context) (string, error)
	Port               func(context.Context) (int, error)
	Username           func(context.Context) (string, error)
	Password           func(context.Context) (string, error)
	RemoteDirectory    func(context.Context) (string, error)
	HostKeyFingerprint func(context.Context) (string, error)
}

func (s Settings) complete() bool {
	return s.Enabled != nil && s.Host != nil && s.Port != nil && s.Username != nil &&
		s.Password != nil && s.RemoteDirectory != nil && s.HostKeyFingerprint != nil
}

// SettingKeys names the setting keys in form order, so an incomplete
// configuration can tell the school exactly which fields are still empty
// without this package hardcoding the settings vocabulary.
type SettingKeys struct {
	Host               string
	Port               string
	Username           string
	Password           string
	RemoteDirectory    string
	HostKeyFingerprint string
}

type Dependencies struct {
	DB       *bun.DB
	Settings Settings
	Keys     SettingKeys
	Logger   *slog.Logger
}

// New builds the capability with the production transport: public
// destinations only. The address policy is the guard that keeps a settings
// field from becoming a way to reach the internal network, so nothing here
// exposes a way to relax it.
func New(dependencies Dependencies) (*exporttransfer.Module, error) {
	// No parameter for the address policy on purpose: the public-only default
	// IS the guard, and a production caller must have no way to weaken it.
	return newModule(dependencies)
}

// newModule is the single wiring path. The integration test calls it with the
// loopback policy appended, so the test exercises exactly what production
// builds — minus the one rule it cannot satisfy from a test server.
func newModule(dependencies Dependencies, transportOptions ...sftp.Option) (*exporttransfer.Module, error) {
	if dependencies.DB == nil || !dependencies.Settings.complete() {
		return nil, errors.New("export transfer compose: DB and all settings resolvers are required")
	}
	if dependencies.Keys.Host == "" || dependencies.Keys.HostKeyFingerprint == "" {
		return nil, errors.New("export transfer compose: setting keys are required")
	}

	resolver := application.SettingsResolver{Settings: application.Settings(dependencies.Settings), Keys: application.SettingKeys(dependencies.Keys)}
	service := application.New(resolver, uploader{client: sftp.New(transportOptions...)}, postgres.New(database()), ports.CommandRuntime{
		Active: tenant.HasAfterCommitHooks,
		Detached: func(ctx context.Context) context.Context {
			return tenant.ContextWithoutAfterCommitHooks(tenant.ContextWithoutTransaction(ctx))
		},
		WithinTenant: tenant.WithinCurrentTenant,
	}, dependencies.Logger)
	return exporttransfer.NewModule(engine{service: service}), nil
}

// database yields the caller's tenant transaction for the journal. The entry
// is written on the SAME transaction as the request, so it cannot end up
// describing a state the database never reached.
func database() postgres.Database {
	return func(ctx context.Context) (bun.IDB, int64, error) {
		transaction, ok := tenant.TransactionFromContext(ctx)
		if !ok {
			return nil, 0, errors.New("export transfer postgres: transaction is required")
		}
		tx, ok := transaction.(bun.Tx)
		if !ok {
			return nil, 0, fmt.Errorf("export transfer postgres: unsupported transaction %T", transaction)
		}
		tenantID, err := tenant.TenantFromContext(ctx)
		if err != nil {
			return nil, 0, err
		}
		return tx, tenantID.Int64(), nil
	}
}

// uploader binds the transport to the port. Port and transport carry their
// own destination types so neither imports the other; translating between
// them is composition work, and it stays a field copy — any rule about WHERE
// a file may go belongs in the transport's address policy.
type uploader struct{ client *sftp.Client }

func (u uploader) Prepare(ctx context.Context, target domain.Target, filename string, data []byte) (func() error, func() error, error) {
	pending, err := u.client.Prepare(ctx, sftp.Target{
		Host:               target.Host,
		Port:               target.Port,
		Username:           target.Username,
		Password:           target.Password,
		RemoteDirectory:    target.RemoteDirectory,
		HostKeyFingerprint: target.HostKeyFingerprint,
	}, filename, data)
	if err != nil {
		return nil, nil, err
	}
	return pending.Commit, pending.Rollback, nil
}

// engine translates between the domain values the application works on and
// the public capability types. Keeping the translation here is what lets the
// application layer stay free of the contract package.
type engine struct{ service *application.Service }

func (e engine) Status(ctx context.Context) (exporttransfer.Status, error) {
	state, err := e.service.State(ctx)
	if err != nil {
		return exporttransfer.Status{}, err
	}
	return exporttransfer.Status{
		Enabled:         state.Enabled,
		Ready:           state.Ready(),
		Host:            state.Host,
		Port:            state.Port,
		RemoteDirectory: state.RemoteDirectory,
		MissingSettings: state.MissingSettings,
	}, nil
}

func (e engine) Transfer(ctx context.Context, request exporttransfer.Request) (exporttransfer.Outcome, error) {
	result, err := e.service.Transfer(ctx, domain.Request{
		Kind:           request.Kind,
		Format:         request.Format,
		Filename:       request.Filename,
		Data:           request.Data,
		PrepareFile:    request.PrepareFile,
		ActorAccountID: request.ActorAccountID,
		ActorName:      request.ActorName,
	})
	if err != nil {
		return exporttransfer.Outcome{}, err
	}
	return exporttransfer.Outcome{
		Transferred:     result.Success,
		Filename:        result.Filename,
		ByteSize:        result.ByteSize,
		TargetHost:      result.TargetHost,
		TargetDirectory: result.TargetDirectory,
		Reason:          result.Reason,
	}, nil
}
