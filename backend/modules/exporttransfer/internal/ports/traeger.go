package ports

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/exporttransfer/internal/domain"
)

// The ports of the Träger scope (ADR 0218). Every store method runs in the
// Träger transaction of the organization it names and filters by it itself.

// PasswordCipher seals and opens the recipient password with the bank ring
// (ADR 0183). The root binds Security Runtime's bank cipher.
type PasswordCipher interface {
	Seal(plaintext, context []byte) ([]byte, error)
	Open(sealed, context []byte) ([]byte, error)
}

// PasswordRewrapper re-seals a password on the active bank key; only the
// maintenance command holds it.
type PasswordRewrapper interface {
	Rewrap(sealed, context []byte) ([]byte, error)
}

// RecipientStore keeps the one recipient of a Träger.
type RecipientStore interface {
	Recipient(ctx context.Context, organizationID int64) (domain.StoredRecipient, bool, error)
	// LockRecipient reads the recipient FOR UPDATE.
	LockRecipient(ctx context.Context, organizationID int64) (domain.StoredRecipient, bool, error)
	SaveRecipient(ctx context.Context, recipient domain.StoredRecipient, actorAccountID int64) error
	DeleteRecipient(ctx context.Context, organizationID int64) error
	// ReplaceSealedPassword writes only the ciphertext, for the rekey.
	ReplaceSealedPassword(ctx context.Context, organizationID int64, sealed []byte) error
}

// TraegerJournal is the journal of the handovers of a Träger with the
// one-shot dispatch fence of the school journal.
type TraegerJournal interface {
	Begin(ctx context.Context, entry domain.TraegerJournalEntry) (int64, error)
	Claim(ctx context.Context, organizationID, id int64) (string, error)
	Complete(ctx context.Context, organizationID, id int64, token string, success bool, reason string) error
	List(ctx context.Context, organizationID int64, limit int) ([]domain.TraegerJournalRow, error)
}

// TraegerRuntime opens the Träger transactions of the scope. Detached drops
// an ambient request transaction, so each step commits on its own.
type TraegerRuntime struct {
	Detached      func(context.Context) context.Context
	WithinTraeger func(context.Context, int64, func(context.Context) error) error
}
