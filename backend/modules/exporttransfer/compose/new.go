// Package compose wires the Export Transfer capability.
package compose

import (
	"context"
	"errors"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/modules/exporttransfer"
	"github.com/moto-nrw/project-phoenix/modules/exporttransfer/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/exporttransfer/internal/adapters/sftp"
	"github.com/moto-nrw/project-phoenix/modules/exporttransfer/internal/application"
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
	service := application.New(resolver, retainedUploader{Client: sftp.New(transportOptions...)}, postgres.New(retainedDatabase()), ports.CommandRuntime{
		Active:       tenant.HasAfterCommitHooks,
		Detached:     retainedNewDetachedContext(),
		WithinTenant: tenant.WithinCurrentTenant,
	}, dependencies.Logger)
	return exporttransfer.NewModule(retainedEngine{Service: service}), nil
}
