package config

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/models/platform"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

type settingsService struct {
	valueRepo  config.SettingValueRepository
	auditRepo  config.SettingAuditRepository
	schoolRepo platform.SchoolRepository
	db         *bun.DB
	logger     *slog.Logger
}

// NewSettingsService creates a new SettingsService.
func NewSettingsService(
	valueRepo config.SettingValueRepository,
	auditRepo config.SettingAuditRepository,
	schoolRepo platform.SchoolRepository,
	db *bun.DB,
	logger *slog.Logger,
) SettingsService {
	return &settingsService{
		valueRepo:  valueRepo,
		auditRepo:  auditRepo,
		schoolRepo: schoolRepo,
		db:         db,
		logger:     logger.With("service", "settings"),
	}
}

// Resolve returns the value for a setting: tenant override if it exists,
// otherwise the registry default.
func (s *settingsService) Resolve(ctx context.Context, key string) (any, error) {
	def := config.GetDefinition(key)
	if def == nil {
		return nil, &SettingsError{
			Op:  "resolve",
			Err: &DefinitionNotFoundError{Key: key},
		}
	}

	tenantID := tenant.FromContext(ctx)
	if tenantID > 0 {
		sv, err := s.valueRepo.FindByTenantAndKey(ctx, tenantID, key)
		if err != nil {
			return nil, &SettingsError{Op: "resolve", Err: err}
		}
		if sv != nil {
			var value any
			if err := json.Unmarshal(sv.Value, &value); err != nil {
				return nil, &SettingsError{Op: "resolve", Err: fmt.Errorf("unmarshal value: %w", err)}
			}
			return value, nil
		}
	}

	return def.Default, nil
}

// ResolveStringForTenant resolves a setting as a string for a specific tenant,
// wrapping the query in a tenant transaction to satisfy RLS.
func (s *settingsService) ResolveStringForTenant(ctx context.Context, tenantID int64, key string) (string, error) {
	var result string
	err := tenant.WithTenantTx(ctx, s.db, tenantID, func(txCtx context.Context, _ bun.Tx) error {
		val, resolveErr := s.ResolveString(txCtx, key)
		if resolveErr != nil {
			return resolveErr
		}
		result = val
		return nil
	})
	if err != nil {
		return "", err
	}
	return result, nil
}

// ResolveBoolForTenant resolves a setting as a bool for a specific tenant,
// wrapping the query in a tenant transaction to satisfy RLS. Required from
// call sites that run outside TenantTxMiddleware (e.g. /auth/mfa/verify)
// but already know which tenant they're acting on.
func (s *settingsService) ResolveBoolForTenant(ctx context.Context, tenantID int64, key string) (bool, error) {
	var result bool
	err := tenant.WithTenantTx(ctx, s.db, tenantID, func(txCtx context.Context, _ bun.Tx) error {
		val, resolveErr := s.ResolveBool(txCtx, key)
		if resolveErr != nil {
			return resolveErr
		}
		result = val
		return nil
	})
	if err != nil {
		return false, err
	}
	return result, nil
}

// ResolveIntForTenant mirrors ResolveBoolForTenant but for integer settings.
func (s *settingsService) ResolveIntForTenant(ctx context.Context, tenantID int64, key string) (int, error) {
	var result int
	err := tenant.WithTenantTx(ctx, s.db, tenantID, func(txCtx context.Context, _ bun.Tx) error {
		val, resolveErr := s.ResolveInt(txCtx, key)
		if resolveErr != nil {
			return resolveErr
		}
		result = val
		return nil
	})
	if err != nil {
		return 0, err
	}
	return result, nil
}

// ResolveString resolves a setting as a string.
func (s *settingsService) ResolveString(ctx context.Context, key string) (string, error) {
	val, err := s.Resolve(ctx, key)
	if err != nil {
		return "", err
	}
	if val == nil {
		return "", nil
	}
	str, ok := val.(string)
	if !ok {
		return fmt.Sprintf("%v", val), nil
	}
	return str, nil
}

// ResolveBool resolves a setting as a bool.
func (s *settingsService) ResolveBool(ctx context.Context, key string) (bool, error) {
	val, err := s.Resolve(ctx, key)
	if err != nil {
		return false, err
	}
	if val == nil {
		return false, nil
	}
	b, ok := val.(bool)
	if !ok {
		return false, &SettingsError{Op: "resolve_bool", Err: fmt.Errorf("expected bool, got %T", val)}
	}
	return b, nil
}

// ResolveInt resolves a setting as an int.
func (s *settingsService) ResolveInt(ctx context.Context, key string) (int, error) {
	val, err := s.Resolve(ctx, key)
	if err != nil {
		return 0, err
	}
	if val == nil {
		return 0, nil
	}
	switch n := val.(type) {
	case float64:
		if n != math.Floor(n) {
			return 0, &SettingsError{Op: "resolve_int", Err: fmt.Errorf("value %v has fractional part", n)}
		}
		if n > float64(math.MaxInt) || n < float64(math.MinInt) {
			return 0, &SettingsError{Op: "resolve_int", Err: fmt.Errorf("value %v out of int range", n)}
		}
		return int(n), nil
	case int:
		return n, nil
	case json.Number:
		i, err := n.Int64()
		if err != nil {
			return 0, &SettingsError{Op: "resolve_int", Err: fmt.Errorf("invalid number: %w", err)}
		}
		if i > int64(math.MaxInt) || i < int64(math.MinInt) {
			return 0, &SettingsError{Op: "resolve_int", Err: fmt.Errorf("value %v out of int range", i)}
		}
		return int(i), nil
	default:
		return 0, &SettingsError{Op: "resolve_int", Err: fmt.Errorf("expected number, got %T", val)}
	}
}

// HasTenantOverride checks if a tenant has an explicit DB override for a setting.
func (s *settingsService) HasTenantOverride(ctx context.Context, key string) (bool, error) {
	tenantID := tenant.FromContext(ctx)
	if tenantID <= 0 {
		return false, nil
	}
	sv, err := s.valueRepo.FindByTenantAndKey(ctx, tenantID, key)
	if err != nil {
		return false, &SettingsError{Op: "has_override", Err: err}
	}
	return sv != nil, nil
}

// checkWritePermission verifies the caller has the required write permission.
// If userPermissions is nil, the check is skipped (system-level callers).
// Uses wildcard-aware matching (e.g. "admin:*" grants "config:update").
func checkWritePermission(def *config.Definition, userPermissions []string) error {
	if userPermissions == nil || def.WritePermission == "" {
		return nil
	}
	if authorize.HasPermission(def.WritePermission, userPermissions) {
		return nil
	}
	return &PermissionDeniedError{Key: def.Key, RequiredPermission: def.WritePermission}
}

// SetValue sets a tenant override for a setting.
// CheckOperatorWritable enforces the operator write/reveal access policy: an
// unknown key yields *DefinitionNotFoundError, an AccessAdminOnly key yields
// ErrOperatorAdminOnly, everything else is writable.
func (s *settingsService) CheckOperatorWritable(key string) error {
	def := config.GetDefinition(key)
	if def == nil {
		return &DefinitionNotFoundError{Key: key}
	}
	if def.AccessPolicy == config.AccessAdminOnly {
		return ErrOperatorAdminOnly
	}
	return nil
}

func (s *settingsService) SetValue(ctx context.Context, key string, value any, changedBy *int64, userPermissions []string) error {
	def := config.GetDefinition(key)
	if def == nil {
		return &SettingsError{
			Op:  "set_value",
			Err: &DefinitionNotFoundError{Key: key},
		}
	}

	if err := checkWritePermission(def, userPermissions); err != nil {
		return &SettingsError{Op: "set_value", Err: err}
	}

	if err := validateValue(def, value); err != nil {
		return &SettingsError{
			Op:  "set_value",
			Err: &InvalidValueError{Key: key, Reason: err.Error()},
		}
	}

	valueJSON, err := json.Marshal(value)
	if err != nil {
		return &SettingsError{Op: "set_value", Err: fmt.Errorf("marshal value: %w", err)}
	}

	tenantID := tenant.FromContext(ctx)
	if tenantID <= 0 {
		return &SettingsError{Op: "set_value", Err: fmt.Errorf("no tenant context")}
	}

	// Cross-field invariants the per-field validation above cannot express.
	// Rejecting the pair here is what keeps an unusable configuration from ever
	// being persisted (#1565 review: an inverted Ganztag cutoff pair passed
	// FieldTime validation and then 500'd every pickup list, options and export).
	if err := s.validateCrossField(ctx, key, value); err != nil {
		return &SettingsError{
			Op:  "set_value",
			Err: &InvalidValueError{Key: key, Reason: err.Error()},
		}
	}

	// Read current value for audit
	existing, err := s.valueRepo.FindByTenantAndKey(ctx, tenantID, key)
	if err != nil {
		return &SettingsError{Op: "set_value", Err: err}
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
		return &SettingsError{Op: "set_value", Err: err}
	}

	// Audit — redact password values to avoid storing cleartext PINs
	auditOld := oldValue
	auditNew := valueJSON
	if def.Type == config.FieldPassword {
		auditOld = json.RawMessage(`"[REDACTED]"`)
		auditNew = json.RawMessage(`"[REDACTED]"`)
	}
	audit := config.NewAuditEntry(tenantID, key, "set", auditOld, auditNew, changedBy)
	if err := s.auditRepo.Create(ctx, audit); err != nil {
		s.logger.Error("failed to write audit entry",
			"key", key,
			"error", err.Error(),
		)
	}

	s.logger.Info("setting value updated",
		"key", key,
		"tenant_id", tenantID,
	)

	return nil
}

// ResetValue removes a tenant override, falling back to the registry default.
func (s *settingsService) ResetValue(ctx context.Context, key string, changedBy *int64, userPermissions []string) error {
	def := config.GetDefinition(key)
	if def == nil {
		return &SettingsError{
			Op:  "reset_value",
			Err: &DefinitionNotFoundError{Key: key},
		}
	}

	if err := checkWritePermission(def, userPermissions); err != nil {
		return &SettingsError{Op: "reset_value", Err: err}
	}

	tenantID := tenant.FromContext(ctx)
	if tenantID <= 0 {
		return &SettingsError{Op: "reset_value", Err: fmt.Errorf("no tenant context")}
	}

	// Read current value for audit
	existing, err := s.valueRepo.FindByTenantAndKey(ctx, tenantID, key)
	if err != nil {
		return &SettingsError{Op: "reset_value", Err: err}
	}
	var oldValue json.RawMessage
	if existing != nil {
		oldValue = existing.Value
	}

	if err := s.valueRepo.Delete(ctx, tenantID, key); err != nil {
		return &SettingsError{Op: "reset_value", Err: err}
	}

	// Audit — redact password values
	auditOld := oldValue
	if def.Type == config.FieldPassword {
		auditOld = json.RawMessage(`"[REDACTED]"`)
	}
	audit := config.NewAuditEntry(tenantID, key, "reset", auditOld, nil, changedBy)
	if err := s.auditRepo.Create(ctx, audit); err != nil {
		s.logger.Error("failed to write audit entry",
			"key", key,
			"error", err.Error(),
		)
	}

	s.logger.Info("setting value reset",
		"key", key,
		"tenant_id", tenantID,
	)

	return nil
}

// GetSchema delegates to the schema builder for tenant admin callers.
func (s *settingsService) GetSchema(ctx context.Context, userPermissions []string) (*SettingsSchema, error) {
	return buildSchema(ctx, s, userPermissions)
}

// GetSchemaForOperator delegates to the operator-scoped schema builder.
func (s *settingsService) GetSchemaForOperator(ctx context.Context, userPermissions []string) (*SettingsSchema, error) {
	return buildSchemaForOperator(ctx, s, userPermissions)
}

// --- Validation ---

func validateValue(def *config.Definition, value any) error {
	// Check required constraint (from validation rules)
	if def.Validation != nil && def.Validation.Required && value == nil {
		return fmt.Errorf("value is required")
	}

	// Type-specific validation
	switch def.Type {
	case config.FieldBoolean:
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("expected a boolean")
		}

	case config.FieldNumber:
		num, ok := toFloat64(value)
		if !ok {
			return fmt.Errorf("expected a number")
		}
		if num != math.Floor(num) {
			return fmt.Errorf("expected an integer, got %v", num)
		}
		if def.Validation != nil {
			if def.Validation.Min != nil && num < *def.Validation.Min {
				return fmt.Errorf("value %v is below minimum %v", num, *def.Validation.Min)
			}
			if def.Validation.Max != nil && num > *def.Validation.Max {
				return fmt.Errorf("value %v exceeds maximum %v", num, *def.Validation.Max)
			}
		}

	case config.FieldTime:
		str, ok := value.(string)
		if !ok {
			return fmt.Errorf("expected a time string")
		}
		if err := validateTimeFormat(str); err != nil {
			return err
		}

	case config.FieldDate:
		str, ok := value.(string)
		if !ok {
			return fmt.Errorf("expected a date string")
		}
		if err := validateDateFormat(str); err != nil {
			return err
		}

	case config.FieldText, config.FieldTextarea:
		if _, ok := value.(string); !ok {
			return fmt.Errorf("expected a string")
		}

	case config.FieldPassword:
		str, ok := value.(string)
		if !ok {
			return fmt.Errorf("expected a string")
		}
		// Apply pattern validation from registry definition if present
		if def.Validation != nil && def.Validation.CompiledPattern != nil && str != "" {
			if !def.Validation.CompiledPattern.MatchString(str) {
				return fmt.Errorf("value does not match required pattern")
			}
		}

	case config.FieldSelect:
		if def.Options == nil || len(def.Options.Static) == 0 {
			return fmt.Errorf("select field has no options defined")
		}
		if !isValidSelectOption(value, def.Options.Static) {
			return fmt.Errorf("value is not a valid option")
		}
	}

	return nil
}

// validateCrossField enforces invariants that span more than one setting, using
// the sibling setting's currently effective value (the new value is not yet
// persisted). There is always an edit order that reaches any valid target pair,
// so a rejection here only asks the admin to set the values in a consistent
// order — never blocks a reachable configuration.
func (s *settingsService) validateCrossField(ctx context.Context, key string, value any) error {
	switch key {
	case config.KeySlotListShortDayCutoff, config.KeySlotListLongDayCutoff:
		return s.validateSlotListCutoffPair(ctx, key, value)
	}
	return nil
}

// validateSlotListCutoffPair rejects a Ganztag cutoff pair where the long-day
// cutoff is not strictly after the short-day cutoff — the pickup buckets are
// [.., short] and (short, long], so an inverted or equal pair leaves the
// long-day bucket empty and, before this guard, made every pickup list fail.
func (s *settingsService) validateSlotListCutoffPair(ctx context.Context, key string, value any) error {
	newVal, ok := value.(string)
	if !ok || newVal == "" {
		// A non-string or empty value falls back to the registry default, which
		// is a valid pair; format errors are already caught by validateValue.
		return nil
	}
	short, long := newVal, newVal
	var err error
	if key == config.KeySlotListShortDayCutoff {
		long, err = s.ResolveString(ctx, config.KeySlotListLongDayCutoff)
	} else {
		short, err = s.ResolveString(ctx, config.KeySlotListShortDayCutoff)
	}
	if err != nil {
		return fmt.Errorf("resolve paired Ganztag cutoff: %w", err)
	}
	shortT, err := time.Parse("15:04", strings.TrimSpace(short))
	if err != nil {
		return nil // sibling is unparseable/empty; nothing to compare against yet
	}
	longT, err := time.Parse("15:04", strings.TrimSpace(long))
	if err != nil {
		return nil
	}
	if !longT.After(shortT) {
		return fmt.Errorf("der lange Ganztag (%s) muss nach dem kurzen Ganztag (%s) liegen", long, short)
	}
	return nil
}

// validateTimeFormat checks that a string is a valid HH:MM time.
func validateTimeFormat(s string) error {
	if _, err := time.Parse("15:04", strings.TrimSpace(s)); err != nil {
		return fmt.Errorf("expected time in HH:MM format")
	}
	return nil
}

func validateDateFormat(s string) error {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return nil
	}
	if _, err := time.Parse("2006-01-02", trimmed); err != nil {
		return fmt.Errorf("expected date in YYYY-MM-DD format")
	}
	return nil
}

// isValidSelectOption checks if a value matches one of the allowed select options.
func isValidSelectOption(value any, options []config.SelectOption) bool {
	vJSON, err := json.Marshal(value)
	if err != nil {
		return false
	}
	for _, opt := range options {
		optJSON, err := json.Marshal(opt.Value)
		if err != nil {
			continue
		}
		if string(vJSON) == string(optJSON) {
			return true
		}
	}
	return false
}

func toFloat64(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case json.Number:
		f, err := n.Float64()
		return f, err == nil
	default:
		return 0, false
	}
}

// --- Login image settings (stored in platform.schools.settings JSONB) ---

const loginImageKey = "loginImageUrl"

// GetLoginImageURL returns the loginImageUrl from the school's settings JSONB.
func (s *settingsService) GetLoginImageURL(ctx context.Context, tenantID int64) (string, error) {
	if tenantID <= 0 {
		return "", nil
	}

	school, err := s.schoolRepo.FindByID(ctx, tenantID)
	if err != nil {
		return "", fmt.Errorf("find school: %w", err)
	}

	settings, err := unmarshalSchoolSettings(school.Settings)
	if err != nil {
		return "", err
	}

	url, _ := settings[loginImageKey].(string)
	return url, nil
}

// SetLoginImageURL sets the loginImageUrl in the school's settings JSONB.
func (s *settingsService) SetLoginImageURL(ctx context.Context, tenantID int64, imageURL string) (oldURL string, err error) {
	return s.updateSchoolSetting(ctx, tenantID, &imageURL)
}

// ClearLoginImageURL removes the loginImageUrl from the school's settings JSONB.
func (s *settingsService) ClearLoginImageURL(ctx context.Context, tenantID int64) (oldURL string, err error) {
	return s.updateSchoolSetting(ctx, tenantID, nil)
}

// updateSchoolSetting updates the loginImageUrl in the school's settings JSONB.
// Uses WithAdminTx because platform.schools requires the phoenix_admin role.
// Pass nil imageURL to remove the key.
func (s *settingsService) updateSchoolSetting(ctx context.Context, tenantID int64, imageURL *string) (oldURL string, err error) {
	err = tenant.WithAdminTx(ctx, s.db, func(adminCtx context.Context, _ bun.Tx) error {
		// Use FOR UPDATE lock to serialize concurrent read-modify-write on the JSONB settings.
		// FOR SHARE is insufficient here: two transactions could both acquire the shared lock,
		// read the same JSONB, and then deadlock when both try to UPDATE the row.
		school, findErr := s.schoolRepo.FindByIDForUpdate(adminCtx, tenantID)
		if findErr != nil {
			return fmt.Errorf("find school: %w", findErr)
		}

		settings, unmarshalErr := unmarshalSchoolSettings(school.Settings)
		if unmarshalErr != nil {
			return unmarshalErr
		}

		oldURL, _ = settings[loginImageKey].(string)

		if imageURL != nil {
			settings[loginImageKey] = *imageURL
		} else {
			delete(settings, loginImageKey)
		}

		settingsJSON, marshalErr := json.Marshal(settings)
		if marshalErr != nil {
			return fmt.Errorf("marshal settings: %w", marshalErr)
		}
		school.Settings = string(settingsJSON)

		return s.schoolRepo.Update(adminCtx, school)
	})
	return oldURL, err
}

// unmarshalSchoolSettings parses the school's settings JSONB string into a map.
// Empty or missing settings return an empty map. Corrupt JSON returns an error
// to prevent silent data loss on subsequent writes.
func unmarshalSchoolSettings(raw string) (map[string]interface{}, error) {
	if raw == "" || raw == "{}" || raw == "null" {
		return make(map[string]interface{}), nil
	}
	var settings map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &settings); err != nil {
		return nil, fmt.Errorf("corrupt school settings JSON: %w", err)
	}
	// json.Unmarshal into map returns nil for the JSON literal "null".
	// Coerce to an empty map so callers can safely assign keys.
	if settings == nil {
		return make(map[string]interface{}), nil
	}
	return settings, nil
}
