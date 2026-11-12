package postgres

import (
	"context"
	"fmt"

	"github.com/moto-ogs/project-phoenix/modules/parentbilling"
)

// The submitted messages of the Rückgabe of a Träger (#6314), read one
// message at a time so the export holds one file in memory. Every statement
// names the Träger itself: the Träger transaction of the internal procedure
// runs without a request role, so row level security is no backstop.

// sepaReturnMessagesSQL answers every submitted message of the Träger: each
// with a submitted debit and each whose stored original stayed after its
// last debit (a submitted file stays while an installment notice of it
// stays). A file kept without its bytes and without a debit has nothing
// to ship.
const sepaReturnMessagesSQL = `SELECT debit.message_id FROM parent_billing.sepa_debits AS debit
	WHERE debit.organization_id = ?0 AND debit.status = 'submitted' AND debit.message_id IS NOT NULL AND debit.message_id <> ''
UNION
SELECT file.message_id FROM parent_billing.sepa_debit_files AS file
	WHERE file.organization_id = ?0 AND file.submitted_at IS NOT NULL AND file.file_sealed IS NOT NULL
ORDER BY message_id`

// sepaReturnMessageDebitsSQL reads the submitted debits of one message with
// their sealed account, in the order of the file (invoice number, then ID,
// as the download reads a run).
const sepaReturnMessageDebitsSQL = sepaDebitColumns + sepaDebitAccountColumns + sepaDebitState +
	`debit.status = 'submitted' AND debit.message_id = ?1 ORDER BY debit.invoice_number, debit.id`

// ReturnSEPAMessages answers the ID of every submitted message of the Träger.
func (s *Store) ReturnSEPAMessages(ctx context.Context, organizationID int64) ([]string, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	var messages []string
	if err := db.NewRaw(sepaReturnMessagesSQL, organizationID).Scan(ctx, &messages); err != nil {
		return nil, fmt.Errorf("read the SEPA messages of the Rückgabe: %w", err)
	}
	return messages, nil
}

// ReturnSEPAMessageDebits reads the submitted debits of one message with
// their sealed account.
func (s *Store) ReturnSEPAMessageDebits(ctx context.Context, organizationID int64, messageID string) ([]parentbilling.SealedSEPADebit, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	return scanSEPADebits(ctx, db.NewRaw(sepaReturnMessageDebitsSQL, organizationID, messageID))
}
