package postgres

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
)

// The sealed debits of the PAIN.008 files of the Rückgabe of a Träger (#6314). Every statement
// names the Träger itself: the Träger transaction of the internal procedure
// runs without a request role, so row level security is no backstop.

// sepaReturnDebitsSQL reads every debit of the Träger with its sealed
// account, grouped by message and in the order of the file (invoice number,
// then ID, as the download reads a run). Pending debits come last.
const sepaReturnDebitsSQL = sepaDebitColumns + sepaDebitAccountColumns + sepaDebitState +
	`TRUE ORDER BY debit.message_id NULLS LAST, debit.invoice_number, debit.id`

// ReturnSEPADebits reads every debit of the Träger with its sealed account.
func (s *Store) ReturnSEPADebits(ctx context.Context, organizationID int64) ([]parentbilling.SealedSEPADebit, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	return scanSEPADebits(ctx, db.NewRaw(sepaReturnDebitsSQL, organizationID))
}
