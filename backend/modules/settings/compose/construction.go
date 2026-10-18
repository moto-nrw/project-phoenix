package compose

import (
	"context"
	"log/slog"

	configModel "github.com/moto-nrw/project-phoenix/modules/settings/records"
	configSvc "github.com/moto-nrw/project-phoenix/services/config"
	"github.com/moto-nrw/project-phoenix/services/config/sideeffects"
)

// SettingsDependencies are every collaborator of the settings service at
// construction (ADR 0059): the root's tenant runtime, the lookup observer
// and Enrollment's collection-guard probes (#1663). Registry is optional.
type SettingsDependencies struct {
	Values                configModel.SettingValueRepository
	Audit                 configModel.SettingAuditRepository
	Schools               SchoolSettingsStore
	Runtime               SettingsRuntime
	Logger                *slog.Logger
	HomeLayouts           *configSvc.HomeLayoutService
	ObserveLookup         SettingsLookupObserver
	ClassRestrictionGuard func(ctx context.Context) (bool, error)
	GradeRestrictionGuard func(ctx context.Context) (bool, error)
	GradeCapGuard         func(ctx context.Context) (int, error)
	Registry              *configModel.Registry
}

// NewSettings builds the settings service with every dependency; a missing
// one is a construction error.
func NewSettings(dependencies SettingsDependencies) (BatchSettingsService, error) {
	return configSvc.NewSettingsServiceWithDependencies(configSvc.SettingsServiceDependencies(dependencies))
}

// SettingSideEffect binds one setting key to its write side effect, which
// runs in the write transaction and may return a post-commit closure.
type SettingSideEffect struct {
	Key    string
	Handle func(ctx context.Context, tenantID int64, value any) (postCommit func(), err error)
}

// NewSideEffects builds the side-effect registry with its failure observer
// and every side effect at construction. A missing observer, a nil handler
// or a key bound twice is a construction error.
func NewSideEffects(observer SideEffectFailureObserver, effects ...SettingSideEffect) (*SideEffectRegistry, error) {
	handlers := make([]sideeffects.Handler, 0, len(effects))
	for _, effect := range effects {
		handlers = append(handlers, sideeffects.Handler{Key: effect.Key, Handle: effect.Handle})
	}
	return sideeffects.NewRegistryWith(observer, handlers...)
}
