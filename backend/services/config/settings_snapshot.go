package config

import (
	"context"
	"encoding/json"
	"fmt"
	"math"

	configModel "github.com/moto-nrw/project-phoenix/models/config"
)

type settingsSnapshotContextKey struct{}

type snapshotValue struct {
	value       any
	hasOverride bool
	err         error
}

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

func snapshotFromContext(ctx context.Context, tenantID int64, keys []string) *SettingsSnapshot {
	snapshot, _ := ctx.Value(settingsSnapshotContextKey{}).(*SettingsSnapshot)
	if snapshot == nil || snapshot.tenantID != tenantID || !snapshot.containsAll(keys) {
		return nil
	}
	return snapshot
}

func newSettingsSnapshot(tenantID int64, keys []string, stored []*configModel.SettingValue) (*SettingsSnapshot, error) {
	values := make(map[string]snapshotValue, len(keys))
	for _, key := range keys {
		if _, exists := values[key]; exists {
			continue
		}
		def := configModel.GetDefinition(key)
		if def == nil {
			return nil, &SettingsError{
				Op:  "resolve_many",
				Err: &DefinitionNotFoundError{Key: key},
			}
		}
		values[key] = snapshotValue{value: def.Default}
	}

	for _, storedValue := range stored {
		if storedValue == nil || storedValue.GetTenantID() != tenantID {
			continue
		}
		if _, requested := values[storedValue.SettingKey]; !requested {
			continue
		}
		var value any
		err := json.Unmarshal(storedValue.Value, &value)
		if err != nil {
			err = fmt.Errorf("unmarshal value for %s: %w", storedValue.SettingKey, err)
		}
		values[storedValue.SettingKey] = snapshotValue{
			value:       value,
			hasOverride: true,
			err:         err,
		}
	}

	return &SettingsSnapshot{tenantID: tenantID, values: values}, nil
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
