package config

import (
	"context"
	"errors"
	"log/slog"

	config "github.com/moto-nrw/project-phoenix/modules/settings/records"
)

type settingsService struct {
	config.OperatorWritePolicy
	valueRepo     config.SettingValueRepository
	auditRepo     config.SettingAuditRepository
	traegerValues config.TraegerSettingRepository
	schoolTraeger func(context.Context, []int64) (map[int64]int64, error)
	schoolStore   config.SchoolSettingsStore
	runtime       config.Runtime
	logger        *slog.Logger
	registry      *config.Registry
	observeLookup config.SettingsLookupObserver
	homeLayouts   *HomeLayoutService
	// classRestrictionGuard, when set, reports whether the tenant in
	// context currently has an active enrollment phase that restricts
	// eligibility to specific school classes. It gates disabling the
	// concrete-class collection toggles so an admin cannot make every
	// submission to such a phase fail (#1663). Optional: nil skips the
	// guard (unit tests, callers that never wire enrollment). Injected at
	// construction (SettingsServiceDependencies) to keep the config package
	// decoupled from the enrollment domain.
	classRestrictionGuard func(ctx context.Context) (bool, error)
	// gradeRestrictionGuard is the same probe for phases restricted to
	// specific GRADE LEVELS. Separate from classRestrictionGuard because the
	// two hang off different toggles: a grade restriction only needs
	// collect_grade_level, so disabling collect_school_class must not be
	// blocked by it (#1663). Optional in the same way.
	gradeRestrictionGuard func(ctx context.Context) (bool, error)
	// gradeCapGuard reports the highest grade any active phase restricts
	// itself to (0 when none does). It gates LOWERING
	// enrollment.grade_level_max: the public form only offers grades up to the
	// cap and the submit path re-checks it, so a cap below a live restriction
	// leaves that phase unable to accept any submission (#1663). Optional in
	// the same way as the two probes above.
	gradeCapGuard func(ctx context.Context) (int, error)
}

// NewSettingsService creates a new config.SettingsService.
func NewSettingsService(
	valueRepo config.SettingValueRepository,
	auditRepo config.SettingAuditRepository,
	schoolStore config.SchoolSettingsStore,
	runtime config.Runtime,
	logger *slog.Logger,
	registries ...*config.Registry,
) config.SettingsService {
	return newSettingsService(valueRepo, auditRepo, schoolStore, runtime, logger, nil, config.ServiceOptions{}, registries...)
}

// SettingsServiceDependencies are every collaborator of the settings service,
// fixed at construction. Registry is optional and defaults to the global
// registry; everything else is required.
type SettingsServiceDependencies struct {
	Values        config.SettingValueRepository
	Audit         config.SettingAuditRepository
	Schools       config.SchoolSettingsStore
	Runtime       config.Runtime
	Logger        *slog.Logger
	HomeLayouts   *HomeLayoutService
	ObserveLookup config.SettingsLookupObserver
	// The Enrollment probes of the collection guards (#1663).
	ClassRestrictionGuard func(ctx context.Context) (bool, error)
	GradeRestrictionGuard func(ctx context.Context) (bool, error)
	GradeCapGuard         func(ctx context.Context) (int, error)
	// TraegerValues is the Träger level every school resolves beneath its
	// own override (ADR 0086); SchoolTraeger names the Träger of each school
	// for the batch read over several schools (Organisation & Tenancy).
	TraegerValues config.TraegerSettingRepository
	SchoolTraeger func(context.Context, []int64) (map[int64]int64, error)
	Registry      *config.Registry
}

// NewSettingsServiceWithDependencies builds the settings service with its
// runtime, lookup observer and collection guards at construction (ADR 0059).
// A missing dependency is a construction error.
func NewSettingsServiceWithDependencies(dependencies SettingsServiceDependencies) (config.BatchSettingsService, error) {
	if dependencies.Values == nil || dependencies.Audit == nil || dependencies.Schools == nil || dependencies.Runtime == nil ||
		dependencies.Logger == nil || dependencies.HomeLayouts == nil || dependencies.ObserveLookup == nil ||
		dependencies.ClassRestrictionGuard == nil || dependencies.GradeRestrictionGuard == nil || dependencies.GradeCapGuard == nil ||
		dependencies.TraegerValues == nil || dependencies.SchoolTraeger == nil {
		return nil, errors.New("settings service: all dependencies are required")
	}
	registry := dependencies.Registry
	if registry == nil {
		registry = config.DefaultRegistry()
	}
	if err := ValidateTraegerLevels(registry); err != nil {
		return nil, err
	}
	return newSettingsService(dependencies.Values, dependencies.Audit, dependencies.Schools, dependencies.Runtime,
		dependencies.Logger, dependencies.HomeLayouts, config.ServiceOptions{
			ObserveLookup: dependencies.ObserveLookup, ClassRestrictionGuard: dependencies.ClassRestrictionGuard,
			GradeRestrictionGuard: dependencies.GradeRestrictionGuard, GradeCapGuard: dependencies.GradeCapGuard,
			TraegerValues: dependencies.TraegerValues, SchoolTraeger: dependencies.SchoolTraeger,
		}, dependencies.Registry), nil
}

func newSettingsService(
	valueRepo config.SettingValueRepository,
	auditRepo config.SettingAuditRepository,
	schoolStore config.SchoolSettingsStore,
	runtime config.Runtime,
	logger *slog.Logger,
	homeLayouts *HomeLayoutService,
	options config.ServiceOptions,
	registries ...*config.Registry,
) *settingsService {
	registry := config.DefaultRegistry()
	if len(registries) > 0 && registries[0] != nil {
		registry = registries[0]
	}
	return &settingsService{
		OperatorWritePolicy:   registry,
		valueRepo:             valueRepo,
		auditRepo:             auditRepo,
		traegerValues:         options.TraegerValues,
		schoolTraeger:         options.SchoolTraeger,
		schoolStore:           schoolStore,
		runtime:               runtime,
		logger:                logger.With("service", "settings"),
		registry:              registry,
		homeLayouts:           homeLayouts,
		observeLookup:         options.ObserveLookup,
		classRestrictionGuard: options.ClassRestrictionGuard,
		gradeRestrictionGuard: options.GradeRestrictionGuard,
		gradeCapGuard:         options.GradeCapGuard,
	}
}

func definitionFor(settings config.SettingsService, key string) *config.Definition {
	if service, ok := settings.(*settingsService); ok {
		return service.registry.GetDefinition(key)
	}
	return config.GetDefinition(key)
}

// --- Validation ---

// --- Login image settings (stored in platform.schools.settings JSONB) ---

const loginImageKey = "loginImageUrl"
