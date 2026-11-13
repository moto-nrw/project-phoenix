package config

import (
	"context"
	"encoding/json"

	config "github.com/moto-ogs/project-phoenix/modules/settings/records"
)

// OperatorSchoolSettings combines the existing schema and write capabilities.
type OperatorSchoolSettings struct {
	config.SettingsSchemas
	OperatorSettingsService
	settings config.SettingsService
	runtime  OperatorSettingsRuntime
}

func NewOperatorSchoolSettings(settings config.SettingsService, runtime OperatorSettingsRuntime, writes OperatorSettingsService) *OperatorSchoolSettings {
	return &OperatorSchoolSettings{SettingsSchemas: settings, OperatorSettingsService: writes, settings: settings, runtime: runtime}
}

// Schema passes nil permissions: operators see every setting.
func (s *OperatorSchoolSettings) Schema(ctx context.Context, schoolID int64) (json.RawMessage, error) {
	return OperatorSettingSchema(ctx, schoolID, s.runtime.WithinTenant, func() config.SettingsService { return s.settings })
}

func (s *OperatorSchoolSettings) Reveal(ctx context.Context, schoolID int64, key string) (any, error) {
	return OperatorSettingReveal(ctx, schoolID, key, s.runtime.WithinTenant, func() config.SettingsService { return s.settings })
}
