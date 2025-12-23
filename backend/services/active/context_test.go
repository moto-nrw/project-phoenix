package active

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWithAttendanceAutoSync(t *testing.T) {
	tests := []struct {
		name     string
		setup    func() context.Context
		expected bool
	}{
		{
			name: "adds auto sync flag to context",
			setup: func() context.Context {
				return context.Background()
			},
			expected: true,
		},
		{
			name: "adds auto sync flag to context with existing values",
			setup: func() context.Context {
				return context.WithValue(context.Background(), "other-key", "other-value")
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := tt.setup()
			newCtx := WithAttendanceAutoSync(ctx)

			assert.NotEqual(t, ctx, newCtx)
			assert.True(t, shouldAutoSyncAttendance(newCtx))
		})
	}
}

func TestShouldAutoSyncAttendance(t *testing.T) {
	tests := []struct {
		name     string
		ctx      context.Context
		expected bool
	}{
		{
			name:     "returns false for plain context",
			ctx:      context.Background(),
			expected: false,
		},
		{
			name:     "returns true when flag is set",
			ctx:      WithAttendanceAutoSync(context.Background()),
			expected: true,
		},
		{
			name: "returns false for wrong type in context",
			ctx: context.WithValue(context.Background(), attendanceAutoSyncKey, "not-a-bool"),
			expected: false,
		},
		{
			name: "returns false for nil value in context",
			ctx: context.WithValue(context.Background(), attendanceAutoSyncKey, nil),
			expected: false,
		},
		{
			name: "returns false when flag is explicitly set to false",
			ctx: context.WithValue(context.Background(), attendanceAutoSyncKey, false),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := shouldAutoSyncAttendance(tt.ctx)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestContextKey(t *testing.T) {
	// Verify the context key type is properly defined
	var key contextKey = "test-key"
	assert.Equal(t, "test-key", string(key))

	// Verify attendance key has expected value
	assert.Equal(t, "active:autoSyncAttendance", string(attendanceAutoSyncKey))
}
