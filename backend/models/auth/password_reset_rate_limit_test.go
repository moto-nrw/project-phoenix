package auth

import (
	"testing"
	"time"
)

func TestPasswordResetRateLimit_Validate(t *testing.T) {
	tests := []struct {
		name    string
		limit   *PasswordResetRateLimit
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid rate limit",
			limit: &PasswordResetRateLimit{
				Email:       "test@example.com",
				Attempts:    1,
				WindowStart: time.Now(),
			},
			wantErr: false,
		},
		{
			name: "empty email",
			limit: &PasswordResetRateLimit{
				Email:       "",
				Attempts:    1,
				WindowStart: time.Now(),
			},
			wantErr: true,
			errMsg:  "email is required",
		},
		{
			name: "negative attempts",
			limit: &PasswordResetRateLimit{
				Email:       "test@example.com",
				Attempts:    -1,
				WindowStart: time.Now(),
			},
			wantErr: true,
			errMsg:  "attempts cannot be negative",
		},
		{
			name: "zero attempts is valid",
			limit: &PasswordResetRateLimit{
				Email:       "test@example.com",
				Attempts:    0,
				WindowStart: time.Now(),
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.limit.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("PasswordResetRateLimit.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && err != nil && err.Error() != tt.errMsg {
				t.Errorf("PasswordResetRateLimit.Validate() error = %q, want %q", err.Error(), tt.errMsg)
			}
		})
	}
}

func TestPasswordResetRateLimit_IncrementAttempts(t *testing.T) {
	limit := &PasswordResetRateLimit{
		Email:       "test@example.com",
		Attempts:    0,
		WindowStart: time.Now(),
	}

	limit.IncrementAttempts()
	if limit.Attempts != 1 {
		t.Errorf("Attempts = %d, want 1", limit.Attempts)
	}

	limit.IncrementAttempts()
	if limit.Attempts != 2 {
		t.Errorf("Attempts = %d, want 2", limit.Attempts)
	}

	limit.IncrementAttempts()
	if limit.Attempts != 3 {
		t.Errorf("Attempts = %d, want 3", limit.Attempts)
	}
}

func TestRateLimitState_RetryAfterSeconds(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name     string
		state    RateLimitState
		checkAt  time.Time
		expected int
	}{
		{
			name: "zero retry time",
			state: RateLimitState{
				Attempts: 1,
				RetryAt:  time.Time{},
			},
			checkAt:  now,
			expected: 0,
		},
		{
			name: "retry time in past",
			state: RateLimitState{
				Attempts: 3,
				RetryAt:  now.Add(-5 * time.Minute),
			},
			checkAt:  now,
			expected: 0,
		},
		{
			name: "retry time at current time",
			state: RateLimitState{
				Attempts: 3,
				RetryAt:  now,
			},
			checkAt:  now,
			expected: 0,
		},
		{
			name: "retry time in future - 5 minutes",
			state: RateLimitState{
				Attempts: 3,
				RetryAt:  now.Add(5 * time.Minute),
			},
			checkAt:  now,
			expected: 300,
		},
		{
			name: "retry time in future - 30 seconds",
			state: RateLimitState{
				Attempts: 3,
				RetryAt:  now.Add(30 * time.Second),
			},
			checkAt:  now,
			expected: 30,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.state.RetryAfterSeconds(tt.checkAt)
			if got != tt.expected {
				t.Errorf("RateLimitState.RetryAfterSeconds() = %v, want %v", got, tt.expected)
			}
		})
	}
}
