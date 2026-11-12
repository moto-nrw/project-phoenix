package ports

import (
	"context"

	"github.com/moto-ogs/project-phoenix/modules/exporttransfer/internal/domain"
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

// RecipientStore keeps the recipients of a Träger and the format each one
// receives. A recipient of another Träger is never found.
type RecipientStore interface {
	// Recipients reads every recipient of the Träger, ordered by name, with
	// its formats.
	Recipients(ctx context.Context, organizationID int64) ([]domain.StoredRecipient, error)
	Recipient(ctx context.Context, organizationID, recipientID int64) (domain.StoredRecipient, bool, error)
	// RecipientForFormat reads the recipient the format goes to.
	RecipientForFormat(ctx context.Context, organizationID int64, format string) (domain.StoredRecipient, bool, error)
	// LockRecipient reads the recipient FOR UPDATE.
	LockRecipient(ctx context.Context, organizationID, recipientID int64) (domain.StoredRecipient, bool, error)
	// LockRecipientsAfter reads up to limit recipients with an id above
	// cursor FOR UPDATE, by id, for the rekey.
	LockRecipientsAfter(ctx context.Context, organizationID, cursor int64, limit int) ([]domain.StoredRecipient, error)
	// InsertRecipient answers the new id; a taken name is domain.ErrNameTaken.
	InsertRecipient(ctx context.Context, recipient domain.StoredRecipient, actorAccountID int64) (int64, error)
	// UpdateRecipient answers whether the recipient existed; a taken name is
	// domain.ErrNameTaken.
	UpdateRecipient(ctx context.Context, recipient domain.StoredRecipient, actorAccountID int64) (bool, error)
	// DeleteRecipient answers whether it existed; its formats go with it.
	DeleteRecipient(ctx context.Context, organizationID, recipientID int64) (bool, error)
	// ReplaceSealedPassword writes only the ciphertext, for the rekey.
	ReplaceSealedPassword(ctx context.Context, organizationID, recipientID int64, sealed []byte) error
	// AssignFormat sends the format to the recipient, or to the download for
	// recipientID 0; a recipient of another Träger answers false.
	AssignFormat(ctx context.Context, organizationID int64, format string, recipientID, actorAccountID int64) (bool, error)
}

// TraegerJournal is the journal of the handovers of a Träger with the
// one-shot dispatch fence of the school journal.
type TraegerJournal interface {
	Begin(ctx context.Context, entry domain.TraegerJournalEntry) (int64, error)
	Claim(ctx context.Context, organizationID, id int64) (string, error)
	Complete(ctx context.Context, organizationID, id int64, token string, success bool, reason string) error
	// List reads the newest attempts; recipientID 0 reads those of every
	// recipient.
	List(ctx context.Context, organizationID, recipientID int64, limit int) ([]domain.TraegerJournalRow, error)
}

// TraegerRuntime opens the Träger transactions of the scope. Detached drops
// an ambient request transaction, so each step commits on its own.
type TraegerRuntime struct {
	Detached      func(context.Context) context.Context
	WithinTraeger func(context.Context, int64, func(context.Context) error) error
}
