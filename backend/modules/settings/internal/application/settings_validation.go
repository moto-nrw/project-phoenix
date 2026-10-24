package config

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	config "github.com/moto-nrw/project-phoenix/modules/settings/records"
)

func validateValue(def *config.Definition, value any) error {
	if def.Validation != nil && def.Validation.Required && value == nil {
		return fmt.Errorf("value is required")
	}
	switch def.Type {
	case config.FieldBoolean:
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("expected a boolean")
		}
	case config.FieldNumber:
		return validateNumberValue(def, value)
	case config.FieldTime:
		return validateTimeValue(def, value)
	case config.FieldDate:
		str, ok := value.(string)
		if !ok {
			return fmt.Errorf("expected a date string")
		}
		return validateDateFormat(str)
	case config.FieldText, config.FieldTextarea, config.FieldPassword:
		return validateTextValue(def, value)
	case config.FieldSelect:
		if def.Options == nil || len(def.Options.Static) == 0 {
			return fmt.Errorf("select field has no options defined")
		}
		if !isValidSelectOption(value, def.Options.Static) {
			return fmt.Errorf("value is not a valid option")
		}
	case config.FieldMultiSelect:
		return validateMultiSelectValue(def, value)
	}
	return nil
}

// validateMultiSelectValue accepts an array of distinct option values; an
// empty array selects nothing.
func validateMultiSelectValue(def *config.Definition, value any) error {
	if def.Options == nil || len(def.Options.Static) == 0 {
		return fmt.Errorf("select field has no options defined")
	}
	var items []any
	switch v := value.(type) {
	case []any:
		items = v
	case []string:
		for _, item := range v {
			items = append(items, item)
		}
	default:
		return fmt.Errorf("expected a list of options")
	}
	seen := make(map[string]bool, len(items))
	for _, item := range items {
		if !isValidSelectOption(item, def.Options.Static) {
			return fmt.Errorf("value is not a valid option")
		}
		key := fmt.Sprint(item)
		if seen[key] {
			return fmt.Errorf("option %v is selected twice", item)
		}
		seen[key] = true
	}
	return nil
}

func validateNumberValue(def *config.Definition, value any) error {
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

	return nil
}

func validateTimeValue(def *config.Definition, value any) error {
	str, ok := value.(string)
	if !ok {
		return fmt.Errorf("expected a time string")
	}
	defaultValue, optional := def.Default.(string)
	if optional && defaultValue == "" && strings.TrimSpace(str) == "" {
		return nil
	}
	if err := validateTimeFormat(str); err != nil {
		return err
	}

	return nil
}

func validateTextValue(def *config.Definition, value any) error {
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

	return nil
}

// validateCrossField enforces invariants that span more than one setting, using
// the sibling setting's currently effective value (the new value is not yet
// persisted). There is always an edit order that reaches any valid target pair,
// so a rejection here only asks the admin to set the values in a consistent
// order — never blocks a reachable configuration.
func (s *settingsService) validateCrossField(ctx context.Context, key string, value any) error {
	if module, ok := config.ModuleForSettingKey(key); ok {
		return s.validateModuleBooking(ctx, key, module, value)
	}
	switch key {
	case config.KeyOperationalOverviewScope, config.KeyAttendanceEditScope, config.KeyBlockStartScope, config.KeyBlockCompleteScope:
		return s.validateOverviewScopeDependents(ctx, key, value)
	case config.KeySlotListShortDayCutoff, config.KeySlotListLongDayCutoff:
		return s.validateSlotListCutoffPair(ctx, key, value)
	case config.KeyEnrollmentCollectGradeLevel, config.KeyEnrollmentCollectSchoolClass:
		return s.validateClassCollectionGuard(ctx, key, value)
	case config.KeyEnrollmentGradeLevelMax:
		return s.validateGradeLevelCapGuard(ctx, key, value)
	case config.KeyAttendanceVisibleDays, config.KeyGDPRAttendanceRetentionDays:
		return s.validateAttendanceWindows(ctx, key, value)
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
		return &config.InvalidValueError{
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
	if err := s.validateGradeCollectionDisabled(ctx, key, newVal); err != nil {
		return err
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
		return &config.InvalidValueError{
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
		return &config.InvalidValueError{
			Key:    key,
			Reason: fmt.Sprintf("der lange Ganztag (%s) muss nach dem kurzen Ganztag (%s) liegen", long, short),
		}
	}
	return nil
}

// LockSlotListCutoffPair takes the per-tenant transaction-scoped advisory lock
// that guards the Ganztag pickup-cutoff pair. Both the pair validator on the
// write path (validateSlotListCutoffPair) and the slot-list reader on the read
// path (the class-day projection, modules/classday) take it, so a concurrent lowering of
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
	if s.runtime == nil || !s.runtime.HasTransaction(ctx) {
		return nil
	}
	if err := s.runtime.AcquireLock(ctx, slotListCutoffLockKey(s.tenantID(ctx)), false); err != nil {
		return fmt.Errorf("lock Ganztag cutoff pair: %w", err)
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

func (s *settingsService) validateGradeCollectionDisabled(ctx context.Context, key string, newVal bool) error {
	if key == config.KeyEnrollmentCollectGradeLevel && !newVal && s.gradeRestrictionGuard != nil {
		restricted, err := s.gradeRestrictionGuard(ctx)
		if err != nil {
			return fmt.Errorf("check grade-restricted phases: %w", err)
		}
		if restricted {
			return &config.InvalidValueError{
				Key:    key,
				Reason: "Die Klassenstufen-Abfrage kann nicht deaktiviert werden, solange eine aktive Anmeldephase auf bestimmte Klassenstufen beschränkt ist. Entfernen Sie zuerst die Klassenstufenbeschränkung der betroffenen Phase.",
			}
		}
	}
	return nil
}
