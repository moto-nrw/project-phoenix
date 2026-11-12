package identityaccess

import "context"

// GuardianInvitationAccountLock locks the inviter and any existing account of
// the invited address FOR UPDATE, in ascending ID order. The caller must hold
// no account or child locks yet and must use the same school transaction for
// the invitation and its authorization guard.
type GuardianInvitationAccountLock func(ctx context.Context, inviterID int64, email string) error
