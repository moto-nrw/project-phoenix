package config

import (
	"context"
	"encoding/json"
	"fmt"

	config "github.com/moto-nrw/project-phoenix/modules/settings/records"
)

func (s *settingsService) SetValue(ctx context.Context, key string, value any, changedBy *int64, userPermissions []string) error {
	def, err := s.writeDefinition(key, userPermissions, "set_value")
	if err != nil {
		return err
	}
	if mode, ok := value.(string); ok && parentReportApprovalKey(key) != "" {
		return s.setParentReportMode(ctx, key, mode, changedBy, userPermissions)
	}

	if err := validateValue(def, value); err != nil {
		return &config.SettingsError{
			Op:  "set_value",
			Err: &config.InvalidValueError{Key: key, Reason: err.Error()},
		}
	}

	valueJSON, err := json.Marshal(value)
	if err != nil {
		return &config.SettingsError{Op: "set_value", Err: fmt.Errorf("marshal value: %w", err)}
	}

	tenantID := s.tenantID(ctx)
	if tenantID <= 0 {
		return &config.SettingsError{Op: "set_value", Err: fmt.Errorf("no tenant context")}
	}

	return s.setStoredValue(ctx, tenantID, key, value, valueJSON, changedBy, def)
}

// ResetValue removes a tenant override, falling back to the registry default.
func (s *settingsService) ResetValue(ctx context.Context, key string, changedBy *int64, userPermissions []string) error {
	def, err := s.writeDefinition(key, userPermissions, "reset_value")
	if err != nil {
		return err
	}

	tenantID := s.tenantID(ctx)
	if tenantID <= 0 {
		return &config.SettingsError{Op: "reset_value", Err: fmt.Errorf("no tenant context")}
	}
	if parentReportApprovalKey(key) != "" && ctx.Value(parentReportResetContextKey{}) != true {
		return s.resetParentReportMode(ctx, key, changedBy, userPermissions)
	}

	return s.resetStoredValue(ctx, tenantID, key, changedBy, def)
}

func (s *settingsService) writeDefinition(key string, permissions []string, operation string) (*config.Definition, error) {
	definition := s.registry.GetDefinition(key)
	if definition == nil {
		return nil, &config.SettingsError{Op: operation, Err: &config.DefinitionNotFoundError{Key: key}}
	}
	if err := checkWritePermission(definition, permissions); err != nil {
		return nil, &config.SettingsError{Op: operation, Err: err}
	}
	return definition, nil
}

func (s *settingsService) validateLockedSettingWrite(ctx context.Context, key string, value any, operation string) error {
	if isParentReportSettingKey(key) {
		if err := s.lockParentReportModes(ctx); err != nil {
			return err
		}
	}

	// Serialize a security.mfa_mode change against session mints that are
	// re-deciding their MFA gate right now — see lockMFAPolicy.
	if err := s.lockMFAPolicyForWrite(ctx, key); err != nil {
		return &config.SettingsError{Op: operation, Err: err}
	}
	if err := s.lockParentPickupChangePolicyForWrite(ctx, key); err != nil {
		return &config.SettingsError{Op: operation, Err: err}
	}

	// Cross-field invariants the per-field validation above cannot express.
	// Rejecting the pair here is what keeps an unusable configuration from ever
	// being persisted (#1565 review: an inverted Ganztag cutoff pair passed
	// FieldTime validation and then 500'd every pickup list, options and export).
	if err := s.validateCrossField(ctx, key, value); err != nil {
		// validateCrossField returns *config.InvalidValueError only for a genuinely
		// invalid pair (→ 400); lock/lookup failures stay operational (→ 500).
		return &config.SettingsError{Op: operation, Err: err}
	}

	return nil
}

func (s *settingsService) setStoredValueInTx(ctx context.Context, tenantID int64, key string, value any, valueJSON json.RawMessage, changedBy *int64, def *config.Definition) error {
	if err := s.validateLockedSettingWrite(ctx, key, value, "set_value"); err != nil {
		return err
	}

	// Read current value for audit
	existing, err := s.valueRepo.FindByTenantAndKey(ctx, tenantID, key)
	if err != nil {
		return &config.SettingsError{Op: "set_value", Err: err}
	}
	var oldValue json.RawMessage
	if existing != nil {
		oldValue = existing.Value
	}

	// Upsert
	sv := &config.SettingValue{
		SettingKey: key,
		Value:      valueJSON,
		UpdatedBy:  changedBy,
	}
	sv.TenantID = tenantID

	if err := s.valueRepo.Upsert(ctx, sv); err != nil {
		return &config.SettingsError{Op: "set_value", Err: err}
	}

	// Evict the written key from the request cache so same-request reads —
	// including side-effect hooks running inside this transaction — see the
	// new value instead of a memoized pre-write entry.
	if cache := config.RequestCacheFromContext(ctx); cache != nil {
		cache.EvictKey(tenantID, key)
	}

	s.recordSettingAudit(ctx, tenantID, key, "set", oldValue, valueJSON, changedBy, def.Type == config.FieldPassword)

	s.logger.Info("setting value updated",
		"key", key,
		"tenant_id", tenantID,
	)

	return nil
}

func (s *settingsService) resetStoredValueInTx(ctx context.Context, tenantID int64, key string, changedBy *int64, def *config.Definition) error {
	if err := s.validateLockedSettingWrite(ctx, key, def.Default, "reset_value"); err != nil {
		return err
	}

	// Read current value for audit
	existing, err := s.valueRepo.FindByTenantAndKey(ctx, tenantID, key)
	if err != nil {
		return &config.SettingsError{Op: "reset_value", Err: err}
	}
	var oldValue json.RawMessage
	if existing != nil {
		oldValue = existing.Value
	}

	if err := s.valueRepo.Delete(ctx, tenantID, key); err != nil {
		return &config.SettingsError{Op: "reset_value", Err: err}
	}

	// See SetValue: same-request reads must observe the reset immediately.
	if cache := config.RequestCacheFromContext(ctx); cache != nil {
		cache.EvictKey(tenantID, key)
	}

	s.recordSettingAudit(ctx, tenantID, key, "reset", oldValue, nil, changedBy, def.Type == config.FieldPassword)

	s.logger.Info("setting value reset",
		"key", key,
		"tenant_id", tenantID,
	)

	return nil
}

func (s *settingsService) recordSettingAudit(ctx context.Context, tenantID int64, key, action string, oldValue, newValue json.RawMessage, changedBy *int64, password bool) {
	if password {
		oldValue = json.RawMessage(`"[REDACTED]"`)
		if newValue != nil {
			newValue = json.RawMessage(`"[REDACTED]"`)
		}
	}
	audit := config.NewAuditEntry(tenantID, key, action, oldValue, newValue, changedBy)
	if err := s.auditRepo.Create(ctx, audit); err != nil {
		s.logger.Error("failed to write audit entry",
			"key", key,
			"error", err.Error(),
		)
	}
}

// Each standalone override command commits the locked validation, mutation and
// audit together; a request/workflow transaction remains authoritative.
func (s *settingsService) setStoredValue(ctx context.Context, tenantID int64, key string, value any, valueJSON json.RawMessage, changedBy *int64, def *config.Definition) error {
	if s.runtime == nil {
		return config.ErrRuntimeUnavailable
	}
	return s.runtime.WithinTenant(ctx, tenantID, func(txCtx context.Context) error {
		return s.setStoredValueInTx(txCtx, tenantID, key, value, valueJSON, changedBy, def)
	})
}
func (s *settingsService) resetStoredValue(ctx context.Context, tenantID int64, key string, changedBy *int64, def *config.Definition) error {
	if s.runtime == nil {
		return config.ErrRuntimeUnavailable
	}
	return s.runtime.WithinTenant(ctx, tenantID, func(txCtx context.Context) error {
		return s.resetStoredValueInTx(txCtx, tenantID, key, changedBy, def)
	})
}
