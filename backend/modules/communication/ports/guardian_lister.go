package ports

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/communication"
)

// GuardianLister answers which guardians of a child hold a portal account and
// may therefore receive a message. People Directory owns those rows, so the
// composition root supplies this instead of Communication joining them.
type GuardianLister func(ctx context.Context, studentID int64) ([]*communication.ParentGuardianRecipient, error)
