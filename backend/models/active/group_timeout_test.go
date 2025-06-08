package active

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestGroup_UpdateActivity(t *testing.T) {
	group := &Group{
		StartTime:      time.Now().Add(-30 * time.Minute),
		LastActivity:   time.Now().Add(-10 * time.Minute),
		TimeoutMinutes: 30,
	}

	originalActivity := group.LastActivity
	time.Sleep(1 * time.Millisecond) // Ensure time difference

	group.UpdateActivity()

	assert.True(t, group.LastActivity.After(originalActivity), "LastActivity should be updated to a more recent time")
	assert.WithinDuration(t, time.Now(), group.LastActivity, 1*time.Second, "LastActivity should be close to current time")
}

func TestGroup_GetTimeoutDuration(t *testing.T) {
	tests := []struct {
		name           string
		timeoutMinutes int
		expectedDuration time.Duration
	}{
		{
			name:           "default 30 minutes",
			timeoutMinutes: 30,
			expectedDuration: 30 * time.Minute,
		},
		{
			name:           "custom 60 minutes",
			timeoutMinutes: 60,
			expectedDuration: 60 * time.Minute,
		},
		{
			name:           "zero timeout defaults to 30 minutes",
			timeoutMinutes: 0,
			expectedDuration: 30 * time.Minute,
		},
		{
			name:           "negative timeout defaults to 30 minutes",
			timeoutMinutes: -10,
			expectedDuration: 30 * time.Minute,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			group := &Group{
				TimeoutMinutes: tt.timeoutMinutes,
			}

			duration := group.GetTimeoutDuration()
			assert.Equal(t, tt.expectedDuration, duration)
		})
	}
}

func TestGroup_GetInactivityDuration(t *testing.T) {
	now := time.Now()
	
	tests := []struct {
		name               string
		lastActivity       time.Time
		expectedInactivity time.Duration
		tolerance          time.Duration
	}{
		{
			name:               "5 minutes inactive",
			lastActivity:       now.Add(-5 * time.Minute),
			expectedInactivity: 5 * time.Minute,
			tolerance:          1 * time.Second,
		},
		{
			name:               "30 minutes inactive",
			lastActivity:       now.Add(-30 * time.Minute),
			expectedInactivity: 30 * time.Minute,
			tolerance:          1 * time.Second,
		},
		{
			name:               "just active (0 minutes)",
			lastActivity:       now,
			expectedInactivity: 0,
			tolerance:          1 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			group := &Group{
				LastActivity: tt.lastActivity,
			}

			inactivity := group.GetInactivityDuration()
			
			// Check that the inactivity duration is within tolerance of expected
			diff := inactivity - tt.expectedInactivity
			if diff < 0 {
				diff = -diff
			}
			assert.True(t, diff <= tt.tolerance, 
				"Inactivity duration %v should be within %v of expected %v", 
				inactivity, tt.tolerance, tt.expectedInactivity)
		})
	}
}

func TestGroup_IsTimedOut(t *testing.T) {
	now := time.Now()
	
	tests := []struct {
		name           string
		lastActivity   time.Time
		timeoutMinutes int
		endTime        *time.Time
		expectedResult bool
	}{
		{
			name:           "active session - not timed out",
			lastActivity:   now.Add(-10 * time.Minute),
			timeoutMinutes: 30,
			endTime:        nil,
			expectedResult: false,
		},
		{
			name:           "active session - timed out",
			lastActivity:   now.Add(-35 * time.Minute),
			timeoutMinutes: 30,
			endTime:        nil,
			expectedResult: true,
		},
		{
			name:           "already ended session - not timed out",
			lastActivity:   now.Add(-35 * time.Minute),
			timeoutMinutes: 30,
			endTime:        func(t time.Time) *time.Time { return &t }(now.Add(-5 * time.Minute)),
			expectedResult: false,
		},
		{
			name:           "exactly at timeout threshold",
			lastActivity:   now.Add(-30 * time.Minute),
			timeoutMinutes: 30,
			endTime:        nil,
			expectedResult: true, // Should be considered timed out when exactly at threshold
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			group := &Group{
				LastActivity:   tt.lastActivity,
				TimeoutMinutes: tt.timeoutMinutes,
				EndTime:        tt.endTime,
			}

			result := group.IsTimedOut()
			assert.Equal(t, tt.expectedResult, result)
		})
	}
}

func TestGroup_GetTimeUntilTimeout(t *testing.T) {
	now := time.Now()
	
	tests := []struct {
		name           string
		lastActivity   time.Time
		timeoutMinutes int
		expectedSign   string // "positive", "negative", or "zero"
		tolerance      time.Duration
	}{
		{
			name:           "10 minutes until timeout",
			lastActivity:   now.Add(-20 * time.Minute),
			timeoutMinutes: 30,
			expectedSign:   "positive",
			tolerance:      1 * time.Second,
		},
		{
			name:           "already timed out (5 minutes over)",
			lastActivity:   now.Add(-35 * time.Minute),
			timeoutMinutes: 30,
			expectedSign:   "negative",
			tolerance:      1 * time.Second,
		},
		{
			name:           "exactly at timeout",
			lastActivity:   now.Add(-30 * time.Minute),
			timeoutMinutes: 30,
			expectedSign:   "zero",
			tolerance:      1 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			group := &Group{
				LastActivity:   tt.lastActivity,
				TimeoutMinutes: tt.timeoutMinutes,
			}

			timeUntil := group.GetTimeUntilTimeout()

			switch tt.expectedSign {
			case "positive":
				assert.True(t, timeUntil > 0, "Time until timeout should be positive")
				expectedTime := 10 * time.Minute // Based on test case
				diff := timeUntil - expectedTime
				if diff < 0 {
					diff = -diff
				}
				assert.True(t, diff <= tt.tolerance, 
					"Time until timeout %v should be within %v of expected %v", 
					timeUntil, tt.tolerance, expectedTime)
			case "negative":
				assert.True(t, timeUntil < 0, "Time until timeout should be negative (already timed out)")
				expectedTime := -5 * time.Minute // Based on test case
				diff := timeUntil - expectedTime
				if diff < 0 {
					diff = -diff
				}
				assert.True(t, diff <= tt.tolerance, 
					"Time until timeout %v should be within %v of expected %v", 
					timeUntil, tt.tolerance, expectedTime)
			case "zero":
				assert.True(t, timeUntil >= -tt.tolerance && timeUntil <= tt.tolerance, 
					"Time until timeout should be approximately zero, got %v", timeUntil)
			}
		})
	}
}

func TestGroup_TimeoutMethods_Integration(t *testing.T) {
	// Create a group that started 45 minutes ago with last activity 35 minutes ago
	now := time.Now()
	group := &Group{
		StartTime:      now.Add(-45 * time.Minute),
		LastActivity:   now.Add(-35 * time.Minute),
		TimeoutMinutes: 30,
		EndTime:        nil, // Active session
	}

	// Test all timeout methods together
	t.Run("integration test", func(t *testing.T) {
		// Should be timed out (35 minutes > 30 minute timeout)
		assert.True(t, group.IsTimedOut(), "Group should be timed out")

		// Inactivity should be ~35 minutes
		inactivity := group.GetInactivityDuration()
		assert.True(t, inactivity >= 34*time.Minute && inactivity <= 36*time.Minute, 
			"Inactivity duration should be around 35 minutes, got %v", inactivity)

		// Time until timeout should be negative (already timed out by ~5 minutes)
		timeUntil := group.GetTimeUntilTimeout()
		assert.True(t, timeUntil < 0, "Time until timeout should be negative")
		assert.True(t, timeUntil >= -6*time.Minute && timeUntil <= -4*time.Minute, 
			"Time until timeout should be around -5 minutes, got %v", timeUntil)

		// Update activity and retest
		group.UpdateActivity()
		
		// Should no longer be timed out
		assert.False(t, group.IsTimedOut(), "Group should not be timed out after activity update")
		
		// Time until timeout should now be positive (~30 minutes)
		timeUntil = group.GetTimeUntilTimeout()
		assert.True(t, timeUntil > 0, "Time until timeout should be positive after activity update")
		assert.True(t, timeUntil >= 29*time.Minute && timeUntil <= 30*time.Minute, 
			"Time until timeout should be around 30 minutes, got %v", timeUntil)
	})
}