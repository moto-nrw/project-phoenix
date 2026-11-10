package parentbilling

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// Import of existing SEPA mandates from a CSV file (#6198, ADR 0171 point
// 1). The Träger takes over the mandates it collected with before moto,
// for example from OGS Connect. The export format is open, so the office
// maps the columns of its file to the fields of a mandate; the preview
// shows every row with what moto would do before anything is stored. The
// mandates are written by the one owner of the mandates (SEPAMandates,
// Source MandateFromImport). The file itself is never stored: the trail of
// each imported mandate keeps the name of the file and the line it came
// from.

// Fields of a mandate a column of the file can be mapped to.
const (
	ImportFieldReference     = "reference"
	ImportFieldSignedOn      = "signed_on"
	ImportFieldIBAN          = "iban"
	ImportFieldBIC           = "bic"
	ImportFieldAccountHolder = "account_holder"
	ImportFieldPayer         = "payer"
	ImportFieldCreditorID    = "creditor_id"
	ImportFieldSequence      = "sequence"
	ImportFieldLastCollected = "last_collected_on"
	// ImportFieldDebtorNumber is the debtor number of the family in the
	// finance system of the Träger (OGS Connect "kinder-Debitorennummer",
	// #4693); optional, it goes to the family, not to the mandate.
	ImportFieldDebtorNumber = "debtor_number"
)

// What the import does with the debtor number of a row (#4693).
const (
	// DebtorNumberNew is taken over for the family.
	DebtorNumberNew = "new"
	// DebtorNumberSame is the number the family already has.
	DebtorNumberSame = "same"
	// DebtorNumberSiblingsDiffer is a family whose rows (siblings) carry
	// different numbers: the import takes over none of them.
	DebtorNumberSiblingsDiffer = "siblings_differ"
	// DebtorNumberDiffers is a family that has another number in moto: the
	// import never overwrites it.
	DebtorNumberDiffers = "differs"
	// DebtorNumberInvalid is too long or has a line break.
	DebtorNumberInvalid = "invalid"
	// DebtorNumberNoFamily is a row whose family moto does not know.
	DebtorNumberNoFamily = "no_family"
)

// How the payer column names the family of a mandate.
const (
	// PayerByEmail is the mail address of the contract party, the family
	// key of the Abrechnung (FamilyKey, ADR 0167).
	PayerByEmail = "email"
	// PayerByContract is the number of the Betreuungsvertrag in moto.
	PayerByContract = "contract_id"
)

// What the import does with a row.
const (
	// ImportRowReady is imported as it is.
	ImportRowReady = "ready"
	// ImportRowClarify needs the office to say whether the mandate was
	// collected with before; it is imported only with that answer.
	ImportRowClarify = "clarify"
	// ImportRowDuplicate is a mandate the Träger already has with the same
	// reference, family, date and creditor: a repeated import skips it.
	ImportRowDuplicate = "duplicate"
	// ImportRowConflict contradicts a mandate the Träger has or another row;
	// it is never overwritten.
	ImportRowConflict = "conflict"
	// ImportRowInvalid misses a field or carries a value that is no mandate.
	ImportRowInvalid = "invalid"
	// ImportRowImported is a row the import stored.
	ImportRowImported = "imported"
)

// Problems of a row. Each names a field and a code; none repeats a value.
const (
	ImportProblemMissing           = "missing"
	ImportProblemInvalid           = "invalid"
	ImportProblemFuture            = "future"
	ImportProblemBeforeSigned      = "before_signed"
	ImportProblemBICRequired       = "bic_required"
	ImportProblemTooLong           = "too_long"
	ImportProblemPayerUnknown      = "payer_unknown"
	ImportProblemFamilyUnknown     = "family_unknown"
	ImportProblemCreditorMismatch  = "creditor_mismatch"
	ImportProblemReferenceTaken    = "reference_taken"
	ImportProblemReferenceRepeated = "reference_repeated"
	ImportProblemPayerHasMandate   = "payer_has_mandate"
	ImportProblemPayerRepeated     = "payer_repeated"
	ImportProblemSequenceUnknown   = "sequence_unknown"
	ImportProblemSequenceConflict  = "sequence_contradicts"
	ImportProblemOneOff            = "one_off"
	ImportProblemExpired           = "expired"
	// ImportProblemAccountDiffers is a mandate the Träger has with the same
	// reference, family, date and creditor but another IBAN or account
	// holder: a conflict, never a duplicate and never overwritten.
	ImportProblemAccountDiffers = "account_differs"
)

// Hints of a row the import still takes over.
const (
	// ImportHintLastCollectionUnknown is a mandate collected with before
	// whose last collection the file does not tell: the 36 months until it
	// lapses count from the import.
	ImportHintLastCollectionUnknown = "last_collection_unknown"
)

// Limits of an import file.
const (
	MaxImportFileBytes = 2 << 20
	MaxImportRows      = 5000
)

var (
	// ErrImportFileInvalid reports a file that is no readable CSV with a
	// header and at least one row.
	ErrImportFileInvalid = errors.New("the import file is no readable CSV file with a header row and data rows")
	// ErrImportChanged reports an import whose rows no longer are what the
	// office confirmed in the preview: the mandates of the Träger, its
	// contracts or the answers changed since. The office loads the preview
	// again.
	ErrImportChanged = errors.New("the import differs from the confirmed preview")
	// ErrImportFileTooLarge reports a file over MaxImportFileBytes or with
	// more than MaxImportRows rows.
	ErrImportFileTooLarge = errors.New("the import file is too large")
)

// SEPAMandateImportColumns names per field the header of the column it
// comes from; empty leaves the field unmapped.
type SEPAMandateImportColumns struct {
	Reference     string `json:"reference"`
	SignedOn      string `json:"signed_on"`
	IBAN          string `json:"iban"`
	BIC           string `json:"bic"`
	AccountHolder string `json:"account_holder"`
	Payer         string `json:"payer"`
	CreditorID    string `json:"creditor_id"`
	Sequence      string `json:"sequence"`
	LastCollected string `json:"last_collected_on"`
	DebtorNumber  string `json:"debtor_number"`
}

// SEPAMandateImportMapping is how the rows of a file become mandates.
// PayerKey says what the payer column holds. CreditorFromTraeger is the
// explicit choice that every mandate of the file was given to the Träger's
// creditor identifier, for a file without that column.
type SEPAMandateImportMapping struct {
	Columns             SEPAMandateImportColumns `json:"columns"`
	PayerKey            string                   `json:"payer_key"`
	CreditorFromTraeger bool                     `json:"creditor_from_traeger"`
	// ConfirmedCreditorID is the creditor identifier the office saw when it
	// chose CreditorFromTraeger. The choice holds only while the Träger
	// still has that identifier.
	ConfirmedCreditorID string `json:"confirmed_creditor_id"`
}

// MaxImportFileNameLength bounds the file name the trail keeps.
const MaxImportFileNameLength = 255

// SEPAMandateImportRequest is one look at a file: the preview when Apply is
// not set, the import of its ready rows when it is. Mapping nil takes the
// mapping moto suggests from the headers. Delimiter is ";", "," or "\t",
// empty detects it. Sequences answers per line of a row to clarify whether
// the mandate was collected with before (SequenceFirst or
// SequenceRecurring).
type SEPAMandateImportRequest struct {
	Content   []byte
	FileName  string
	Delimiter string
	Mapping   *SEPAMandateImportMapping
	Sequences map[int]string
	Apply     bool
	// Fingerprint is the fingerprint of the preview the office confirmed;
	// Apply refuses with ErrImportChanged when the rows differ from it.
	Fingerprint string
}

// SEPAMandateImportProblem is one problem of a row: the field and the code.
type SEPAMandateImportProblem struct {
	Field string `json:"field"`
	Code  string `json:"code"`
}

// SEPAMandateImportRow is a row of the file as the import reads it. The
// IBAN shows only as its mask, and only when it is valid.
type SEPAMandateImportRow struct {
	Line            int                        `json:"line"`
	Status          string                     `json:"status"`
	Reference       string                     `json:"reference"`
	SignedOn        *calendar.Date             `json:"signed_on"`
	IBANMasked      string                     `json:"iban_masked"`
	BIC             string                     `json:"bic"`
	AccountHolder   string                     `json:"account_holder"`
	ContractID      int64                      `json:"contract_id,omitempty"`
	PayerFirstName  string                     `json:"payer_first_name"`
	PayerLastName   string                     `json:"payer_last_name"`
	Sequence        string                     `json:"sequence"`
	LastCollectedOn *calendar.Date             `json:"last_collected_on"`
	MandateID       int64                      `json:"mandate_id,omitempty"`
	Problems        []SEPAMandateImportProblem `json:"problems"`
	Hints           []string                   `json:"hints"`
	// DebtorNumber is the debtor number of the row and DebtorNumberStatus
	// what the import does with it (#4693); empty without the column.
	DebtorNumber       string `json:"debtor_number,omitempty"`
	DebtorNumberStatus string `json:"debtor_number_status,omitempty"`
}

// SEPAMandateImportCounts counts the rows per status.
type SEPAMandateImportCounts struct {
	Ready     int `json:"ready"`
	Clarify   int `json:"clarify"`
	Duplicate int `json:"duplicate"`
	Conflict  int `json:"conflict"`
	Invalid   int `json:"invalid"`
	Imported  int `json:"imported"`
	// DebtorNumbers counts the families whose debtor number the import
	// takes over; DebtorConflicts the rows whose number it leaves.
	DebtorNumbers   int `json:"debtor_numbers"`
	DebtorConflicts int `json:"debtor_conflicts"`
}

// FamilyDebtorNumber is a debtor number the import sets for a family.
type FamilyDebtorNumber struct {
	FamilyEmail  string
	ContractID   int64
	DebtorNumber string
}

// SEPAMandateImport is the answer to a look at a file: how it was read, the
// mapping used and every row.
type SEPAMandateImport struct {
	Encoding  string                   `json:"encoding"`
	Delimiter string                   `json:"delimiter"`
	Headers   []string                 `json:"headers"`
	Mapping   SEPAMandateImportMapping `json:"mapping"`
	// Unmapped names the required fields without a column; the rows are
	// read once each has one.
	Unmapped   []string `json:"unmapped"`
	CreditorID string   `json:"creditor_id"`
	// Fingerprint names the rows and what the import does with each; the
	// import takes over only the preview it was given (#6198).
	Fingerprint string                  `json:"fingerprint,omitempty"`
	Applied     bool                    `json:"applied"`
	Counts      SEPAMandateImportCounts `json:"counts"`
	Rows        []SEPAMandateImportRow  `json:"rows"`
}

// ImportedSEPAMandate is a mandate an import stored and the line of the
// file it came from, as the store records it.
type ImportedSEPAMandate struct {
	Mandate SEPAMandate
	Line    int
}

// SEPAMandateImportEntry is one import in the log: when, by whom, which
// file and how many mandates it took over.
type SEPAMandateImportEntry struct {
	File       string    `json:"file"`
	AccountID  *int64    `json:"account_id"`
	ImportedAt time.Time `json:"imported_at"`
	Mandates   int       `json:"mandates"`
}

// SEPAMandateImports is the import of existing mandates, part of
// SEPAMandates.
type SEPAMandateImports interface {
	// ImportSEPAMandates reads a file with the mapping and answers every
	// row; with Apply it stores the ready rows and the clarified ones in
	// one go and records each with the name and the line of the file.
	ImportSEPAMandates(ctx context.Context, reviewer HolidayReviewer, request SEPAMandateImportRequest) (SEPAMandateImport, error)
	// SEPAMandateImportLog lists the imports of the Träger, newest first.
	SEPAMandateImportLog(ctx context.Context, reviewer HolidayReviewer) ([]SEPAMandateImportEntry, error)
}
