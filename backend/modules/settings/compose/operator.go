// Package compose builds the Settings Platform capabilities over the
// retained settings service (#2736).
package compose

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/settings"
	configSvc "github.com/moto-nrw/project-phoenix/services/config"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// OperatorDependencies are what the operator's school settings need from
// the root. Notify, OpenAttendance and CareLifecycle are optional.
type OperatorDependencies struct {
	Settings configSvc.SettingsService
	// DB opens the school's tenant transaction.
	DB *bun.DB
	// Notify tells open tenant tabs that a setting changed, so they drop
	// their settings caches. Nil disables the broadcast.
	Notify configSvc.SettingsChangedNotifier
	// OpenAttendance feeds the presence-mode switch guard. Nil disables it.
	OpenAttendance configSvc.OpenAttendanceChecker
	// CareLifecycle previews the booking authority impact.
	CareLifecycle careplan.BookingAuthority
	// OnValueSet runs a write's side effects inside the tenant transaction;
	// the closure it returns runs only after a successful commit.
	OnValueSet configSvc.OperatorValueSetHook
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
		settings: deps.Settings,
		runtime:  runtime,
		writes: configSvc.NewOperatorSettingsService(
			deps.Settings,
			runtime,
			deps.Notify,
			deps.OpenAttendance,
			logger,
		),
		careLifecycle: deps.CareLifecycle,
		onValueSet:    deps.OnValueSet,
	}
}

type operatorSchoolSettings struct {
	settings      configSvc.SettingsService
	runtime       operatorSettingsRuntime
	writes        configSvc.OperatorSettingsService
	careLifecycle careplan.BookingAuthority
	onValueSet    configSvc.OperatorValueSetHook
}

func (s *operatorSchoolSettings) CheckOperatorWritable(key string) error {
	return s.settings.CheckOperatorWritable(key)
}

// Schema passes nil permissions: operators see every setting.
func (s *operatorSchoolSettings) Schema(ctx context.Context, schoolID int64) (json.RawMessage, error) {
	var schema *configSvc.SettingsSchema
	err := s.runtime.WithinTenant(ctx, schoolID, func(ctx context.Context) error {
		var schemaErr error
		schema, schemaErr = s.settings.GetSchemaForOperator(ctx, nil)
		return schemaErr
	})
	if err != nil {
		return nil, err
	}
	return json.Marshal(schema)
}

func (s *operatorSchoolSettings) Reveal(ctx context.Context, schoolID int64, key string) (any, error) {
	var value any
	err := s.runtime.WithinTenant(ctx, schoolID, func(ctx context.Context) error {
		var resolveErr error
		value, resolveErr = s.settings.Resolve(ctx, key)
		return resolveErr
	})
	return value, err
}

func (s *operatorSchoolSettings) SetValue(ctx context.Context, schoolID int64, key string, value any, changedBy int64, force bool) error {
	return s.writes.SetValue(ctx, schoolID, key, value, changedBy, force, s.onValueSet)
}

func (s *operatorSchoolSettings) ResetValue(ctx context.Context, schoolID int64, key string, changedBy int64) error {
	return s.writes.ResetValue(ctx, schoolID, key, changedBy, s.onValueSet)
}

// BookingAuthorityImpact is a preview only: the write path evaluates the
// impact again under the booking-write lock, so a stale preview cannot
// bypass the guard.
func (s *operatorSchoolSettings) BookingAuthorityImpact(ctx context.Context, schoolID int64) (*careplan.BookingAuthorityImpact, error) {
	if s.careLifecycle == nil {
		return nil, settings.ErrBookingAuthorityImpactUnavailable
	}
	var impact *careplan.BookingAuthorityImpact
	err := s.runtime.WithinTenant(ctx, schoolID, func(ctx context.Context) error {
		var impactErr error
		impact, impactErr = s.careLifecycle.PreviewBookingAuthorityImpact(ctx, timezone.TodayDate())
		return impactErr
	})
	return impact, err
}

type operatorSettingsRuntime struct{ db *bun.DB }

func (r operatorSettingsRuntime) WithinTenant(ctx context.Context, schoolID int64, fn func(context.Context) error) error {
	return tenant.WithTenantTx(ctx, r.db, schoolID, func(txCtx context.Context, _ bun.Tx) error {
		return fn(txCtx)
	})
}

func (operatorSettingsRuntime) AfterCommit(ctx context.Context, fn func()) {
	tenant.RegisterAfterCommit(ctx, fn)
}

func (operatorSettingsRuntime) Today() configModel.CalendarDate {
	today := timezone.TodayDate()
	return configModel.NewCalendarDate(today.Year(), today.Month(), today.Day())
}
