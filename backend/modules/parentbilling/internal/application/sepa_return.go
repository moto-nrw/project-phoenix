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

// SEPAReturnStore reads the sealed debits and stored files of one Träger.
// Every method runs in the Träger transaction of organizationID.
type SEPAReturnStore interface {
	// ReturnSEPADebits reads every debit with its sealed account, ordered
	// by message ID, then as the file orders them.
	ReturnSEPADebits(ctx context.Context, organizationID int64) ([]parentbilling.SealedSEPADebit, error)
	// ReturnSEPADebitFiles reads the stored files of the submitted messages.
	ReturnSEPADebitFiles(ctx context.Context, organizationID int64) ([]parentbilling.StoredSEPADebitFile, error)
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

// SEPAReturn answers the PAIN.008 file of each submitted message: the
// stored original, or else the file regenerated from the debits.
func (r *SEPAReturn) SEPAReturn(ctx context.Context, organizationID int64) (parentbilling.SEPAReturnData, error) {
	if organizationID <= 0 {
		return parentbilling.SEPAReturnData{}, parentbilling.ErrSEPADebitRunNotFound
	}
	debits, err := r.store.ReturnSEPADebits(ctx, organizationID)
	if err != nil {
		return parentbilling.SEPAReturnData{}, err
	}
	order, messages := submittedMessages(debits)
	if len(order) == 0 {
		return parentbilling.SEPAReturnData{}, nil
	}
	storedFiles, err := r.store.ReturnSEPADebitFiles(ctx, organizationID)
	if err != nil {
		return parentbilling.SEPAReturnData{}, err
	}
	stored := make(map[string]parentbilling.StoredSEPADebitFile, len(storedFiles))
	for _, file := range storedFiles {
		stored[file.MessageID] = file
	}
	today := func() (*parentbilling.LetterheadBankAccount, error) { return r.creditorAccount(ctx, organizationID) }
	files := make([]parentbilling.SEPAReturnFile, 0, len(order))
	for _, messageID := range order {
		kept, found := stored[messageID]
		file, err := r.messageFile(organizationID, messageID, messages[messageID], kept, found, today)
		if err != nil {
			return parentbilling.SEPAReturnData{}, err
		}
		files = append(files, file)
	}
	return parentbilling.SEPAReturnData{Files: files}, nil
}

// submittedMessages groups the submitted debits per message, in the order
// of the debits.
func submittedMessages(debits []parentbilling.SealedSEPADebit) ([]string, map[string][]parentbilling.SealedSEPADebit) {
	var order []string
	messages := map[string][]parentbilling.SealedSEPADebit{}
	for _, entry := range debits {
		if entry.Debit.Status != parentbilling.DebitSubmitted || entry.Debit.MessageID == "" {
			continue
		}
		if _, seen := messages[entry.Debit.MessageID]; !seen {
			order = append(order, entry.Debit.MessageID)
		}
		messages[entry.Debit.MessageID] = append(messages[entry.Debit.MessageID], entry)
	}
	return order, messages
}

// messageFile is the file of one message with its origin: the stored
// original, else reconstructed with the creditor account of the submission,
// else with today's default account.
func (r *SEPAReturn) messageFile(organizationID int64, messageID string, entries []parentbilling.SealedSEPADebit,
	kept parentbilling.StoredSEPADebitFile, found bool, today func() (*parentbilling.LetterheadBankAccount, error),
) (parentbilling.SEPAReturnFile, error) {
	file := parentbilling.SEPAReturnFile{Path: messageID + ".xml"}
	var err error
	switch {
	case found && kept.Sealed != nil:
		file.Origin, file.Reason = parentbilling.SEPAReturnOriginal, reasonOriginal
		file.Data, err = r.storedOriginal(organizationID, kept)
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
