package config

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"maps"
	"math"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
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
	// classRestrictionGuard, when set, reports whether the tenant in
	// context currently has an active enrollment phase that restricts
	// eligibility to specific school classes. It gates disabling the
	// concrete-class collection toggles so an admin cannot make every
	// submission to such a phase fail (#1663). Optional: nil skips the
	// guard (unit tests, callers that never wire enrollment). Injected via
	// SetClassRestrictionGuard to keep the config package decoupled from
	// the enrollment domain.
	classRestrictionGuard func(ctx context.Context) (bool, error)
	// gradeRestrictionGuard is the same probe for phases restricted to
	// specific GRADE LEVELS. Separate from classRestrictionGuard because the
	// two hang off different toggles: a grade restriction only needs
	// collect_grade_level, so disabling collect_school_class must not be
	// blocked by it (#1663). Optional in the same way; injected via
	// SetGradeRestrictionGuard.
	gradeRestrictionGuard func(ctx context.Context) (bool, error)
	// gradeCapGuard reports the highest grade any active phase restricts
	// itself to (0 when none does). It gates LOWERING
	// enrollment.grade_level_max: the public form only offers grades up to the
	// cap and the submit path re-checks it, so a cap below a live restriction
	// leaves that phase unable to accept any submission (#1663). Optional in
	// the same way as the two probes above; injected via SetGradeCapGuard.
	gradeCapGuard func(ctx context.Context) (int, error)
}

// SetClassRestrictionGuard wires the enrollment class-restriction probe used
// by the concrete-class collection guard. Kept off the constructor (and off
// the SettingsService interface) so existing call sites and tests are
// unaffected; the factory sets it after construction (#1663).
func (s *settingsService) SetClassRestrictionGuard(fn func(ctx context.Context) (bool, error)) {
	s.classRestrictionGuard = fn
}

// SetGradeRestrictionGuard wires the enrollment grade-restriction probe used
// by the grade-level collection guard, following the same off-constructor
// convention as SetClassRestrictionGuard (#1663).
func (s *settingsService) SetGradeRestrictionGuard(fn func(ctx context.Context) (bool, error)) {
	s.gradeRestrictionGuard = fn
}

// SetGradeCapGuard wires the enrollment highest-restricted-grade probe used by
// the grade-level-cap guard, following the same off-constructor convention as
// SetClassRestrictionGuard (#1663).
func (s *settingsService) SetGradeCapGuard(fn func(ctx context.Context) (int, error)) {
	s.gradeCapGuard = fn
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
	snapshot, err := s.ResolveMany(ctx, []string{key})
	if err != nil {
		return nil, err
	}
	return snapshot.Value(key)
}

// ResolveMany resolves several settings in one repository query. A compatible
// context snapshot wins, which lets scheduler-invoked downstream services
// share the minute snapshot without knowing about scheduler internals. Below
// the explicit snapshot sits the request-scoped memo cache (issue #2065):
// within one request every (tenant_id, key) pair is loaded from PostgreSQL at
// most once; only cache-missing keys reach the repository.
func (s *settingsService) ResolveMany(ctx context.Context, keys []string) (*SettingsSnapshot, error) {
	tenantID := tenant.FromContext(ctx)
	if snapshot := snapshotFromContext(ctx, tenantID, keys); snapshot != nil {
		// Explicit snapshots are returned as-is and deliberately NEVER copied
		// into the request cache: the scheduler's minute snapshot may be up to
		// a minute old, and promoting its values into request scope would
		// leak that staleness into paths that expect request freshness.
		return snapshot, nil
	}

	if tenantID <= 0 || len(keys) == 0 {
		return newSettingsSnapshot(tenantID, keys, nil)
	}

	// Validate every key up front so an unknown key fails deterministically
	// regardless of which keys happen to be cached already.
	for _, key := range keys {
		if config.GetDefinition(key) == nil {
			return nil, &SettingsError{
				Op:  "resolve_many",
				Err: &DefinitionNotFoundError{Key: key},
			}
		}
	}

	cache := requestCacheFromContext(ctx)
	if cache == nil {
		stored, err := s.valueRepo.FindByTenantAndKeys(ctx, tenantID, keys)
		if err != nil {
			return nil, &SettingsError{Op: "resolve_many", Err: err}
		}
		return newSettingsSnapshot(tenantID, keys, stored)
	}

	resolved, missing := cache.lookup(tenantID, keys)
	if len(missing) > 0 {
		stored, err := s.valueRepo.FindByTenantAndKeys(ctx, tenantID, missing)
		if err != nil {
			return nil, &SettingsError{Op: "resolve_many", Err: err}
		}
		loaded, err := newSettingsSnapshot(tenantID, missing, stored)
		if err != nil {
			return nil, err
		}
		cache.store(tenantID, loaded.values)
		maps.Copy(resolved, loaded.values)
	}
	return newSettingsSnapshotFromValues(tenantID, resolved), nil
}

// ResolveManyForTenant resolves several settings inside one tenant
// transaction. A matching context snapshot avoids both the transaction and
// query when a scheduler already prefetched the values, and a full
// request-cache hit skips the tenant transaction entirely (issue #2065).
func (s *settingsService) ResolveManyForTenant(ctx context.Context, tenantID int64, keys []string) (*SettingsSnapshot, error) {
	if snapshot := snapshotFromContext(ctx, tenantID, keys); snapshot != nil {
		return snapshot, nil
	}

	// Consult the request cache only when the ambient tenant context is absent
	// or matches tenantID: a mismatched ambient tenant must still reach
	// WithTenantTx below so its nested-transaction guard surfaces the wiring
	// bug instead of the cache silently succeeding.
	if ctxTenant := tenant.FromContext(ctx); tenantID > 0 && len(keys) > 0 &&
		(ctxTenant == 0 || ctxTenant == tenantID) {
		if cache := requestCacheFromContext(ctx); cache != nil {
			for _, key := range keys {
				if config.GetDefinition(key) == nil {
					return nil, &SettingsError{
						Op:  "resolve_many",
						Err: &DefinitionNotFoundError{Key: key},
					}
				}
			}
			if resolved, missing := cache.lookup(tenantID, keys); len(missing) == 0 {
				return newSettingsSnapshotFromValues(tenantID, resolved), nil
			}
		}
	}

	var result *SettingsSnapshot
	err := tenant.WithTenantTx(ctx, s.db, tenantID, func(txCtx context.Context, _ bun.Tx) error {
		var resolveErr error
		result, resolveErr = s.ResolveMany(txCtx, keys)
		return resolveErr
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// ResolveManyForTenants resolves all tenant/key pairs in one privileged query.
// Registry defaults are filled independently for every requested tenant.
// Deliberately NOT request-cached: its only callers are the scheduler (which
// owns the minute snapshot) and other admin-tx batch paths that already
// coalesce their reads.
func (s *settingsService) ResolveManyForTenants(ctx context.Context, tenantIDs []int64, keys []string) (map[int64]*SettingsSnapshot, error) {
	uniqueTenantIDs := make([]int64, 0, len(tenantIDs))
	seenTenantIDs := make(map[int64]struct{}, len(tenantIDs))
	for _, tenantID := range tenantIDs {
		if tenantID <= 0 {
			continue
		}
		if _, seen := seenTenantIDs[tenantID]; seen {
			continue
		}
		seenTenantIDs[tenantID] = struct{}{}
		uniqueTenantIDs = append(uniqueTenantIDs, tenantID)
	}

	result := make(map[int64]*SettingsSnapshot, len(uniqueTenantIDs))
	if len(uniqueTenantIDs) == 0 {
		return result, nil
	}

	var stored []*config.SettingValue
	if len(keys) > 0 {
		err := tenant.WithAdminTx(ctx, s.db, func(txCtx context.Context, _ bun.Tx) error {
			var findErr error
			stored, findErr = s.valueRepo.FindByTenantsAndKeys(txCtx, uniqueTenantIDs, keys)
			return findErr
		})
		if err != nil {
			return nil, &SettingsError{Op: "resolve_many_for_tenants", Err: err}
		}
	}

	storedByTenant := make(map[int64][]*config.SettingValue, len(uniqueTenantIDs))
	for _, value := range stored {
		if value == nil {
			continue
		}
		tenantID := value.GetTenantID()
		if _, requested := seenTenantIDs[tenantID]; requested {
			storedByTenant[tenantID] = append(storedByTenant[tenantID], value)
		}
	}
	for _, tenantID := range uniqueTenantIDs {
		snapshot, err := newSettingsSnapshot(tenantID, keys, storedByTenant[tenantID])
		if err != nil {
			return nil, err
		}
		result[tenantID] = snapshot
	}
	return result, nil
}

// ResolveStringForTenant resolves a setting as a string for a specific tenant,
// wrapping the query in a tenant transaction to satisfy RLS.
func (s *settingsService) ResolveStringForTenant(ctx context.Context, tenantID int64, key string) (string, error) {
	snapshot, err := s.ResolveManyForTenant(ctx, tenantID, []string{key})
	if err != nil {
		return "", err
	}
	return snapshot.String(key)
}

// ResolveStringForTenantInTx resolves a setting as a string on the caller's
// already-open transaction, deliberately consulting neither the context
// snapshot nor the request-scoped memo cache — see the interface doc for why
// that matters and who needs it.
//
// It opens no transaction of its own: the repository read runs on the ambient
// tx via base.GetDB, so the value is the committed state seen by a statement
// of THAT transaction, and the caller's locks still hold when it is used. The
// alternative — resolving through ResolveManyForTenant, which opens its own
// transaction — would take a second pooled connection while the caller holds
// one, which is a deadlock waiting for a saturated pool.
func (s *settingsService) ResolveStringForTenantInTx(ctx context.Context, tenantID int64, key string) (string, error) {
	if tenantID <= 0 {
		return "", &SettingsError{
			Op:  "resolve_in_tx",
			Err: fmt.Errorf("tenant id is required to resolve %q inside a transaction", key),
		}
	}
	if tx, ok := modelBase.TxFromContext(ctx); !ok || tx == nil {
		return "", &SettingsError{
			Op:  "resolve_in_tx",
			Err: fmt.Errorf("resolving %q inside a transaction requires an ambient transaction", key),
		}
	}
	if config.GetDefinition(key) == nil {
		return "", &SettingsError{Op: "resolve_in_tx", Err: &DefinitionNotFoundError{Key: key}}
	}

	stored, err := s.valueRepo.FindByTenantAndKeys(ctx, tenantID, []string{key})
	if err != nil {
		return "", &SettingsError{Op: "resolve_in_tx", Err: err}
	}
	snapshot, err := newSettingsSnapshot(tenantID, []string{key}, stored)
	if err != nil {
		return "", err
	}
	return snapshot.String(key)
}

// ResolveBoolForTenant resolves a setting as a bool for a specific tenant,
// wrapping the query in a tenant transaction to satisfy RLS. Required from
// call sites that run outside TenantTxMiddleware (e.g. /auth/mfa/verify)
// but already know which tenant they're acting on.
func (s *settingsService) ResolveBoolForTenant(ctx context.Context, tenantID int64, key string) (bool, error) {
	snapshot, err := s.ResolveManyForTenant(ctx, tenantID, []string{key})
	if err != nil {
		return false, err
	}
	return snapshot.Bool(key)
}

// ResolveIntForTenant mirrors ResolveBoolForTenant but for integer settings.
func (s *settingsService) ResolveIntForTenant(ctx context.Context, tenantID int64, key string) (int, error) {
	snapshot, err := s.ResolveManyForTenant(ctx, tenantID, []string{key})
	if err != nil {
		return 0, err
	}
	return snapshot.Int(key)
}

// ResolveString resolves a setting as a string.
func (s *settingsService) ResolveString(ctx context.Context, key string) (string, error) {
	snapshot, err := s.ResolveMany(ctx, []string{key})
	if err != nil {
		return "", err
	}
	return snapshot.String(key)
}

// ResolveBool resolves a setting as a bool.
func (s *settingsService) ResolveBool(ctx context.Context, key string) (bool, error) {
	snapshot, err := s.ResolveMany(ctx, []string{key})
	if err != nil {
		return false, err
	}
	return snapshot.Bool(key)
}

// ResolveBools resolves multiple boolean settings while loading all tenant
// overrides in one query. Registry defaults are filled first, then explicit
// tenant values replace them.
func (s *settingsService) ResolveBools(ctx context.Context, keys []string) (map[string]bool, error) {
	snapshot, err := s.ResolveMany(ctx, keys)
	if err != nil {
		return nil, err
	}
	resolved := make(map[string]bool, len(keys))
	for _, key := range keys {
		if _, seen := resolved[key]; seen {
			continue
		}
		value, resolveErr := snapshot.Bool(key)
		if resolveErr != nil {
			return nil, resolveErr
		}
		resolved[key] = value
	}
	return resolved, nil
}

// ResolveInt resolves a setting as an int.
func (s *settingsService) ResolveInt(ctx context.Context, key string) (int, error) {
	snapshot, err := s.ResolveMany(ctx, []string{key})
	if err != nil {
		return 0, err
	}
	return snapshot.Int(key)
}

// HasTenantOverride checks if a tenant has an explicit DB override for a setting.
func (s *settingsService) HasTenantOverride(ctx context.Context, key string) (bool, error) {
	snapshot, err := s.ResolveMany(ctx, []string{key})
	if err != nil {
		return false, err
	}
	return snapshot.HasOverride(key)
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

	// Serialize a security.mfa_mode change against session mints that are
	// re-deciding their MFA gate right now — see lockMFAPolicy.
	if err := s.lockMFAPolicyForWrite(ctx, key); err != nil {
		return &SettingsError{Op: "set_value", Err: err}
	}

	// Cross-field invariants the per-field validation above cannot express.
	// Rejecting the pair here is what keeps an unusable configuration from ever
	// being persisted (#1565 review: an inverted Ganztag cutoff pair passed
	// FieldTime validation and then 500'd every pickup list, options and export).
	if err := s.validateCrossField(ctx, key, value); err != nil {
		// validateCrossField returns *InvalidValueError only for a genuinely
		// invalid pair (→ 400); lock/lookup failures stay operational (→ 500).
		return &SettingsError{Op: "set_value", Err: err}
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

	// Evict the written key from the request cache so same-request reads —
	// including side-effect hooks running inside this transaction — see the
	// new value instead of a memoized pre-write entry.
	if cache := requestCacheFromContext(ctx); cache != nil {
		cache.evictKey(tenantID, key)
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

	// A reset restores the registry default, which changes the effective mode
	// exactly like a write does — same lock, same reason (see lockMFAPolicy).
	if err := s.lockMFAPolicyForWrite(ctx, key); err != nil {
		return &SettingsError{Op: "reset_value", Err: err}
	}

	// Resetting restores the registry default, which can invert a cross-field
	// invariant even though SetValue guards it (e.g. resetting the long Ganztag
	// cutoff to its 16:00 default while a 18:00 short-cutoff override stays put).
	// Validate the effective pair with the default in place before deleting, so a
	// reset can never persist an unusable configuration that then 500s every
	// pickup list (#1565 review).
	if err := s.validateCrossField(ctx, key, def.Default); err != nil {
		// See SetValue: only an inverted pair is a 400; operational failures 500.
		return &SettingsError{Op: "reset_value", Err: err}
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

	// See SetValue: same-request reads must observe the reset immediately.
	if cache := requestCacheFromContext(ctx); cache != nil {
		cache.evictKey(tenantID, key)
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

	case config.FieldText, config.FieldTextarea, config.FieldPassword:
		str, ok := value.(string)
		if !ok {
			return fmt.Errorf("expected a string")
		}
		if def.Validation != nil &&
			def.Validation.CompiledPattern != nil &&
			(str != "" || !def.Validation.AllowEmpty) {
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
	case config.KeyEnrollmentCollectGradeLevel, config.KeyEnrollmentCollectSchoolClass:
		return s.validateClassCollectionGuard(ctx, key, value)
	case config.KeyEnrollmentGradeLevelMax:
		return s.validateGradeLevelCapGuard(ctx, key, value)
	}
	return nil
}

// validateGradeLevelCapGuard blocks lowering enrollment.grade_level_max below
// a grade an active phase already restricts itself to. The public form offers
// grades 1..cap and the submit path re-checks the cap, so such a phase would
// end up unable to accept any submission: the parent cannot pick the eligible
// grade, and a hand-crafted submission carrying it is rejected by the cap.
// This is the inverse of the phase-side ensureEligibleGradeLevelsWithinTenantCap
// check — together they keep the pair consistent from either edit direction,
// including via ResetValue, which restores the registry default 4 (#1663).
//
// Takes the same per-tenant lock as the class/grade collection guards so a
// concurrent phase write and a cap change cannot both pass on a stale read.
// Best-effort: a nil guard skips the check.
func (s *settingsService) validateGradeLevelCapGuard(ctx context.Context, key string, value any) error {
	if s.gradeCapGuard == nil {
		return nil
	}
	num, ok := toFloat64(value)
	if !ok {
		// A non-numeric value is already rejected by validateValue; nothing to
		// compare here.
		return nil
	}
	newMax := int(num)
	if err := s.LockClassCollectionPair(ctx); err != nil {
		return err
	}
	highest, err := s.gradeCapGuard(ctx)
	if err != nil {
		// Operational failure stays a plain error → 500, not a "your value is
		// invalid" 400.
		return fmt.Errorf("check grade-restricted phases: %w", err)
	}
	if highest > newMax {
		return &InvalidValueError{
			Key: key,
			Reason: fmt.Sprintf(
				"Die höchste Klassenstufe kann nicht auf %d gesenkt werden, solange eine aktive Anmeldephase auf Klassenstufe %d beschränkt ist. Entfernen Sie zuerst die Klassenstufenbeschränkung der betroffenen Phase.",
				newMax,
				highest,
			),
		}
	}
	return nil
}

// validateClassCollectionGuard blocks disabling concrete-class collection
// while an active enrollment phase restricts eligibility to specific school
// classes. Concrete-class collection is effective only when BOTH
// collect_grade_level and collect_school_class are on (a class without its
// grade is ambiguous). When it turns off, the submit path forces every
// child's class to nil, so a phase with a non-empty eligible_school_classes
// gate rejects every submission with class_not_eligible. This is the inverse
// of the phase-side validateEligibleClassesCollectable check: together they
// keep the restricted-phase / class-collection pair consistent from either
// edit direction (#1663). Best-effort: nil guard skips the check.
//
// It guards the grade-level restriction on the same lock: an active phase
// limited to whole grades survives collect_school_class being off, but not
// collect_grade_level.
func (s *settingsService) validateClassCollectionGuard(ctx context.Context, key string, value any) error {
	if s.classRestrictionGuard == nil && s.gradeRestrictionGuard == nil {
		return nil
	}
	newVal, ok := value.(bool)
	if !ok {
		// Non-bool falls back to the registry default / is caught by
		// validateValue; nothing to compare here.
		return nil
	}
	// Serialize the read-validate-write of this invariant against a concurrent
	// activation of a class-restricted phase. The phase side takes the same
	// per-tenant lock before it resolves these toggles, so the second writer
	// blocks and observes the first's committed state instead of both passing
	// on a stale read (#1663). Lock first, then read the sibling toggle and the
	// restricted-phase probe below. Held until the setting upsert commits with
	// the request tx.
	if err := s.LockClassCollectionPair(ctx); err != nil {
		return err
	}
	// Grade-level restrictions hang off collect_grade_level ALONE — a phase
	// targeting a whole grade never needs concrete classes — so this check runs
	// only when the write itself turns grade collection off, and independently
	// of the class-collection pair below (#1663).
	if key == config.KeyEnrollmentCollectGradeLevel && !newVal && s.gradeRestrictionGuard != nil {
		restricted, err := s.gradeRestrictionGuard(ctx)
		if err != nil {
			return fmt.Errorf("check grade-restricted phases: %w", err)
		}
		if restricted {
			return &InvalidValueError{
				Key:    key,
				Reason: "Die Klassenstufen-Abfrage kann nicht deaktiviert werden, solange eine aktive Anmeldephase auf bestimmte Klassenstufen beschränkt ist. Entfernen Sie zuerst die Klassenstufenbeschränkung der betroffenen Phase.",
			}
		}
	}
	if s.classRestrictionGuard == nil {
		return nil
	}
	// Compute the effective collection state with `value` applied to `key`
	// and the sibling toggle resolved live (its current effective value).
	collectGrade, collectClass := newVal, newVal
	var err error
	if key == config.KeyEnrollmentCollectGradeLevel {
		collectClass, err = s.ResolveBool(ctx, config.KeyEnrollmentCollectSchoolClass)
	} else {
		collectGrade, err = s.ResolveBool(ctx, config.KeyEnrollmentCollectGradeLevel)
	}
	if err != nil {
		return fmt.Errorf("resolve paired class-collection toggle: %w", err)
	}
	if collectGrade && collectClass {
		// Collection stays effective — no restricted phase is endangered.
		return nil
	}
	restricted, err := s.classRestrictionGuard(ctx)
	if err != nil {
		// Operational failure stays a plain error → surfaces as a 500, not a
		// "your value is invalid" 400.
		return fmt.Errorf("check class-restricted phases: %w", err)
	}
	if restricted {
		return &InvalidValueError{
			Key:    key,
			Reason: "Die Klassen-Abfrage kann nicht deaktiviert werden, solange eine aktive Anmeldephase auf bestimmte Klassen beschränkt ist. Entfernen Sie zuerst die Klassenbeschränkung der betroffenen Phase.",
		}
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
	// Serialize the read-validate-write of the cutoff pair against a concurrent
	// write of the sibling cutoff (#1565 review). Without this, two requests that
	// both start from a valid pair each read the OLD sibling, both pass this
	// check, and then commit an inverted pair (e.g. short=15:30 alongside
	// long=15:00) that 500s every pickup list, preview and export. The
	// transaction-scoped advisory lock — held until the request tx commits after
	// the upsert — forces the second writer to block, then read the first's
	// committed value and reject.
	if err := s.LockSlotListCutoffPair(ctx); err != nil {
		return err
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
		// Only the inverted pair is a client validation error (→ 400). The lock
		// and sibling-resolve failures above are operational and stay plain
		// errors so they surface as a 500, not as "your time is invalid"
		// (#1565 review).
		return &InvalidValueError{
			Key:    key,
			Reason: fmt.Sprintf("der lange Ganztag (%s) muss nach dem kurzen Ganztag (%s) liegen", long, short),
		}
	}
	return nil
}

// LockSlotListCutoffPair takes the per-tenant transaction-scoped advisory lock
// that guards the Ganztag pickup-cutoff pair. Both the pair validator on the
// write path (validateSlotListCutoffPair) and the slot-list reader on the read
// path (services/slotlists pickupBuckets) take it, so a concurrent lowering of
// both cutoffs cannot interleave with a read and expose an inverted short/long
// pair under READ COMMITTED (#1565 review). Best-effort: without an ambient
// transaction the xact lock is meaningless — the settings read/write paths always
// run inside a tenant tx (the RLS-scoped repos require one) — so it is skipped
// rather than failing.
// Taking the lock is also the freshness barrier of the request cache: the
// tenant bucket is flushed (before the no-tx early return, so tests without an
// ambient transaction behave like production) so post-lock reads hit the
// database and observe the concurrent writer's committed state.
func (s *settingsService) LockSlotListCutoffPair(ctx context.Context) error {
	s.flushRequestCacheForLock(ctx)
	if _, hasTx := modelBase.TxFromContext(ctx); !hasTx {
		return nil
	}
	if err := base.AcquireXactLock(ctx, s.db, slotListCutoffLockKey(ctx)); err != nil {
		return fmt.Errorf("lock Ganztag cutoff pair: %w", err)
	}
	return nil
}

// flushRequestCacheForLock drops every request-cache entry for the ambient
// tenant. All Lock* helpers call it first: which keys a cross-field guard
// re-reads under its lock is deliberately not enumerated here — a whole-tenant
// flush stays correct when the next guard adds another key (#1565/#1663).
func (s *settingsService) flushRequestCacheForLock(ctx context.Context) {
	if cache := requestCacheFromContext(ctx); cache != nil {
		cache.evictTenant(tenant.FromContext(ctx))
	}
}

// LockSlotListCutoffPairShared takes the SHARED variant of the Ganztag cutoff
// lock for read paths (services/slotlists pickupBuckets). Shared holders never
// block one another, so concurrent /options, pickup-preview and export requests
// no longer serialize behind a single exclusive lock — a slow export scanning
// rosters or rendering a PDF/XLSX cannot stall every other reader in the tenant.
// It still conflicts with the exclusive writer lock, so a cutoff update can never
// commit a partial pair while a reader observes the two cutoffs (#1565 review).
// Flushes the tenant's request-cache bucket like the exclusive variant — the
// slotlists reader resolves both cutoffs after taking this lock and must see
// the writer's committed pair, not memoized pre-lock values.
func (s *settingsService) LockSlotListCutoffPairShared(ctx context.Context) error {
	s.flushRequestCacheForLock(ctx)
	if _, hasTx := modelBase.TxFromContext(ctx); !hasTx {
		return nil
	}
	if err := base.AcquireXactLockShared(ctx, s.db, slotListCutoffLockKey(ctx)); err != nil {
		return fmt.Errorf("lock Ganztag cutoff pair (shared): %w", err)
	}
	return nil
}

// slotListCutoffLockKey is the per-tenant advisory-lock key shared by the
// exclusive writer lock and the shared reader lock so the two conflict.
func slotListCutoffLockKey(ctx context.Context) string {
	return fmt.Sprintf("slot-list-cutoff:%d", tenant.FromContext(ctx))
}

// LockClassCollectionPair takes the per-tenant transaction-scoped advisory lock
// that guards the enrollment class-restriction / class-collection invariant.
// Two writes can otherwise race: one disabling concrete-class collection
// (validateClassCollectionGuard here) and one activating a class-restricted
// phase (validateEligibleClassesCollectable in services/enrollment). Under READ
// COMMITTED each reads the other's pre-commit state, both pass, and they commit
// an active restricted phase with class collection off — every submission then
// fails class_not_eligible. Both sides take THIS lock on the same key, so the
// second writer blocks, re-reads the first's committed state, and rejects.
// Best-effort: without an ambient transaction the xact lock is meaningless (the
// settings and phase write paths always run inside a tenant tx), so it is
// skipped rather than failing.
// Flushes the tenant's request-cache bucket first: both the settings-side
// guard and the enrollment-side phase guard re-read settings (including
// enrollment.grade_level_max) under this lock and must observe committed
// state, not memoized pre-lock values.
func (s *settingsService) LockClassCollectionPair(ctx context.Context) error {
	s.flushRequestCacheForLock(ctx)
	if _, hasTx := modelBase.TxFromContext(ctx); !hasTx {
		return nil
	}
	if err := base.AcquireXactLock(ctx, s.db, classCollectionLockKey(ctx)); err != nil {
		return fmt.Errorf("lock class-collection pair: %w", err)
	}
	return nil
}

// classCollectionLockKey is the per-tenant advisory-lock key shared by the
// settings-side class-collection guard and the enrollment-side phase
// eligibility guard so the two conflict.
func classCollectionLockKey(ctx context.Context) string {
	return fmt.Sprintf("enrollment-class-collection:%d", tenant.FromContext(ctx))
}

// LockMFAPolicy takes the exclusive per-tenant advisory lock on
// security.mfa_mode for the WRITE side — see the interface doc and
// lockMFAPolicy.
func (s *settingsService) LockMFAPolicy(ctx context.Context) error {
	return s.lockMFAPolicy(ctx, tenant.FromContext(ctx), false)
}

// LockMFAPolicySharedForTenant takes the shared variant for the token-mint READ
// side, with the tenant passed explicitly because the login flows have no
// tenant context.
func (s *settingsService) LockMFAPolicySharedForTenant(ctx context.Context, tenantID int64) error {
	return s.lockMFAPolicy(ctx, tenantID, true)
}

// lockMFAPolicy is the shared body of the two MFA-policy lock helpers.
//
// Without it, "is MFA required for this session?" is a read that commits
// nothing: a school login resolves security.mfa_mode as off, an admin switches
// it to required and commits, and the login — which has not written its refresh
// token yet — still mints a session that never saw a second factor (#2207
// review). Re-reading the mode inside the mint transaction narrows that window
// but cannot close it: under READ COMMITTED the re-read simply happens a few
// statements earlier than the concurrent write. Only a lock the two sides share
// orders them, so either the writer waits for the mint to commit (and the next
// login sees the new mode) or the mint's re-read observes the committed value
// and refuses.
//
// Best-effort like the sibling helpers: without an ambient transaction an xact
// lock is meaningless — both sides always run in one — so it is skipped rather
// than failing. A tenant-less caller (registry default, no per-school mode to
// guard) is skipped for the same reason.
func (s *settingsService) lockMFAPolicy(ctx context.Context, tenantID int64, shared bool) error {
	// Same freshness barrier as the other Lock* helpers: post-lock reads must
	// hit the database instead of replaying a memoized pre-lock value.
	if cache := requestCacheFromContext(ctx); cache != nil {
		cache.evictTenant(tenantID)
	}
	if tenantID <= 0 {
		return nil
	}
	if _, hasTx := modelBase.TxFromContext(ctx); !hasTx {
		return nil
	}
	acquire := base.AcquireXactLock
	if shared {
		acquire = base.AcquireXactLockShared
	}
	if err := acquire(ctx, s.db, mfaPolicyLockKey(tenantID)); err != nil {
		return fmt.Errorf("lock mfa policy: %w", err)
	}
	return nil
}

// mfaPolicyLockKey is the per-tenant advisory-lock key shared by the
// security.mfa_mode writer and the token mints that re-decide the MFA gate, so
// the two conflict. Per tenant: one school's admin must not serialize another
// school's logins.
func mfaPolicyLockKey(tenantID int64) string {
	return fmt.Sprintf("mfa-policy:%d", tenantID)
}

// lockMFAPolicyForWrite takes the exclusive policy lock when the key being
// written is one a session mint reads. Called by SetValue and ResetValue
// BEFORE they read the current value, so the whole read-modify-write is
// ordered against an in-flight mint rather than just the final upsert.
func (s *settingsService) lockMFAPolicyForWrite(ctx context.Context, key string) error {
	if key != config.KeyMFAMode {
		return nil
	}
	return s.LockMFAPolicy(ctx)
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
