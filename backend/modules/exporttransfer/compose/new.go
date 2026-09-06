// Package compose wires the Export Transfer capability.
package compose

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/exporttransfer"
	"github.com/moto-nrw/project-phoenix/modules/exporttransfer/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/exporttransfer/internal/adapters/sftp"
	"github.com/moto-nrw/project-phoenix/modules/exporttransfer/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/exporttransfer/internal/domain"
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

	journal := postgres.New(database())

	resolver := settingsResolver{settings: dependencies.Settings, keys: dependencies.Keys}
	service := application.New(resolver, uploader{client: sftp.New(transportOptions...)}, journal, dependencies.Logger)
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

func (u uploader) Upload(ctx context.Context, target domain.Target, filename string, data []byte) error {
	return u.client.Upload(ctx, sftp.Target{
		Host:               target.Host,
		Port:               target.Port,
		Username:           target.Username,
		Password:           target.Password,
		RemoteDirectory:    target.RemoteDirectory,
		HostKeyFingerprint: target.HostKeyFingerprint,
	}, filename, data)
}

// settingsResolver turns the seven settings into a validated target.
//
// Completeness is decided here; reachability is not. Whether the host is a
// public address and presents the configured key is decided at connect time
// by the transport — those are properties of the network, not of a form.
type settingsResolver struct {
	settings Settings
	keys     SettingKeys
}

func (r settingsResolver) read(ctx context.Context) (bool, domain.Target, error) {
	enabled, err := r.settings.Enabled(ctx)
	if err != nil {
		return false, domain.Target{}, fmt.Errorf("resolve sftp enabled: %w", err)
	}

	target := domain.Target{}
	// Trimmed: surrounding whitespace in a host, path or fingerprint is
	// always a copy-paste artefact, never part of the value.
	for _, field := range []struct {
		read func(context.Context) (string, error)
		into *string
		name string
	}{
		{r.settings.Host, &target.Host, "host"},
		{r.settings.Username, &target.Username, "username"},
		{r.settings.RemoteDirectory, &target.RemoteDirectory, "remote directory"},
		{r.settings.HostKeyFingerprint, &target.HostKeyFingerprint, "host key fingerprint"},
	} {
		value, err := field.read(ctx)
		if err != nil {
			return false, domain.Target{}, fmt.Errorf("resolve sftp %s: %w", field.name, err)
		}
		*field.into = strings.TrimSpace(value)
	}

	// The password is taken EXACTLY as stored. Trimming it would authenticate
	// with a different secret than the school entered, and the failure would
	// look like a wrong password on the far side.
	password, err := r.settings.Password(ctx)
	if err != nil {
		return false, domain.Target{}, fmt.Errorf("resolve sftp password: %w", err)
	}
	target.Password = password

	port, err := r.settings.Port(ctx)
	if err != nil {
		return false, domain.Target{}, fmt.Errorf("resolve sftp port: %w", err)
	}
	target.Port = port

	return enabled, target, nil
}

// missing lists the setting keys that block a transfer, in form order. The
// password is checked for presence only; its content is never inspected.
func (r settingsResolver) missing(target domain.Target) []string {
	var missing []string
	if target.Host == "" {
		missing = append(missing, r.keys.Host)
	}
	if !domain.ValidPort(target.Port) {
		missing = append(missing, r.keys.Port)
	}
	if target.Username == "" {
		missing = append(missing, r.keys.Username)
	}
	if target.Password == "" {
		missing = append(missing, r.keys.Password)
	}
	if !domain.ValidRemoteDirectory(target.RemoteDirectory) {
		missing = append(missing, r.keys.RemoteDirectory)
	}
	if !domain.ValidFingerprint(target.HostKeyFingerprint) {
		missing = append(missing, r.keys.HostKeyFingerprint)
	}
	return missing
}

func (r settingsResolver) Resolve(ctx context.Context) (domain.Target, error) {
	enabled, target, err := r.read(ctx)
	if err != nil {
		return domain.Target{}, err
	}
	if !enabled {
		return domain.Target{}, fmt.Errorf("%w: transfer is switched off", domain.ErrNotConfigured)
	}
	if missing := r.missing(target); len(missing) > 0 {
		return domain.Target{}, fmt.Errorf("%w: missing %s", domain.ErrNotConfigured, strings.Join(missing, ", "))
	}
	return target, nil
}

func (r settingsResolver) State(ctx context.Context) (domain.TargetState, error) {
	enabled, target, err := r.read(ctx)
	if err != nil {
		return domain.TargetState{}, err
	}
	state := domain.TargetState{
		Enabled:         enabled,
		Host:            target.Host,
		RemoteDirectory: target.RemoteDirectory,
		MissingSettings: r.missing(target),
	}
	if domain.ValidPort(target.Port) {
		state.Port = target.Port
	}
	return state, nil
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
