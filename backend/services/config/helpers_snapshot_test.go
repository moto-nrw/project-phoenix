package config

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"testing"

	"github.com/moto-nrw/project-phoenix/models/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func helperSnapshot(
	t *testing.T,
	key string,
	fieldType config.FieldType,
	defaultValue any,
	override json.RawMessage,
) *SettingsSnapshot {
	t.Helper()
	setupTest(t)
	registerTestSetting(key, fieldType, defaultValue)

	repo := newMockValueRepo()
	if override != nil {
		value := &config.SettingValue{SettingKey: key, Value: override}
		value.TenantID = 41
		repo.values[repo.key(value.TenantID, key)] = value
	}

	service := createService(repo, &mockAuditRepo{})
	batch, ok := service.(BatchSettingsService)
	require.True(t, ok)
	snapshot, err := batch.ResolveMany(tenantCtx(41), []string{key})
	require.NoError(t, err)
	return snapshot
}

func snapshotMock(snapshot *SettingsSnapshot, err error) *fakeSettingsService {
	return &fakeSettingsService{snapshot: snapshot, snapshotErr: err}
}

func TestResolveOrDefaultReturnsFallbackForSnapshotFailures(t *testing.T) {
	fallbackErr := errors.New("settings unavailable")
	logger := slog.New(slog.DiscardHandler)

	t.Run("boolean batch failure without logger", func(t *testing.T) {
		got := ResolveBoolOrDefault(
			context.Background(),
			snapshotMock(nil, fallbackErr),
			"test.bool",
			true,
			nil,
		)
		assert.True(t, got)
	})

	t.Run("integer batch failure", func(t *testing.T) {
		got := ResolveIntOrDefault(
			context.Background(),
			snapshotMock(nil, fallbackErr),
			"test.int",
			17,
			logger,
		)
		assert.Equal(t, 17, got)
	})

	t.Run("string batch failure", func(t *testing.T) {
		got := ResolveStringOrDefault(
			context.Background(),
			snapshotMock(nil, fallbackErr),
			"test.string",
			"fallback",
			logger,
		)
		assert.Equal(t, "fallback", got)
	})

	t.Run("boolean missing from snapshot", func(t *testing.T) {
		snapshot := helperSnapshot(t, "test.present_bool", config.FieldBoolean, false, nil)
		got := ResolveBoolOrDefault(
			context.Background(),
			snapshotMock(snapshot, nil),
			"test.missing_bool",
			true,
			logger,
		)
		assert.True(t, got)
	})

	t.Run("integer missing from snapshot", func(t *testing.T) {
		snapshot := helperSnapshot(t, "test.present_int", config.FieldNumber, 3, nil)
		got := ResolveIntOrDefault(
			context.Background(),
			snapshotMock(snapshot, nil),
			"test.missing_int",
			17,
			logger,
		)
		assert.Equal(t, 17, got)
	})

	t.Run("string missing from snapshot", func(t *testing.T) {
		snapshot := helperSnapshot(t, "test.present_string", config.FieldText, "value", nil)
		got := ResolveStringOrDefault(
			context.Background(),
			snapshotMock(snapshot, nil),
			"test.missing_string",
			"fallback",
			logger,
		)
		assert.Equal(t, "fallback", got)
	})

	t.Run("boolean type mismatch", func(t *testing.T) {
		snapshot := helperSnapshot(
			t,
			"test.bad_bool",
			config.FieldBoolean,
			false,
			json.RawMessage(`"not-a-bool"`),
		)
		got := ResolveBoolOrDefault(
			context.Background(),
			snapshotMock(snapshot, nil),
			"test.bad_bool",
			true,
			logger,
		)
		assert.True(t, got)
	})

	t.Run("fractional integer", func(t *testing.T) {
		snapshot := helperSnapshot(
			t,
			"test.bad_int",
			config.FieldNumber,
			3,
			json.RawMessage(`1.5`),
		)
		got := ResolveIntOrDefault(
			context.Background(),
			snapshotMock(snapshot, nil),
			"test.bad_int",
			17,
			logger,
		)
		assert.Equal(t, 17, got)
	})

	t.Run("malformed string", func(t *testing.T) {
		snapshot := helperSnapshot(
			t,
			"test.bad_string",
			config.FieldText,
			"default",
			json.RawMessage(`{`),
		)
		got := ResolveStringOrDefault(
			context.Background(),
			snapshotMock(snapshot, nil),
			"test.bad_string",
			"fallback",
			logger,
		)
		assert.Equal(t, "fallback", got)
	})

	t.Run("empty string override", func(t *testing.T) {
		snapshot := helperSnapshot(
			t,
			"test.empty_string",
			config.FieldText,
			"default",
			json.RawMessage(`""`),
		)
		got := ResolveStringOrDefault(
			context.Background(),
			snapshotMock(snapshot, nil),
			"test.empty_string",
			"fallback",
			logger,
		)
		assert.Equal(t, "fallback", got)
	})
}
