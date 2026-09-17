package domain

import (
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

func TestTokenDeliveryBoundsTheErrorByRunes(t *testing.T) {
	t.Parallel()

	text := func(value string) *string { return &value }
	tests := []struct {
		name  string
		error *string
		want  *string
	}{
		{"no error", nil, nil},
		{"empty", text(""), text("")},
		{"short", text("short error"), text("short error")},
		{"exactly the limit", text(strings.Repeat("a", maxDeliveryErrorRunes)), text(strings.Repeat("a", maxDeliveryErrorRunes))},
		{"one rune too long", text(strings.Repeat("b", maxDeliveryErrorRunes+1)), text(strings.Repeat("b", maxDeliveryErrorRunes))},
		// Each ä is two bytes: the cut counts runes, never splits one.
		{"multi-byte", text(strings.Repeat("ä", maxDeliveryErrorRunes+10)), text(strings.Repeat("ä", maxDeliveryErrorRunes))},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			original := TokenDelivery{Error: tt.error, RetryCount: 2}
			bounded := original.Bounded()
			if bounded.RetryCount != 2 {
				t.Fatalf("retry count changed to %d", bounded.RetryCount)
			}
			if tt.want == nil {
				if bounded.Error != nil {
					t.Fatalf("error = %q, want nil", *bounded.Error)
				}
				return
			}
			if bounded.Error == nil || *bounded.Error != *tt.want {
				t.Fatalf("error = %v, want %d runes", bounded.Error, utf8.RuneCountInString(*tt.want))
			}
			if !utf8.ValidString(*bounded.Error) {
				t.Fatal("bounded error is not valid UTF-8")
			}
		})
	}
}

func TestOperatorLinksAreNeverStoredExpiredOrSpent(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC)
	used := now.Add(-time.Minute)
	invitation := func(change func(*OperatorInvitation)) *OperatorInvitation {
		value := &OperatorInvitation{Email: "a@example.org", Token: "t", CreatedBy: 7, ExpiresAt: now.Add(time.Hour)}
		change(value)
		return value
	}
	emailChange := func(change func(*OperatorEmailChange)) *OperatorEmailChange {
		value := &OperatorEmailChange{OperatorID: 7, NewEmail: "a@example.org", Token: "t", Expiry: now.Add(time.Minute)}
		change(value)
		return value
	}
	tests := []struct {
		name string
		err  error
		want string
	}{
		{"valid invitation", invitation(func(*OperatorInvitation) {}).Validate(now), ""},
		{"invitation expiring now", invitation(func(i *OperatorInvitation) { i.ExpiresAt = now }).Validate(now), "token has already expired"},
		{"expired invitation", invitation(func(i *OperatorInvitation) { i.ExpiresAt = now.Add(-time.Second) }).Validate(now), "token has already expired"},
		{"used invitation", invitation(func(i *OperatorInvitation) { i.UsedAt = &used }).Validate(now), "token has already been used"},
		{"invitation without email", invitation(func(i *OperatorInvitation) { i.Email = "" }).Validate(now), "email is required"},
		{"invitation without token", invitation(func(i *OperatorInvitation) { i.Token = "" }).Validate(now), "token value is required"},
		{"invitation without inviter", invitation(func(i *OperatorInvitation) { i.CreatedBy = 0 }).Validate(now), "created_by operator ID is required"},
		{"valid email change", emailChange(func(*OperatorEmailChange) {}).Validate(now), ""},
		{"email change expiring now", emailChange(func(c *OperatorEmailChange) { c.Expiry = now }).Validate(now), "token has already expired"},
		{"expired email change", emailChange(func(c *OperatorEmailChange) { c.Expiry = now.Add(-time.Second) }).Validate(now), "token has already expired"},
		{"used email change", emailChange(func(c *OperatorEmailChange) { c.Used = true }).Validate(now), "token has already been used"},
		{"email change without operator", emailChange(func(c *OperatorEmailChange) { c.OperatorID = 0 }).Validate(now), "operator ID is required"},
		{"email change without address", emailChange(func(c *OperatorEmailChange) { c.NewEmail = "" }).Validate(now), "new email is required"},
		{"email change without token", emailChange(func(c *OperatorEmailChange) { c.Token = "" }).Validate(now), "token value is required"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if tt.want == "" {
				if tt.err != nil {
					t.Fatalf("unexpected error: %v", tt.err)
				}
				return
			}
			if tt.err == nil || tt.err.Error() != tt.want {
				t.Fatalf("error = %v, want %q", tt.err, tt.want)
			}
		})
	}
}
