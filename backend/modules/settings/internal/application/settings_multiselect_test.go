package config

import (
	"encoding/json"
	"testing"

	config "github.com/moto-nrw/project-phoenix/modules/settings/records"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func registerMultiSelect(t *testing.T, registry *config.Registry) {
	t.Helper()
	registry.Register(config.Definition{
		Key:      "test.multi",
		Type:     config.FieldMultiSelect,
		Default:  []string{},
		Tab:      "test",
		Category: "test",
		Options: &config.SelectOptions{
			Static: []config.SelectOption{
				{Label: "A", Value: "a"},
				{Label: "B", Value: "b"},
			},
		},
	})
}

func TestSetValue_MultiSelectRejectsInvalidValues(t *testing.T) {
	t.Parallel()
	registry := setupTest(t)
	registerMultiSelect(t, registry)
	svc := createService(registry, newMockValueRepo(), &mockAuditRepo{})

	for name, value := range map[string]any{
		"null":               nil,
		"single string":      "a",
		"unknown option":     []any{"c"},
		"non-string":         []any{true},
		"duplicate":          []any{"a", "a"},
		"object":             map[string]any{"a": true},
		"one unknown of two": []any{"a", "c"},
	} {
		t.Run(name, func(t *testing.T) {
			err := svc.SetValue(tenantCtx(1), "test.multi", value, nil, nil)
			require.Error(t, err)
			var invalid *config.InvalidValueError
			assert.ErrorAs(t, err, &invalid, "a bad multi-select value is a 400, not a 500")
		})
	}
}

func TestSetValue_MultiSelectStoresChoiceInOptionOrder(t *testing.T) {
	t.Parallel()
	registry := setupTest(t)
	registerMultiSelect(t, registry)
	repo := newMockValueRepo()
	svc := createService(registry, repo, &mockAuditRepo{})
	ctx := tenantCtx(1)

	require.NoError(t, svc.SetValue(ctx, "test.multi", []any{"b", "a"}, nil, nil))
	stored := repo.values[repo.key(1, "test.multi")]
	require.NotNil(t, stored)
	assert.JSONEq(t, `["a","b"]`, string(stored.Value))

	resolved, err := svc.Resolve(tenantCtx(1), "test.multi")
	require.NoError(t, err)
	values, err := config.MultiSelectValues(resolved)
	require.NoError(t, err)
	assert.Equal(t, []string{"a", "b"}, values)
}

func TestSetValue_MultiSelectStoresEmptyListNotNull(t *testing.T) {
	t.Parallel()
	registry := setupTest(t)
	registerMultiSelect(t, registry)
	repo := newMockValueRepo()
	svc := createService(registry, repo, &mockAuditRepo{})

	for name, value := range map[string]any{"json array": []any{}, "nil slice": []string(nil)} {
		t.Run(name, func(t *testing.T) {
			require.NoError(t, svc.SetValue(tenantCtx(1), "test.multi", value, nil, nil))
			stored := repo.values[repo.key(1, "test.multi")]
			require.NotNil(t, stored)
			var decoded any
			require.NoError(t, json.Unmarshal(stored.Value, &decoded))
			assert.Equal(t, []any{}, decoded, "nothing selected is stored as []")
		})
	}
}

func TestResolve_MultiSelectDefaultIsEmptyList(t *testing.T) {
	t.Parallel()
	registry := setupTest(t)
	registerMultiSelect(t, registry)
	svc := createService(registry, newMockValueRepo(), &mockAuditRepo{})

	resolved, err := svc.Resolve(tenantCtx(1), "test.multi")
	require.NoError(t, err)
	values, err := config.MultiSelectValues(resolved)
	require.NoError(t, err)
	assert.Equal(t, []string{}, values)
}
