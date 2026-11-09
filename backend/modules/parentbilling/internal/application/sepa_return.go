package application

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/sharedkernel/sepa"
)

// The PAIN.008 files in the Träger package of the tenant export (#6314,
// ADR 0187). The rows of the mandates and debits, with their opened IBAN and
// account holder, come with the catalog export (ADR 0184); this owner adds
// only the files those rows cannot carry. A submitted message
// ships the original file moto stored at its download (ADR 0187), checked
// against its SHA-256. A message submitted without a stored download is
// regenerated from its stored debits: the message ID, the submitted amounts,
// sequence types and collection days, the account each debit copied at the
// release and its creditor identifier, with the creditor account recorded at
// its submission, or for messages submitted before moto kept either, the
// Träger's default account now. Batches and transactions come in the order
// of the download; the creation time is the confirmation of the submission.
// Each file names its origin and why.

// SEPAReturnStore reads the sealed debits and stored files of one Träger,
// one message at a time. Every method runs in the Träger transaction of
// organizationID.
type SEPAReturnStore interface {
	// ReturnSEPAMessages answers the ID of every submitted message, ordered
	// by message ID: each with a submitted debit and each whose stored
	// original stayed, also after its last debit was deleted (a submitted
	// file stays while an installment notice of it stays).
	ReturnSEPAMessages(ctx context.Context, organizationID int64) ([]string, error)
	// ReturnSEPAMessageDebits reads the submitted debits of one message with
	// their sealed account, as the file orders them.
	ReturnSEPAMessageDebits(ctx context.Context, organizationID int64, messageID string) ([]parentbilling.SealedSEPADebit, error)
	// ReturnSEPADebitFile reads the stored file of one submitted message;
	// found is false without one.
	ReturnSEPADebitFile(ctx context.Context, organizationID int64, messageID string) (file parentbilling.StoredSEPADebitFile, found bool, err error)
}

// The reasons a PAIN.008 file of the Rückgabe names for its origin.
const (
	reasonOriginal = "Original: die bei moto gespeicherte Datei des letzten Downloads vor der Einreichung, Prüfsumme (SHA-256) geprüft."
	reasonSnapshot = "Rekonstruiert: Das Original wurde nicht gespeichert. Neu erzeugt aus den gespeicherten Lastschriften " +
		"mit dem Gläubigerkonto der Einreichung. Abweichend vom Original ist nur die Erstellzeit (Bestätigung der Einreichung)."
	reasonLegacy = "Rekonstruiert: vor der Speicherung der Originale eingereicht. Neu erzeugt aus den gespeicherten Lastschriften. " +
		"Abweichend vom Original: Die Erstellzeit ist die Bestätigung der Einreichung, das Gläubigerkonto ist das heutige Standardkonto des Trägers."
)

// SEPAReturn serves parentbilling.SEPAReturn.
type SEPAReturn struct {
	store       SEPAReturnStore
	creditors   parentbilling.SEPACreditorAccounts
	cipher      parentbilling.BankDataCipher
	fingerprint func([]byte) string
}

var _ parentbilling.SEPAReturn = (*SEPAReturn)(nil)

// NewSEPAReturn builds the bank data of the Rückgabe.
func NewSEPAReturn(store SEPAReturnStore, creditors parentbilling.SEPACreditorAccounts, cipher parentbilling.BankDataCipher,
	fingerprint func([]byte) string,
) *SEPAReturn {
	if store == nil || creditors == nil || cipher == nil || fingerprint == nil {
		panic("parent billing: the store, the creditor accounts, the cipher and the fingerprint are required")
	}
	return &SEPAReturn{store: store, creditors: creditors, cipher: cipher, fingerprint: fingerprint}
}

// SEPAReturn writes the PAIN.008 file of each submitted message: the
// stored original, or else the file regenerated from the debits. It reads
// one message, writes its file and only then reads the next.
func (r *SEPAReturn) SEPAReturn(ctx context.Context, organizationID int64, write func(parentbilling.SEPAReturnFile) error) error {
	if organizationID <= 0 {
		return parentbilling.ErrSEPADebitRunNotFound
	}
	messages, err := r.store.ReturnSEPAMessages(ctx, organizationID)
	if err != nil {
		return err
	}
	today := func() (*parentbilling.LetterheadBankAccount, error) { return r.creditorAccount(ctx, organizationID) }
	for _, messageID := range messages {
		kept, found, err := r.store.ReturnSEPADebitFile(ctx, organizationID, messageID)
		if err != nil {
			return err
		}
		var entries []parentbilling.SealedSEPADebit
		if !found || kept.Sealed == nil { // only a reconstruction reads the debits
			if entries, err = r.store.ReturnSEPAMessageDebits(ctx, organizationID, messageID); err != nil {
				return err
			}
		}
		file, err := r.messageFile(organizationID, messageID, entries, kept, found, today)
		if err != nil {
			return err
		}
		if err := write(file); err != nil {
			return err
		}
	}
	return nil
}

// messageFile is the file of one message with its origin: the stored
// original, also without a surviving debit, else reconstructed with the
// creditor account of the submission, else with today's default account.
func (r *SEPAReturn) messageFile(organizationID int64, messageID string, entries []parentbilling.SealedSEPADebit,
	kept parentbilling.StoredSEPADebitFile, found bool, today func() (*parentbilling.LetterheadBankAccount, error),
) (parentbilling.SEPAReturnFile, error) {
	file := parentbilling.SEPAReturnFile{Path: messageID + ".xml"}
	var err error
	switch {
	case found && kept.Sealed != nil:
		file.Origin, file.Reason = parentbilling.SEPAReturnOriginal, reasonOriginal
		file.Data, err = r.storedOriginal(organizationID, kept)
	case len(entries) == 0:
		return file, fmt.Errorf("parent billing: the SEPA direct debit file %s has neither a stored original nor debits", messageID)
	case found && kept.CreditorIBAN != "":
		file.Origin, file.Reason = parentbilling.SEPAReturnReconstructed, reasonSnapshot
		file.Data, err = r.submittedFile(organizationID, messageID, entries, &parentbilling.LetterheadBankAccount{
			AccountHolder: kept.CreditorName, IBAN: kept.CreditorIBAN, BIC: kept.CreditorBIC})
	default:
		var account *parentbilling.LetterheadBankAccount
		if account, err = today(); err != nil {
			return file, err
		}
		file.Origin, file.Reason = parentbilling.SEPAReturnReconstructed, reasonLegacy
		file.Data, err = r.submittedFile(organizationID, messageID, entries, account)
	}
	return file, err
}

// creditorAccount is the default account of the Träger now, for files
// submitted before moto kept the account of a submission.
func (r *SEPAReturn) creditorAccount(ctx context.Context, organizationID int64) (*parentbilling.LetterheadBankAccount, error) {
	account, err := r.creditors.CreditorAccount(ctx, organizationID)
	if err != nil {
		return nil, fmt.Errorf("parent billing: read the bank account of the Träger: %w", err)
	}
	if account == nil {
		return nil, parentbilling.ErrSEPACreditorAccountMissing
	}
	return account, nil
}

// storedOriginal opens the stored file and checks it against its SHA-256.
func (r *SEPAReturn) storedOriginal(organizationID int64, kept parentbilling.StoredSEPADebitFile) ([]byte, error) {
	data, err := r.cipher.Open(kept.Sealed, debitFileContext(organizationID, kept.MessageID))
	if err != nil {
		return nil, fmt.Errorf("parent billing: open the stored SEPA direct debit file %s: %w", kept.MessageID, err)
	}
	if r.fingerprint(data) != kept.SHA256 {
		return nil, fmt.Errorf("parent billing: the stored SEPA direct debit file %s does not match its checksum", kept.MessageID)
	}
	return data, nil
}

func (r *SEPAReturn) submittedFile(organizationID int64, messageID string, entries []parentbilling.SealedSEPADebit,
	account *parentbilling.LetterheadBankAccount,
) ([]byte, error) {
	collectOn := make(map[int64]calendar.Date, len(entries))
	var submittedAt time.Time
	for _, entry := range entries {
		collectOn[entry.Debit.ID] = entry.Debit.CollectOn
		if entry.Debit.SubmittedAt != nil && entry.Debit.SubmittedAt.After(submittedAt) {
			submittedAt = *entry.Debit.SubmittedAt
		}
	}
	batches, _, err := openDebitBatches(r.cipher, organizationID, messageID, entries, collectOn)
	if err != nil {
		return nil, err
	}
	return renderPain008(debitFileHead{MessageID: messageID, CreatedAt: submittedAt.In(berlin()),
		CreditorName: account.AccountHolder, CreditorIBAN: sepa.Compact(account.IBAN), CreditorBIC: sepa.Compact(account.BIC),
		CreditorID: entries[0].CreditorID}, batches)
}
