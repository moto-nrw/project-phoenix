package config

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/models/config"
)

func parentReportApprovalKey(key string) string {
	switch key {
	case config.KeyParentSickReportsEnabled:
		return config.KeyParentSickRequiresApproval
	case config.KeyParentExcusedReportsEnabled:
		return config.KeyParentExcusedRequiresApproval
	default:
		return ""
	}
}

func isParentReportSettingKey(key string) bool {
	return parentReportApprovalKey(key) != "" || key == config.KeyParentSickNoteEnabled ||
		key == config.KeyParentSickRequiresApproval || key == config.KeyParentExcusedRequiresApproval
}

func (s *settingsService) lockParentReportModes(ctx context.Context) error {
	if s.runtime == nil || !s.runtime.HasTransaction(ctx) {
		return ErrRuntimeUnavailable
	}
	if err := s.runtime.AcquireLock(ctx, fmt.Sprintf("parent-report-modes:%d", s.tenantID(ctx)), false); err != nil {
		return err
	}
	s.flushRequestCacheForLock(ctx)
	return nil
}

// The tri-state editor writes the existing approval booleans and only the
// missing independent enable flags. It never persists a second mode value.
func (s *settingsService) setParentReportMode(ctx context.Context, key, mode string, actor *int64, permissions []string) error {
	if s.tenantID(ctx) <= 0 {
		return fmt.Errorf("no tenant context")
	}
	if mode != "off" && mode != "immediate" && mode != "approval" {
		return &InvalidValueError{Key: key, Reason: "Bitte wählen Sie eine gültige Meldeart."}
	}
	if s.runtime == nil {
		return ErrRuntimeUnavailable
	}
	if !s.runtime.HasTransaction(ctx) {
		return s.runtime.WithinTenant(ctx, s.tenantID(ctx), func(txCtx context.Context) error {
			return s.setParentReportMode(txCtx, key, mode, actor, permissions)
		})
	}
	if err := s.lockParentReportModes(ctx); err != nil {
		return err
	}
	if mode == "off" {
		return s.SetValue(ctx, key, false, actor, permissions)
	}

	// Read under the lock without using immutable read-path snapshots.
	keys := []string{config.KeyParentSickNoteEnabled}
	stored, err := s.valueRepo.FindByTenantAndKeys(ctx, s.tenantID(ctx), keys)
	if err != nil {
		return err
	}
	snapshot, err := newSettingsSnapshot(s.registry, s.tenantID(ctx), keys, stored)
	if err != nil {
		return err
	}
	enabled, err := snapshot.Bool(config.KeyParentSickNoteEnabled)
	if err != nil {
		return err
	}
	if !enabled {
		other := config.KeyParentExcusedReportsEnabled
		if key == other {
			other = config.KeyParentSickReportsEnabled
		}
		// Enabling one kind must not revive the other behind the shared legacy
		// switch. Its remembered approval choice stays untouched.
		if err := s.SetValue(ctx, other, false, actor, permissions); err != nil {
			return err
		}
		if err := s.SetValue(ctx, config.KeyParentSickNoteEnabled, true, actor, permissions); err != nil {
			return err
		}
	}
	if err := s.SetValue(ctx, parentReportApprovalKey(key), mode == "approval", actor, permissions); err != nil {
		return err
	}
	return s.SetValue(ctx, key, true, actor, permissions)
}

func projectParentReportModes(resolved, output map[string]*ResolvedSetting, snapshot *SettingsSnapshot) error {
	for _, key := range []string{config.KeyParentSickReportsEnabled, config.KeyParentExcusedReportsEnabled} {
		field := resolved[key]
		if field == nil {
			continue
		}
		approvalKey := parentReportApprovalKey(key)
		main, err := snapshot.Bool(config.KeyParentSickNoteEnabled)
		if err != nil {
			return err
		}
		enabled, err := snapshot.Bool(key)
		if err != nil {
			return err
		}
		approval, err := snapshot.Bool(approvalKey)
		if err != nil {
			return err
		}
		hasEnableOverride, err := snapshot.HasOverride(key)
		if err != nil {
			return err
		}
		hasApprovalOverride, err := snapshot.HasOverride(approvalKey)
		if err != nil {
			return err
		}
		mode := "off"
		if main && enabled {
			mode = "immediate"
			if approval {
				mode = "approval"
			}
		}
		field.Type = config.FieldSelect
		field.Value = mode
		field.Default = "off"
		if main {
			field.Default = "approval"
		}
		field.IsDefault = !hasEnableOverride && !hasApprovalOverride
		field.Options = &config.SelectOptions{Static: []config.SelectOption{
			{Label: "Aus", Value: "off"},
			{Label: "Sofort übernehmen", Value: "immediate"},
			{Label: "Erst bestätigen", Value: "approval"},
		}}
		delete(output, approvalKey)
		delete(output, config.KeyParentSickNoteEnabled)
	}
	return nil
}

type parentReportResetContextKey struct{}

func (s *settingsService) resetParentReportMode(ctx context.Context, key string, actor *int64, permissions []string) error {
	if s.runtime == nil {
		return ErrRuntimeUnavailable
	}
	if !s.runtime.HasTransaction(ctx) {
		return s.runtime.WithinTenant(ctx, s.tenantID(ctx), func(txCtx context.Context) error {
			return s.resetParentReportMode(txCtx, key, actor, permissions)
		})
	}
	if err := s.lockParentReportModes(ctx); err != nil {
		return err
	}
	ctx = context.WithValue(ctx, parentReportResetContextKey{}, true)
	if err := s.ResetValue(ctx, parentReportApprovalKey(key), actor, permissions); err != nil {
		return err
	}
	return s.ResetValue(ctx, key, actor, permissions)
}
