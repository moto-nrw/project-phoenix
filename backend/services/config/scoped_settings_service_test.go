package config

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/moto-nrw/project-phoenix/models/config"
	"github.com/stretchr/testify/assert"
)

func TestBuildResolutionChain(t *testing.T) {
	svc := &scopedSettingsService{}

	tests := []struct {
		name          string
		scope         config.ScopeRef
		allowedScopes []string
		expectedLen   int
		description   string
	}{
		{
			name:          "system scope only",
			scope:         config.NewSystemScope(),
			allowedScopes: []string{"system"},
			expectedLen:   1,
			description:   "System scope with only system allowed",
		},
		{
			name:          "og scope with system fallback",
			scope:         config.NewOGScope(1),
			allowedScopes: []string{"og", "system"},
			expectedLen:   2,
			description:   "OG scope should have og and system in chain",
		},
		{
			name:          "og scope without system allowed",
			scope:         config.NewOGScope(1),
			allowedScopes: []string{"og"},
			expectedLen:   1,
			description:   "OG scope with only og allowed",
		},
		{
			name:          "user scope with full chain",
			scope:         config.NewUserScope(1),
			allowedScopes: []string{"user", "school", "system"},
			expectedLen:   3,
			description:   "User scope should check user, school, system",
		},
		{
			name:          "user scope when not in allowed",
			scope:         config.NewUserScope(1),
			allowedScopes: []string{"system"},
			expectedLen:   1,
			description:   "User scope falls back to system only",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chain := svc.buildResolutionChain(tt.scope, tt.allowedScopes)
			assert.Len(t, chain, tt.expectedLen, tt.description)
		})
	}
}

func TestEvaluateCondition(t *testing.T) {
	svc := &scopedSettingsService{}

	tests := []struct {
		name        string
		dependency  *config.SettingDependency
		actualValue any
		expected    bool
	}{
		{
			name:        "equals - match",
			dependency:  &config.SettingDependency{Condition: "equals", Value: true},
			actualValue: true,
			expected:    true,
		},
		{
			name:        "equals - no match",
			dependency:  &config.SettingDependency{Condition: "equals", Value: true},
			actualValue: false,
			expected:    false,
		},
		{
			name:        "equals - string match",
			dependency:  &config.SettingDependency{Condition: "equals", Value: "active"},
			actualValue: "active",
			expected:    true,
		},
		{
			name:        "not_equals - match",
			dependency:  &config.SettingDependency{Condition: "not_equals", Value: false},
			actualValue: true,
			expected:    true,
		},
		{
			name:        "not_equals - no match",
			dependency:  &config.SettingDependency{Condition: "not_equals", Value: true},
			actualValue: true,
			expected:    false,
		},
		{
			name:        "in - value in slice",
			dependency:  &config.SettingDependency{Condition: "in", Value: "option1"},
			actualValue: []interface{}{"option1", "option2"},
			expected:    true,
		},
		{
			name:        "in - value not in slice",
			dependency:  &config.SettingDependency{Condition: "in", Value: "option3"},
			actualValue: []interface{}{"option1", "option2"},
			expected:    false,
		},
		{
			name:        "in - string slice",
			dependency:  &config.SettingDependency{Condition: "in", Value: "admin"},
			actualValue: []string{"admin", "user"},
			expected:    true,
		},
		{
			name:        "not_empty - non-empty string",
			dependency:  &config.SettingDependency{Condition: "not_empty"},
			actualValue: "value",
			expected:    true,
		},
		{
			name:        "not_empty - empty string",
			dependency:  &config.SettingDependency{Condition: "not_empty"},
			actualValue: "",
			expected:    false,
		},
		{
			name:        "not_empty - nil",
			dependency:  &config.SettingDependency{Condition: "not_empty"},
			actualValue: nil,
			expected:    false,
		},
		{
			name:        "not_empty - non-zero int",
			dependency:  &config.SettingDependency{Condition: "not_empty"},
			actualValue: 5,
			expected:    true,
		},
		{
			name:        "not_empty - zero int",
			dependency:  &config.SettingDependency{Condition: "not_empty"},
			actualValue: 0,
			expected:    false,
		},
		{
			name:        "not_empty - bool true",
			dependency:  &config.SettingDependency{Condition: "not_empty"},
			actualValue: true,
			expected:    true,
		},
		{
			name:        "not_empty - bool false",
			dependency:  &config.SettingDependency{Condition: "not_empty"},
			actualValue: false,
			expected:    true, // false is still a value
		},
		{
			name:        "greater_than - true",
			dependency:  &config.SettingDependency{Condition: "greater_than", Value: 10},
			actualValue: 15,
			expected:    true,
		},
		{
			name:        "greater_than - false",
			dependency:  &config.SettingDependency{Condition: "greater_than", Value: 10},
			actualValue: 5,
			expected:    false,
		},
		{
			name:        "greater_than - equal",
			dependency:  &config.SettingDependency{Condition: "greater_than", Value: 10},
			actualValue: 10,
			expected:    false,
		},
		{
			name:        "greater_than - float values",
			dependency:  &config.SettingDependency{Condition: "greater_than", Value: 3.14},
			actualValue: float64(3.5),
			expected:    true,
		},
		{
			name:        "unknown condition - defaults to true",
			dependency:  &config.SettingDependency{Condition: "unknown"},
			actualValue: "anything",
			expected:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := svc.evaluateCondition(tt.dependency, tt.actualValue)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestToFloat(t *testing.T) {
	tests := []struct {
		name     string
		input    any
		expected float64
	}{
		{"float64", float64(3.14), 3.14},
		{"int", 42, 42.0},
		{"int64", int64(100), 100.0},
		{"string returns 0", "not a number", 0.0},
		{"nil returns 0", nil, 0.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := toFloat(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetChangeType(t *testing.T) {
	svc := &scopedSettingsService{}

	t.Run("nil existing returns create", func(t *testing.T) {
		result := svc.getChangeType(nil)
		assert.Equal(t, "create", result)
	})

	t.Run("existing value returns update", func(t *testing.T) {
		existing := &config.SettingValue{}
		result := svc.getChangeType(existing)
		assert.Equal(t, "update", result)
	})
}

func TestGetOldValueJSON(t *testing.T) {
	svc := &scopedSettingsService{}

	t.Run("nil existing returns nil", func(t *testing.T) {
		result := svc.getOldValueJSON(nil)
		assert.Nil(t, result)
	})

	t.Run("existing value returns its value", func(t *testing.T) {
		existing := &config.SettingValue{
			Value: []byte(`{"value": true}`),
		}
		result := svc.getOldValueJSON(existing)
		assert.Equal(t, existing.Value, result)
	})
}

func TestGetIPAddress(t *testing.T) {
	tests := []struct {
		name     string
		setup    func() *http.Request
		expected string
	}{
		{
			name: "nil request",
			setup: func() *http.Request {
				return nil
			},
			expected: "",
		},
		{
			name: "X-Forwarded-For header",
			setup: func() *http.Request {
				req := httptest.NewRequest("GET", "/", nil)
				req.Header.Set("X-Forwarded-For", "203.0.113.195")
				return req
			},
			expected: "203.0.113.195",
		},
		{
			name: "X-Real-IP header",
			setup: func() *http.Request {
				req := httptest.NewRequest("GET", "/", nil)
				req.Header.Set("X-Real-IP", "192.168.1.1")
				return req
			},
			expected: "192.168.1.1",
		},
		{
			name: "X-Forwarded-For takes precedence over X-Real-IP",
			setup: func() *http.Request {
				req := httptest.NewRequest("GET", "/", nil)
				req.Header.Set("X-Forwarded-For", "203.0.113.195")
				req.Header.Set("X-Real-IP", "192.168.1.1")
				return req
			},
			expected: "203.0.113.195",
		},
		{
			name: "falls back to RemoteAddr",
			setup: func() *http.Request {
				req := httptest.NewRequest("GET", "/", nil)
				return req
			},
			expected: "192.0.2.1:1234", // Default from httptest
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := tt.setup()
			result := getIPAddress(req)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestActor(t *testing.T) {
	t.Run("actor with permissions", func(t *testing.T) {
		actor := &Actor{
			AccountID:   1,
			PersonID:    2,
			Permissions: []string{"settings:read", "settings:update"},
		}

		assert.Equal(t, int64(1), actor.AccountID)
		assert.Equal(t, int64(2), actor.PersonID)
		assert.Len(t, actor.Permissions, 2)
	})
}

func TestSettingHistoryEntry(t *testing.T) {
	t.Run("history entry structure", func(t *testing.T) {
		entry := &SettingHistoryEntry{
			ID:         1,
			SettingKey: "test.setting",
			ChangeType: "update",
			OldValue:   10,
			NewValue:   20,
			ChangedAt:  "2024-01-15T10:00:00Z",
			Reason:     "Testing change",
		}

		assert.Equal(t, int64(1), entry.ID)
		assert.Equal(t, "test.setting", entry.SettingKey)
		assert.Equal(t, "update", entry.ChangeType)
		assert.Equal(t, 10, entry.OldValue)
		assert.Equal(t, 20, entry.NewValue)
		assert.Equal(t, "2024-01-15T10:00:00Z", entry.ChangedAt)
		assert.Equal(t, "Testing change", entry.Reason)
	})
}
