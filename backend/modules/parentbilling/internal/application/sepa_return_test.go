package application

import (
	"context"
	"errors"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-ogs/project-phoenix/modules/parentbilling"
	testpkg "github.com/moto-ogs/project-phoenix/test"
)

// The Träger package carries the PAIN.008 file of each submitted message
// (#6314, ADR 0187): the stored original, else regenerated from the stored
// debits. The opened accounts of the mandates and debits come with their
// catalog rows (ADR 0192), not from here.

// returnStoreFake answers the messages as the store does: every message
// with a submitted debit or a stored original, ordered by message ID, then
// the debits and the stored file of one message. log records each read.
type returnStoreFake struct {
	debits []parentbilling.SealedSEPADebit
	files  []parentbilling.StoredSEPADebitFile
	log    *[]string
}

func (s returnStoreFake) record(entry string) {
	if s.log != nil {
		*s.log = append(*s.log, entry)
	}
}

func (s returnStoreFake) ReturnSEPAMessages(context.Context, int64) ([]string, error) {
	s.record("messages")
	seen := map[string]bool{}
	for _, entry := range s.debits {
		if entry.Debit.Status == parentbilling.DebitSubmitted && entry.Debit.MessageID != "" {
			seen[entry.Debit.MessageID] = true
		}
	}
	for _, file := range s.files {
		if file.Sealed != nil {
			seen[file.MessageID] = true
		}
	}
	messages := make([]string, 0, len(seen))
	for messageID := range seen {
		messages = append(messages, messageID)
	}
	sort.Strings(messages)
	return messages, nil
}

func (s returnStoreFake) ReturnSEPAMessageDebits(_ context.Context, _ int64, messageID string) ([]parentbilling.SealedSEPADebit, error) {
	s.record("debits " + messageID)
	var debits []parentbilling.SealedSEPADebit
	for _, entry := range s.debits {
		if entry.Debit.Status == parentbilling.DebitSubmitted && entry.Debit.MessageID == messageID {
			debits = append(debits, entry)
		}
	}
	return debits, nil
}

func (s returnStoreFake) ReturnSEPADebitFile(_ context.Context, _ int64, messageID string) (parentbilling.StoredSEPADebitFile, bool, error) {
	s.record("file " + messageID)
	for _, file := range s.files {
		if file.MessageID == messageID {
			return file, true, nil
		}
	}
	return parentbilling.StoredSEPADebitFile{}, false, nil
}

// returnedFiles collects what SEPAReturn writes.
type returnedFiles struct {
	Files []parentbilling.SEPAReturnFile
}

func returnFiles(r *SEPAReturn, organizationID int64) (returnedFiles, error) {
	var collected returnedFiles
	err := r.SEPAReturn(context.Background(), organizationID, func(file parentbilling.SEPAReturnFile) error {
		collected.Files = append(collected.Files, file)
		return nil
	})
	return collected, err
}

type creditorAccountFake struct {
	account *parentbilling.LetterheadBankAccount
}

func (f creditorAccountFake) CreditorAccount(context.Context, int64) (*parentbilling.LetterheadBankAccount, error) {
	return f.account, nil
}

var sjrCreditorAccount = &parentbilling.LetterheadBankAccount{
	AccountHolder: "Stadtjugendring Pforzheim e. V.", IBAN: "DE75 5121 0800 1245 1261 99", BIC: "SOLADES1PFO"}

// creDtTm is the creation time of a file: the download or, regenerated, the
// confirmation of its submission.
var creDtTm = regexp.MustCompile(`<CreDtTm>[^<]*</CreDtTm>`)

func TestSEPAReturnRegeneratesTheSubmittedFile(t *testing.T) {
	t.Parallel()
	store := sjrDebits()
	debits := newSJRDebits(store)
	downloaded, err := debits.SEPADebitFile(context.Background(), sjrReviewer, parentbilling.DebitRunCare, 3)
	require.NoError(t, err)
	run, err := debits.SEPADebitRun(context.Background(), sjrReviewer, parentbilling.DebitRunCare, 3)
	require.NoError(t, err)
	_, err = debits.SubmitSEPADebits(context.Background(), sjrReviewer, parentbilling.DebitRunCare, 3, run.MessageID)
	require.NoError(t, err)
	// A pending debit has no file yet; the office still downloads it while
	// the Träger is active.
	pending := sealedDebit(4, 14, "RE-2027-0004", 1000, parentbilling.SequenceTypeRecurring, "Ida Pending", "DE44500105175407324931")

	returned, err := returnFiles(NewSEPAReturn(returnStoreFake{
		debits: append(append([]parentbilling.SealedSEPADebit{}, store.debits...), pending),
	}, creditorAccountFake{account: sjrCreditorAccount}, prefixCipher{}, testpkg.ContentFingerprint), 7)
	require.NoError(t, err)

	require.Len(t, returned.Files, 1, "one file per submitted message, none for the pending debit")
	files := map[string][]byte{}
	for _, file := range returned.Files {
		files[file.Path] = file.Data
	}
	require.Contains(t, files, run.MessageID+".xml")
	regenerated := files[run.MessageID+".xml"]
	testpkg.AssertXMLMatchesSchema(t, regenerated, filepath.Join("testdata", "sepa", "pain.008.001.08.xsd"))
	assert.Equal(t, string(creDtTm.ReplaceAll(downloaded.Data, nil)), string(creDtTm.ReplaceAll(regenerated, nil)),
		"the regenerated file is the submitted one: message ID, batches, amounts, dates and accounts")
	assert.Contains(t, string(regenerated), "<CreDtTm>2026-11-20T10:00:00</CreDtTm>", "created at the confirmation of its submission")

}

// Without the Träger's account no file can be written; the Rückgabe fails
// instead of leaving the files out.
func TestSEPAReturnNeedsTheCreditorAccountForSubmittedFiles(t *testing.T) {
	t.Parallel()
	store := sjrDebits()
	debits := newSJRDebits(store)
	run, err := debits.SEPADebitRun(context.Background(), sjrReviewer, parentbilling.DebitRunCare, 3)
	require.NoError(t, err)
	_, err = debits.SubmitSEPADebits(context.Background(), sjrReviewer, parentbilling.DebitRunCare, 3, run.MessageID)
	require.NoError(t, err)
	_, err = returnFiles(NewSEPAReturn(returnStoreFake{debits: store.debits}, creditorAccountFake{}, prefixCipher{}, testpkg.ContentFingerprint), 7)
	require.ErrorIs(t, err, parentbilling.ErrSEPACreditorAccountMissing)

	// A sealed value of another Träger does not open under this one.
	_, err = returnFiles(NewSEPAReturn(returnStoreFake{debits: store.debits}, creditorAccountFake{account: sjrCreditorAccount}, prefixCipher{}, testpkg.ContentFingerprint), 8)
	require.Error(t, err)
}

// The download stores the file it hands out, sealed, with its checksum and
// the creditor account in it (#6314, ADR 0187). The Rückgabe ships that
// original: a later change of the Träger's default account does not rewrite
// the history of a submitted file.
func TestSEPAReturnShipsTheStoredOriginalAfterTheDefaultAccountChanged(t *testing.T) {
	t.Parallel()
	store := sjrDebits()
	debits := newSJRDebits(store)
	downloaded, err := debits.SEPADebitFile(context.Background(), sjrReviewer, parentbilling.DebitRunCare, 3)
	require.NoError(t, err)
	run, err := debits.SEPADebitRun(context.Background(), sjrReviewer, parentbilling.DebitRunCare, 3)
	require.NoError(t, err)
	_, err = debits.SubmitSEPADebits(context.Background(), sjrReviewer, parentbilling.DebitRunCare, 3, run.MessageID)
	require.NoError(t, err)
	stored := store.storedFiles()
	require.Len(t, stored, 1, "the download stored its file")
	assert.True(t, stored[0].Submitted)

	changed := &parentbilling.LetterheadBankAccount{AccountHolder: "Neues Konto e. V.", IBAN: "DE12 5001 0517 0648 4898 90", BIC: "INGDDEFFXXX"}
	returned, err := returnFiles(NewSEPAReturn(returnStoreFake{debits: store.debits, files: stored},
		creditorAccountFake{account: changed}, prefixCipher{}, testpkg.ContentFingerprint), 7)
	require.NoError(t, err)
	var file *parentbilling.SEPAReturnFile
	for i := range returned.Files {
		if returned.Files[i].Path == run.MessageID+".xml" {
			file = &returned.Files[i]
		}
	}
	require.NotNil(t, file)
	assert.Equal(t, downloaded.Data, file.Data, "the stored original, byte for byte")
	assert.Equal(t, parentbilling.SEPAReturnOriginal, file.Origin)
	assert.NotContains(t, string(file.Data), "DE12500105170648489890", "not the account of today")

	// A file submitted without a stored original is reconstructed with the
	// creditor account recorded at its submission.
	snapshot := stored[0]
	snapshot.Sealed, snapshot.SHA256 = nil, ""
	returned, err = returnFiles(NewSEPAReturn(returnStoreFake{debits: store.debits, files: []parentbilling.StoredSEPADebitFile{snapshot}},
		creditorAccountFake{account: changed}, prefixCipher{}, testpkg.ContentFingerprint), 7)
	require.NoError(t, err)
	reconstructed := returned.Files[len(returned.Files)-1]
	assert.Equal(t, parentbilling.SEPAReturnReconstructed, reconstructed.Origin)
	assert.Contains(t, reconstructed.Reason, "Einreichung")
	assert.Contains(t, string(reconstructed.Data), "<IBAN>DE75512108001245126199</IBAN>", "the creditor account of the submission")
	assert.Equal(t, string(creDtTm.ReplaceAll(downloaded.Data, nil)), string(creDtTm.ReplaceAll(reconstructed.Data, nil)))

	// An older file without either is reconstructed with today's account and says so.
	returned, err = returnFiles(NewSEPAReturn(returnStoreFake{debits: store.debits},
		creditorAccountFake{account: changed}, prefixCipher{}, testpkg.ContentFingerprint), 7)
	require.NoError(t, err)
	older := returned.Files[len(returned.Files)-1]
	assert.Equal(t, parentbilling.SEPAReturnReconstructed, older.Origin)
	assert.Contains(t, older.Reason, "heutige Standardkonto")
	assert.Contains(t, string(older.Data), "<IBAN>DE12500105170648489890</IBAN>")

	// A stored original whose checksum does not match is refused, never shipped.
	tampered := stored[0]
	tampered.SHA256 = strings.Repeat("0", 64)
	_, err = returnFiles(NewSEPAReturn(returnStoreFake{debits: store.debits, files: []parentbilling.StoredSEPADebitFile{tampered}},
		creditorAccountFake{account: changed}, prefixCipher{}, testpkg.ContentFingerprint), 7)
	require.Error(t, err)
}

// The Rückgabe holds one file in memory at a time: it reads the debits and
// the stored file of a message, hands the file to the writer and only then
// reads the next message (#6317). A writer that fails stops the run.
func TestSEPAReturnWritesEachFileBeforeItReadsTheNextMessage(t *testing.T) {
	t.Parallel()
	store := sjrDebits()
	debits := newSJRDebits(store)
	_, err := debits.SEPADebitFile(context.Background(), sjrReviewer, parentbilling.DebitRunCare, 3)
	require.NoError(t, err)
	run, err := debits.SEPADebitRun(context.Background(), sjrReviewer, parentbilling.DebitRunCare, 3)
	require.NoError(t, err)
	_, err = debits.SubmitSEPADebits(context.Background(), sjrReviewer, parentbilling.DebitRunCare, 3, run.MessageID)
	require.NoError(t, err)
	const older = "MOTO-20261119-OLDEROLDEROLDEROLDER"
	legacy := sealedDebit(5, 15, "RE-2026-0005", 3000, parentbilling.SequenceTypeRecurring, "Ida Legacy", "DE44500105175407324931")
	legacy.Debit.Status, legacy.Debit.MessageID, legacy.Debit.SubmittedAt = parentbilling.DebitSubmitted, older, store.debits[0].Debit.SubmittedAt

	var log []string
	fake := returnStoreFake{debits: append([]parentbilling.SealedSEPADebit{legacy}, store.debits...), files: store.storedFiles(), log: &log}
	sepaReturn := NewSEPAReturn(fake, creditorAccountFake{account: sjrCreditorAccount}, prefixCipher{}, testpkg.ContentFingerprint)
	require.NoError(t, sepaReturn.SEPAReturn(context.Background(), 7, func(file parentbilling.SEPAReturnFile) error {
		log = append(log, "write "+file.Path)
		return nil
	}))
	require.Equal(t, []string{"messages",
		"file " + older, "debits " + older, "write " + older + ".xml",
		"file " + run.MessageID, "write " + run.MessageID + ".xml"}, log, "the stored original needs no debits")

	log = nil
	refused := errors.New("disk full")
	err = sepaReturn.SEPAReturn(context.Background(), 7, func(parentbilling.SEPAReturnFile) error { return refused })
	require.ErrorIs(t, err, refused)
	require.Equal(t, []string{"messages", "file " + older, "debits " + older}, log, "nothing read after the failed write")
}
