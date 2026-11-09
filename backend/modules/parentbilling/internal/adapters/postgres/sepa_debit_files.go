package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// The stored PAIN.008 file of each message (#6314, ADR 0187). Every
// statement names the Träger itself, like the debits.

// saveSEPADebitFileSQL keeps the last download of a message with its
// members until its submission; a submitted row stays as it is.
const saveSEPADebitFileSQL = `INSERT INTO parent_billing.sepa_debit_files AS kept (organization_id, message_id, run_kind, run_id,
	file_sealed, file_sha256, creditor_name, creditor_iban, creditor_bic, creditor_id, debit_ids, notice_ids, mandate_ids,
	downloaded_at, downloaded_by_account_id)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, clock_timestamp(), ?)
	ON CONFLICT (organization_id, message_id) DO UPDATE SET file_sealed = EXCLUDED.file_sealed, file_sha256 = EXCLUDED.file_sha256,
		creditor_name = EXCLUDED.creditor_name, creditor_iban = EXCLUDED.creditor_iban, creditor_bic = EXCLUDED.creditor_bic,
		creditor_id = EXCLUDED.creditor_id, debit_ids = EXCLUDED.debit_ids, notice_ids = EXCLUDED.notice_ids,
		mandate_ids = EXCLUDED.mandate_ids, downloaded_at = EXCLUDED.downloaded_at, downloaded_by_account_id = EXCLUDED.downloaded_by_account_id
	WHERE kept.submitted_at IS NULL`

// sepaDebitFileMembersConstraint names the refusal of a file whose member
// is gone: the trigger that locks the members of a stored file raises it.
const sepaDebitFileMembersConstraint = "sepa_debit_files_members"

// SaveSEPADebitFile keeps the downloaded file of a message. The members
// stay locked until the transaction ends; a member deleted since the run
// was read is ErrSEPADebitsChanged and nothing is kept.
func (s *Store) SaveSEPADebitFile(ctx context.Context, organizationID, accountID int64, file parentbilling.StoredSEPADebitFile) error {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return err
	}
	if _, err := db.NewRaw(saveSEPADebitFileSQL, organizationID, file.MessageID, file.RunKind, file.RunID, file.Sealed, file.SHA256,
		file.CreditorName, file.CreditorIBAN, file.CreditorBIC, file.CreditorID, members(file.DebitIDs), members(file.NoticeIDs),
		members(file.MandateIDs), accountRef(accountID)).Exec(ctx); err != nil {
		return sepaDebitFileError("keep the SEPA debit file", err)
	}
	return nil
}

// sepaDebitFileError reports a gone member as ErrSEPADebitsChanged.
func sepaDebitFileError(action string, err error) error {
	if constraintOf(err) == sepaDebitFileMembersConstraint {
		return fmt.Errorf("%s: %w", action, parentbilling.ErrSEPADebitsChanged)
	}
	return fmt.Errorf("%s: %w", action, err)
}

// dropSupersededSEPADebitFilesSQL drops the unsubmitted downloads of the
// run that share a debit or an installment with the submitted message:
// they are evidence of nothing the bank received and hold families' accounts.
// The request role deletes no file; the owner function is scoped to the
// Träger of the transaction (ADR 0187).
const dropSupersededSEPADebitFilesSQL = `SELECT parent_billing.drop_superseded_sepa_debit_files(?, ?, ?, ?, ?::bigint[], ?::bigint[])`

// submitSEPADebitFileSQL marks the stored file submitted; a message without
// a stored download keeps the creditor account of the submission.
const submitSEPADebitFileSQL = `INSERT INTO parent_billing.sepa_debit_files AS kept (organization_id, message_id, run_kind, run_id,
	creditor_name, creditor_iban, creditor_bic, creditor_id, debit_ids, notice_ids, mandate_ids, submitted_at, submitted_by_account_id)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, clock_timestamp(), ?)
	ON CONFLICT (organization_id, message_id) DO UPDATE SET submitted_at = EXCLUDED.submitted_at,
		submitted_by_account_id = EXCLUDED.submitted_by_account_id
	WHERE kept.submitted_at IS NULL`

// SubmitSEPADebitFile marks the file of a message submitted and drops the
// downloads it superseded.
func (s *Store) SubmitSEPADebitFile(ctx context.Context, organizationID, accountID int64, file parentbilling.StoredSEPADebitFile) error {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return err
	}
	if _, err := db.NewRaw(dropSupersededSEPADebitFilesSQL, organizationID, file.RunKind, file.RunID, file.MessageID,
		members(file.DebitIDs), members(file.NoticeIDs)).Exec(ctx); err != nil {
		return fmt.Errorf("drop the superseded SEPA debit files: %w", err)
	}
	if _, err := db.NewRaw(submitSEPADebitFileSQL, organizationID, file.MessageID, file.RunKind, file.RunID,
		file.CreditorName, file.CreditorIBAN, file.CreditorBIC, file.CreditorID, members(file.DebitIDs), members(file.NoticeIDs),
		members(file.MandateIDs), accountRef(accountID)).Exec(ctx); err != nil {
		return sepaDebitFileError("keep the submitted SEPA debit file", err)
	}
	return nil
}

// members binds the member IDs of a file as an array, empty and never NULL.
func members(ids []int64) any {
	if ids == nil {
		ids = []int64{}
	}
	return pgdialect.Array(ids)
}

type sepaDebitFileRow struct {
	MessageID    string  `bun:"message_id"`
	RunKind      string  `bun:"run_kind"`
	RunID        int64   `bun:"run_id"`
	FileSealed   []byte  `bun:"file_sealed"`
	FileSHA256   *string `bun:"file_sha256"`
	CreditorName string  `bun:"creditor_name"`
	CreditorIBAN string  `bun:"creditor_iban"`
	CreditorBIC  string  `bun:"creditor_bic"`
	CreditorID   string  `bun:"creditor_id"`
}

const returnSEPADebitFilesSQL = `SELECT message_id, run_kind, run_id, file_sealed, file_sha256, creditor_name, creditor_iban, creditor_bic, creditor_id
	FROM parent_billing.sepa_debit_files WHERE organization_id = ? AND submitted_at IS NOT NULL ORDER BY message_id`

// ReturnSEPADebitFiles reads the stored files of the submitted messages.
func (s *Store) ReturnSEPADebitFiles(ctx context.Context, organizationID int64) ([]parentbilling.StoredSEPADebitFile, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	var rows []sepaDebitFileRow
	if err := db.NewRaw(returnSEPADebitFilesSQL, organizationID).Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("read the SEPA debit files of the Rückgabe: %w", err)
	}
	files := make([]parentbilling.StoredSEPADebitFile, len(rows))
	for i, row := range rows {
		files[i] = parentbilling.StoredSEPADebitFile{MessageID: row.MessageID, RunKind: row.RunKind, RunID: row.RunID,
			Sealed: row.FileSealed, CreditorName: row.CreditorName, CreditorIBAN: row.CreditorIBAN, CreditorBIC: row.CreditorBIC,
			CreditorID: row.CreditorID, Submitted: true}
		if row.FileSHA256 != nil {
			files[i].SHA256 = *row.FileSHA256
		}
	}
	return files, nil
}

// DeleteExpiredSEPADebitDownloads deletes, across every Träger, the
// unsubmitted downloads older than 30 days (ADR 0187): they are evidence of
// nothing the bank received and hold families' accounts. Only the
// administrative transaction of the hourly cleanup may run it; the owner
// function never touches a submitted original.
func (s *Store) DeleteExpiredSEPADebitDownloads(ctx context.Context) (int, error) {
	if !tenant.IsAdminTx(ctx) {
		return 0, errors.New("parent billing postgres: the SEPA download expiry requires the administrative transaction")
	}
	transaction, ok := tenant.TransactionFromContext(ctx)
	if !ok {
		return 0, errors.New("parent billing postgres: transaction is required")
	}
	tx, ok := transaction.(bun.Tx)
	if !ok {
		return 0, fmt.Errorf("parent billing postgres: unsupported transaction %T", transaction)
	}
	var deleted int
	if err := tx.NewRaw(`SELECT parent_billing.delete_expired_sepa_debit_downloads()`).Scan(ctx, &deleted); err != nil {
		return 0, fmt.Errorf("parent billing postgres: expire the SEPA debit downloads: %w", err)
	}
	return deleted, nil
}
