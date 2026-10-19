// Package compose builds the Settings Platform capabilities over the
// retained settings service (#2736).
package compose

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/modules/settings"
	configSvc "github.com/moto-nrw/project-phoenix/modules/settings/internal/application"
	configModel "github.com/moto-nrw/project-phoenix/modules/settings/records"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// OperatorDependencies are what the operator's school settings need from
// the root. Notify and OpenAttendance are optional.
type OperatorDependencies struct {
	Settings configModel.SettingsService
	// DB opens the school's tenant transaction.
	DB *bun.DB
	// Notify tells open tenant tabs that a setting changed, so they drop
	// their settings caches. Nil disables the broadcast.
	Notify configModel.SettingsChangedNotifier
	// OpenAttendance feeds the presence-mode switch guard. Nil disables it.
	OpenAttendance configSvc.OpenAttendanceChecker
	// OnValueSet runs a write's side effects inside the tenant transaction;
	// the closure it returns runs only after a successful commit.
	OnValueSet configModel.OperatorValueSetHook
	Logger     *slog.Logger
}

// NewOperatorSchoolSettings composes the operator's management of one
// school's settings.
func NewOperatorSchoolSettings(deps OperatorDependencies) settings.OperatorSchoolSettings {
	logger := deps.Logger
	if logger == nil {
		logger = slog.Default()
	}
	runtime := operatorSettingsRuntime{db: deps.DB}
	return &operatorSchoolSettings{
		settings:        deps.Settings,
		runtime:         runtime,
		SettingsSchemas: deps.Settings,
		OperatorSettingsService: configSvc.NewOperatorSettingsService(
			deps.Settings,
			runtime,
			deps.Notify,
			deps.OpenAttendance,
			logger, deps.OnValueSet,
		),
	}
}

type operatorSchoolSettings struct {
	configModel.SettingsSchemas
	configSvc.OperatorSettingsService
	settings configModel.SettingsService
	runtime  operatorSettingsRuntime
}

// Schema passes nil permissions: operators see every setting.
func (s *operatorSchoolSettings) Schema(ctx context.Context, schoolID int64) (json.RawMessage, error) {
	return configSvc.OperatorSettingSchema(ctx, schoolID, s.runtime.WithinTenant, func() configModel.SettingsService { return s.settings })
}

func (s *operatorSchoolSettings) Reveal(ctx context.Context, schoolID int64, key string) (any, error) {
	return configSvc.OperatorSettingReveal(ctx, schoolID, key, s.runtime.WithinTenant, func() configModel.SettingsService { return s.settings })
}

type operatorSettingsRuntime struct{ db *bun.DB }

func (r operatorSettingsRuntime) WithinTenant(ctx context.Context, schoolID int64, fn func(context.Context) error) error {
	return tenant.WithTenantTx(ctx, r.db, schoolID, configSvc.SettingContextCallback[bun.Tx](fn))
}

func (operatorSettingsRuntime) AfterCommit(ctx context.Context, fn func()) {
	tenant.RegisterAfterCommit(ctx, fn)
}

func (operatorSettingsRuntime) Today() configModel.CalendarDate {
	return configSvc.OperatorSettingToday()
}
