package ports

import (
	"context"
	"time"
)

// CatererInvitationDelivery mails a school's invitation to its caterer (ADR
// 0074) through the outbox, in the school's transaction. The link exists
// only in the mail; a failed enqueue fails the invitation.
type CatererInvitationDelivery interface {
	EnqueueCatererInvitation(ctx context.Context, invitationID, schoolID int64, email, catererName, token string, expiresAt time.Time) error
}
