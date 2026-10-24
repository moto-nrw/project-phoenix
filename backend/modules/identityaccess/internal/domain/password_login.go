package domain

import "time"

// PasswordLoginState belongs to the identity, not its IP, portal or school.
// MFA code attempts have their own counter so a password cannot clear them.
type PasswordLoginState struct {
	PasswordAttempts    int
	PasswordLockedUntil *time.Time
}
