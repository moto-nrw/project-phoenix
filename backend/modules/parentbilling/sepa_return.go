package parentbilling

import "context"

// The PAIN.008 files of a Träger in its Rückgabe at the end of the contract
// (#6314, #4720 Slice 3, AVV Ziff. 8.2/8.4). The Träger package of the tenant
// export carries the mandates and debits with their opened IBAN and account
// holder (ADR 0184). After the deactivation the Träger-Büro can no longer
// download its PAIN.008 files, so the package also carries them: the stored
// original where moto kept it (ADR 0187), otherwise regenerated from the
// stored submitted debits.

// SEPAReturnFile is the PAIN.008 file of one submitted message. It holds
// full bank data (IBAN, account holder).
type SEPAReturnFile struct {
	// Path is the file name, the message ID with .xml.
	Path string
	Data []byte
	// Origin and Reason state whether it is the stored original or
	// reconstructed, and why.
	Origin string
	Reason string
}

// The origins of a PAIN.008 file in the Rückgabe.
const (
	SEPAReturnOriginal      = "Original"
	SEPAReturnReconstructed = "rekonstruiert"
)

// StoredSEPADebitFile is what moto kept of the PAIN.008 file of one message
// (#6314, ADR 0187): the exact bytes of the last download, sealed with the
// bank key and bound to the Träger and the message ID, their SHA-256 and the
// creditor account in the file. Submitted is set by the confirmation of the
// submission; from then on the row never changes. Sealed is nil when the
// file was submitted without a stored download: then only the creditor
// account of the submission is kept.
type StoredSEPADebitFile struct {
	MessageID    string
	RunKind      string
	RunID        int64
	Sealed       []byte
	SHA256       string
	CreditorName string
	CreditorIBAN string
	CreditorBIC  string
	CreditorID   string
	Submitted    bool
	// DebitIDs, NoticeIDs and MandateIDs are the members of the file: its
	// debits, the notices of its pending installments and the mandates
	// whose account it holds. The stored file goes with the first of them.
	DebitIDs   []int64
	NoticeIDs  []int64
	MandateIDs []int64
}

// SEPAReturn writes the PAIN.008 files of one Träger for its Rückgabe, one
// submitted message at a time: it reads a message, hands its file to write
// and reads the next one only after write returned, so at most one opened
// file is in memory. An error of write stops it. It runs in the Träger
// transaction of organizationID, which the caller opens, and changes
// nothing. Never bind it to HTTP: the internal Rückgabe procedure (tenant
// export --organization) is its only caller.
type SEPAReturn interface {
	SEPAReturn(ctx context.Context, organizationID int64, write func(SEPAReturnFile) error) error
}

// SEPADebitDownloadRetention expires, across every Träger, the unsubmitted
// PAIN.008 downloads 30 days after the download (ADR 0187): they are
// evidence of nothing the bank received and hold families' accounts. A
// submitted original never expires here. It runs in the administrative
// transaction of the hourly cleanup.
type SEPADebitDownloadRetention interface {
	DeleteExpiredSEPADebitDownloads(ctx context.Context) (int, error)
}

// SEPACreditorAccounts answers the default bank account of a Träger the
// debits were credited to, nil without one. The root binds it to the master
// data of Organisation & Tenancy.
type SEPACreditorAccounts interface {
	CreditorAccount(ctx context.Context, organizationID int64) (*LetterheadBankAccount, error)
}
