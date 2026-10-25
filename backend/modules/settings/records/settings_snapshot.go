package records

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"math"
	"time"
)

type settingsSnapshotContextKey struct{}

type snapshotValue struct {
	value       any
	hasOverride bool
	err         error
	// source names the level the effective value comes from; traegerSet
	// and traegerChangedAt describe the Träger level beneath a school
	// override as well, so a school can see what a reset would bring back.
	source           ValueSource
	inherited        any
	traegerChangedAt *time.Time
}

// ValueSource names where an effective setting value comes from (ADR 0086
// section 2).
type ValueSource string

const (
	SourceDefault ValueSource = "default"
	SourceTraeger ValueSource = "traeger"
	SourceSchool  ValueSource = "school"
)

// SettingsSnapshot is an immutable, tenant-bound set of resolved settings.
// It keeps override presence separate from the effective value so callers with
// legacy environment fallbacks do not need a second database query.
type SettingsSnapshot struct {
	tenantID int64
	values   map[string]snapshotValue
}

// WithSettingsSnapshot attaches a tenant-bound snapshot to ctx. Settings
// resolution reuses it only when its tenant matches the request context.
func WithSettingsSnapshot(ctx context.Context, snapshot *SettingsSnapshot) context.Context {
	if snapshot == nil {
		return ctx
	}
	return context.WithValue(ctx, settingsSnapshotContextKey{}, snapshot)
}

func SnapshotFromContext(ctx context.Context, tenantID int64, keys []string) *SettingsSnapshot {
	snapshot, _ := ctx.Value(settingsSnapshotContextKey{}).(*SettingsSnapshot)
	if snapshot == nil || snapshot.tenantID != tenantID || !snapshot.containsAll(keys) {
		return nil
	}
	return snapshot
}

func NewSettingsSnapshot(registry *Registry, tenantID int64, keys []string, stored []*SettingValue) (*SettingsSnapshot, error) {
	return NewLayeredSettingsSnapshot(registry, tenantID, keys, stored, nil)
}

// NewLayeredSettingsSnapshot resolves registry default, then the Träger value,
// then the school override (ADR 0086 section 2). traeger holds the values of
// the school's own Träger only. A Träger value counts only for
// a Träger-level definition; a school override of a Träger-only definition is
// ignored, because the school cannot deviate from it.
func NewLayeredSettingsSnapshot(registry *Registry, tenantID int64, keys []string, stored []*SettingValue, traeger []*TraegerSettingValue) (*SettingsSnapshot, error) {
	values := make(map[string]snapshotValue, len(keys))
	for _, key := range keys {
		if _, exists := values[key]; exists {
			continue
		}
		def := registry.GetDefinition(key)
		if def == nil {
			return nil, &SettingsError{
				Op:  "resolve_many",
				Err: &DefinitionNotFoundError{Key: key},
			}
		}
		values[key] = snapshotValue{value: def.Default, source: SourceDefault, inherited: def.Default}
	}

	applyTraegerValues(registry, values, traeger)
	applySchoolOverrides(registry, values, tenantID, stored)
	return &SettingsSnapshot{tenantID: tenantID, values: values}, nil
}

// applyTraegerValues lays the Träger values over the registry defaults of the
// requested Träger-level keys.
func applyTraegerValues(registry *Registry, values map[string]snapshotValue, traeger []*TraegerSettingValue) {
	for _, traegerValue := range traeger {
		if traegerValue == nil {
			continue
		}
		entry, requested := values[traegerValue.SettingKey]
		if !requested || !registry.GetDefinition(traegerValue.SettingKey).Levels.AtTraeger() {
			continue
		}
		value, err := decodeStoredValue(traegerValue.SettingKey, traegerValue.Value)
		changedAt := traegerValue.UpdatedAt
		entry.value, entry.inherited, entry.err = value, value, err
		entry.source, entry.traegerChangedAt = SourceTraeger, &changedAt
		values[traegerValue.SettingKey] = entry
	}
}

// applySchoolOverrides lays the school's overrides over both levels beneath.
// The override wins; a broken Träger value beneath it does not make the
// effective value unreadable. A Träger-only key has no school override.
func applySchoolOverrides(registry *Registry, values map[string]snapshotValue, tenantID int64, stored []*SettingValue) {
	for _, storedValue := range stored {
		if storedValue == nil || storedValue.GetTenantID() != tenantID {
			continue
		}
		entry, requested := values[storedValue.SettingKey]
		if !requested || registry.GetDefinition(storedValue.SettingKey).Levels == LevelTraeger {
			continue
		}
		value, err := decodeStoredValue(storedValue.SettingKey, storedValue.Value)
		entry.value, entry.hasOverride, entry.source, entry.err = value, true, SourceSchool, err
		values[storedValue.SettingKey] = entry
	}
}

func decodeStoredValue(key string, raw json.RawMessage) (any, error) {
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, fmt.Errorf("unmarshal value for %s: %w", key, err)
	}
	return value, nil
}

// newSettingsSnapshotFromValues builds a snapshot from already-resolved
// entries (the request-cache merge path). Callers must have validated every
// key against the registry beforehand.
func newSettingsSnapshotFromValues(tenantID int64, values map[string]snapshotValue) *SettingsSnapshot {
	return &SettingsSnapshot{tenantID: tenantID, values: values}
}

func (s *SettingsSnapshot) containsAll(keys []string) bool {
	if s == nil {
		return false
	}
	for _, key := range keys {
		if _, ok := s.values[key]; !ok {
			return false
		}
	}
	return true
}

func (s *SettingsSnapshot) entry(key string) (snapshotValue, error) {
	if s == nil {
		return snapshotValue{}, &SettingsError{Op: "snapshot", Err: fmt.Errorf("settings snapshot is nil")}
	}
	entry, ok := s.values[key]
	if !ok {
		return snapshotValue{}, &SettingsError{
			Op:  "snapshot",
			Err: &DefinitionNotFoundError{Key: key},
		}
	}
	if entry.err != nil {
		return snapshotValue{}, &SettingsError{Op: "snapshot", Err: entry.err}
	}
	return entry, nil
}

// Value returns the effective override or registry default for key.
func (s *SettingsSnapshot) Value(key string) (any, error) {
	entry, err := s.entry(key)
	if err != nil {
		return nil, err
	}
	return entry.value, nil
}

// HasOverride reports whether key has an explicit tenant row.
func (s *SettingsSnapshot) HasOverride(key string) (bool, error) {
	if s == nil {
		return false, &SettingsError{Op: "snapshot", Err: fmt.Errorf("settings snapshot is nil")}
	}
	entry, ok := s.values[key]
	if !ok {
		return false, &SettingsError{
			Op:  "snapshot",
			Err: &DefinitionNotFoundError{Key: key},
		}
	}
	return entry.hasOverride, nil
}

// Layer describes where key's effective value comes from: its source, the
// value the school inherits without its own override (Träger value or
// registry default), and when the Träger last changed its value (nil
// without one).
func (s *SettingsSnapshot) Layer(key string) (ValueSource, any, *time.Time, error) {
	entry, err := s.entry(key)
	if err != nil {
		return "", nil, nil, err
	}
	return entry.source, entry.inherited, entry.traegerChangedAt, nil
}

// String resolves key with the same conversion semantics as ResolveString.
func (s *SettingsSnapshot) String(key string) (string, error) {
	value, err := s.Value(key)
	if err != nil {
		return "", err
	}
	if value == nil {
		return "", nil
	}
	if stringValue, ok := value.(string); ok {
		return stringValue, nil
	}
	return fmt.Sprintf("%v", value), nil
}

// Bool resolves key as a boolean.
func (s *SettingsSnapshot) Bool(key string) (bool, error) {
	value, err := s.Value(key)
	if err != nil {
		return false, err
	}
	if value == nil {
		return false, nil
	}
	boolean, ok := value.(bool)
	if !ok {
		return false, &SettingsError{Op: "snapshot_bool", Err: fmt.Errorf("expected bool, got %T", value)}
	}
	return boolean, nil
}

// Int resolves key as a platform int without accepting fractional JSON values.
func (s *SettingsSnapshot) Int(key string) (int, error) {
	value, err := s.Value(key)
	if err != nil {
		return 0, err
	}
	if value == nil {
		return 0, nil
	}
	switch number := value.(type) {
	case float64:
		if number != math.Floor(number) {
			return 0, &SettingsError{Op: "snapshot_int", Err: fmt.Errorf("value %v has fractional part", number)}
		}
		if number > float64(math.MaxInt) || number < float64(math.MinInt) {
			return 0, &SettingsError{Op: "snapshot_int", Err: fmt.Errorf("value %v out of int range", number)}
		}
		return int(number), nil
	case int:
		return number, nil
	case json.Number:
		integer, parseErr := number.Int64()
		if parseErr != nil {
			return 0, &SettingsError{Op: "snapshot_int", Err: fmt.Errorf("invalid number: %w", parseErr)}
		}
		if integer > int64(math.MaxInt) || integer < int64(math.MinInt) {
			return 0, &SettingsError{Op: "snapshot_int", Err: fmt.Errorf("value %v out of int range", integer)}
		}
		return int(integer), nil
	default:
		return 0, &SettingsError{Op: "snapshot_int", Err: fmt.Errorf("expected number, got %T", value)}
	}
}

// MergeSettingsSnapshots combines cache hits and freshly resolved keys without exposing their maps.
func MergeSettingsSnapshots(left, right *SettingsSnapshot) *SettingsSnapshot {
	values := maps.Clone(left.values)
	maps.Copy(values, right.values)
	return newSettingsSnapshotFromValues(left.tenantID, values)
}
