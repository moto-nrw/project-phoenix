package carecontract

import (
	"context"
	"errors"
	"time"

	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

// Import of children and their existing care contracts from a CSV file
// (#4723, ADR 0088 section 3, ADR 0232). A Träger that starts with moto
// takes over the contracts it concluded before, for example from OGS
// Connect, so it does not begin with an empty list. The export format is
// open: the office maps the columns of its file to the fields of a contract,
// and the preview shows every row with what moto would do before anything is
// stored.
//
// Invariants:
//   - An imported contract is concluded: status aktiv, its first version
//     wirksam from the day the file names, the method papier, with the name
//     of the file, its line and the account that imported it in the trail.
//   - It books modules of its school as every contract does, with the
//     conditions the Träger keeps for them; a module without conditions
//     takes over no contract.
//   - A child that has a running contract at the school gets no second one:
//     a repeated import of a file stores nothing twice.
//   - The file itself is never stored.
//   - The import links no child of a school: a name and a date of birth do
//     not prove who a child is (ADR 0214). The contract keeps its own copy.

// Fields of a contract a column of the file can be mapped to.
const (
	ContractImportFieldChildFirstName = "child_first_name"
	ContractImportFieldChildLastName  = "child_last_name"
	ContractImportFieldChildBirthDate = "child_birth_date"
	ContractImportFieldSchool         = "school"
	ContractImportFieldModule         = "module"
	ContractImportFieldWeekdays       = "weekdays"
	ContractImportFieldEffectiveFrom  = "effective_from"
	ContractImportFieldEndsOn         = "ends_on"
	ContractImportFieldConcludedOn    = "concluded_on"
	ContractImportFieldPartyFirstName = "party_first_name"
	ContractImportFieldPartyLastName  = "party_last_name"
	ContractImportFieldPartyEmail     = "party_email"
	// ContractImportFieldDebtorNumber is the debtor number of the child or of
	// its payer in the finance system of the Träger (#7007); optional, the
	// Elternabrechnung keeps it.
	ContractImportFieldDebtorNumber = "debtor_number"
)

// What the import does with a row.
const (
	// ContractImportReady is taken over as it is.
	ContractImportReady = "ready"
	// ContractImportDuplicate is a child that has a running contract at the
	// school already: a repeated import skips it.
	ContractImportDuplicate = "duplicate"
	// ContractImportConflict contradicts another row of the file.
	ContractImportConflict = "conflict"
	// ContractImportInvalid misses a field or carries a value that is no
	// contract.
	ContractImportInvalid = "invalid"
	// ContractImportImported is a row the import stored.
	ContractImportImported = "imported"
)

// Problems of a row. Each names a field and a code; none repeats a value.
const (
	ContractImportProblemMissing = "missing"
	ContractImportProblemInvalid = "invalid"
	ContractImportProblemTooLong = "too_long"
	ContractImportProblemFuture  = "future"
	// ContractImportProblemUnknown is a school or module the Träger does
	// not have under that name.
	ContractImportProblemUnknown = "unknown"
	// ContractImportProblemAmbiguous is a name several schools, or several
	// modules of the school, carry.
	ContractImportProblemAmbiguous = "ambiguous"
	// ContractImportProblemConditionsMissing is a module without contract
	// conditions; ContractImportProblemConditionsDiffer are modules of one
	// row whose conditions differ.
	ContractImportProblemConditionsMissing = "conditions_missing"
	ContractImportProblemConditionsDiffer  = "conditions_differ"
	// ContractImportProblemNotOffered is a weekday the module does not run
	// on.
	ContractImportProblemNotOffered = "not_offered"
	// ContractImportProblemBeforeStart is an end before the first day.
	ContractImportProblemBeforeStart = "before_start"
	// ContractImportProblemEnded is a contract that ended before today: the
	// import takes over running contracts only.
	ContractImportProblemEnded = "ended"
	// ContractImportProblemOutsidePhase is a first day after the module's
	// enrollment phase has ended.
	ContractImportProblemOutsidePhase = "outside_phase"
	// ContractImportProblemBeforePhase is a module whose enrollment phase
	// begins only after the first day of the contract, or after today for a
	// contract that runs already.
	ContractImportProblemBeforePhase = "before_phase"
	// ContractImportProblemRepeated is a child another row of the file
	// names for the same school.
	ContractImportProblemRepeated = "repeated"
)

// What the import does with the debtor number of a row; the
// Elternabrechnung decides it (#7007).
const (
	// ContractDebtorNumberNew is taken over.
	ContractDebtorNumberNew = "new"
	// ContractDebtorNumberSame is the number moto already keeps.
	ContractDebtorNumberSame = "same"
	// ContractDebtorNumberDiffers is another number than moto keeps: the
	// import never overwrites it.
	ContractDebtorNumberDiffers = "differs"
	// ContractDebtorNumberRowsDiffer are rows of one payer, or of one
	// child, with different numbers: the import takes over none of them.
	ContractDebtorNumberRowsDiffer = "rows_differ"
	// ContractDebtorNumberInvalid is no number in the format of the Träger.
	ContractDebtorNumberInvalid = "invalid"
	// ContractDebtorNumberNoFamily is a row without a mail address of the
	// contract party: the Elternabrechnung knows a family by its address.
	ContractDebtorNumberNoFamily = "no_family"
)

// Whom a debtor number of the Träger names (#7007).
const (
	// ContractDebtorNumberPerPayer is one number per recipient of the
	// invoices; siblings share it.
	ContractDebtorNumberPerPayer = "payer"
	// ContractDebtorNumberPerChild is one number per child.
	ContractDebtorNumberPerChild = "child"
)

// Limits of an import file.
const (
	MaxContractImportFileBytes = 2 << 20
	MaxContractImportRows      = 5000
	// MaxContractImportFileName bounds the file name the trail keeps.
	MaxContractImportFileName = 255
)

var (
	// ErrContractImportFileInvalid reports a file that is no readable CSV
	// with a header and at least one row.
	ErrContractImportFileInvalid = errors.New("the import file is no readable CSV file with a header row and data rows")
	// ErrContractImportFileTooLarge reports a file over
	// MaxContractImportFileBytes or with more than MaxContractImportRows
	// rows.
	ErrContractImportFileTooLarge = errors.New("the import file is too large")
	// ErrContractImportChanged reports an import whose rows no longer are
	// what the office confirmed in the preview: the contracts, the modules
	// or their conditions changed since. The office loads the preview again.
	ErrContractImportChanged = errors.New("the import differs from the confirmed preview")
)

// ContractImportColumns names per field the header of the column it comes
// from; empty leaves the field unmapped.
type ContractImportColumns struct {
	ChildFirstName string `json:"child_first_name"`
	ChildLastName  string `json:"child_last_name"`
	ChildBirthDate string `json:"child_birth_date"`
	School         string `json:"school"`
	Module         string `json:"module"`
	Weekdays       string `json:"weekdays"`
	EffectiveFrom  string `json:"effective_from"`
	EndsOn         string `json:"ends_on"`
	ConcludedOn    string `json:"concluded_on"`
	PartyFirstName string `json:"party_first_name"`
	PartyLastName  string `json:"party_last_name"`
	PartyEmail     string `json:"party_email"`
	DebtorNumber   string `json:"debtor_number"`
}

// ContractImportMapping is how the rows of a file become contracts. A file
// of one school, or with one module for every child, needs no column for
// it: SchoolID names the school and ModuleName the module of every row.
type ContractImportMapping struct {
	Columns ContractImportColumns `json:"columns"`
	// SchoolID is the school of every row of a file without a school
	// column; 0 with a column.
	SchoolID int64 `json:"school_id"`
	// ModuleName is the module of every row of a file without a module
	// column, by its name at the row's school; empty with a column.
	ModuleName string `json:"module_name"`
}

// ContractImportRequest is one look at a file: the preview when Apply is
// not set, the import of its ready rows when it is. Mapping nil takes the
// mapping moto suggests from the headers. Delimiter is ";", "," or "\t",
// empty detects it.
type ContractImportRequest struct {
	Content   []byte
	FileName  string
	Delimiter string
	Mapping   *ContractImportMapping
	Apply     bool
	// Fingerprint is the fingerprint of the preview the office confirmed;
	// Apply refuses with ErrContractImportChanged when the rows differ from
	// it.
	Fingerprint string
}

// ContractImportProblem is one problem of a row: the field and the code.
type ContractImportProblem struct {
	Field string `json:"field"`
	Code  string `json:"code"`
}

// ContractImportRow is a row of the file as the import reads it.
type ContractImportRow struct {
	Line           int                     `json:"line"`
	Status         string                  `json:"status"`
	ChildFirstName string                  `json:"child_first_name"`
	ChildLastName  string                  `json:"child_last_name"`
	ChildBirthDate *calendar.Date          `json:"child_birth_date"`
	SchoolID       int64                   `json:"school_id,omitempty"`
	SchoolName     string                  `json:"school_name"`
	ModuleNames    []string                `json:"module_names"`
	Weekdays       []string                `json:"weekdays"`
	EffectiveFrom  *calendar.Date          `json:"effective_from"`
	EndsOn         *calendar.Date          `json:"ends_on"`
	PartyName      string                  `json:"party_name"`
	PartyEmail     string                  `json:"party_email"`
	Problems       []ContractImportProblem `json:"problems"`
	// ContractID is the contract the import stored, or the running contract
	// a duplicate row found.
	ContractID int64 `json:"contract_id,omitempty"`
	// RepeatsLine is the line of the row that names the same child before
	// this one (ContractImportProblemRepeated).
	RepeatsLine int `json:"repeats_line,omitempty"`
	// DebtorNumber is the debtor number of the row and DebtorNumberStatus
	// what the import does with it; empty without the column.
	DebtorNumber       string `json:"debtor_number,omitempty"`
	DebtorNumberStatus string `json:"debtor_number_status,omitempty"`
}

// ContractImportCounts counts the rows per status.
type ContractImportCounts struct {
	Ready     int `json:"ready"`
	Duplicate int `json:"duplicate"`
	Conflict  int `json:"conflict"`
	Invalid   int `json:"invalid"`
	Imported  int `json:"imported"`
	// DebtorNumbers counts the payers, or the children when the Träger
	// keeps a number per child (#7007), whose debtor number the import takes
	// over; DebtorConflicts the rows whose number it leaves.
	DebtorNumbers   int `json:"debtor_numbers"`
	DebtorConflicts int `json:"debtor_conflicts"`
}

// ContractImportModule is a module of a school the file can name.
type ContractImportModule struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	PhaseName string `json:"phase_name"`
	// HasConditions tells that the Träger keeps contract conditions for it.
	HasConditions bool `json:"has_conditions"`
}

// ContractImportSchool is a school of the Träger with the modules a
// contract can book there today or later.
type ContractImportSchool struct {
	ID      int64                  `json:"id"`
	Name    string                 `json:"name"`
	Modules []ContractImportModule `json:"modules"`
}

// ContractImport is the answer to a look at a file: how it was read, the
// mapping used and every row.
type ContractImport struct {
	Encoding  string                `json:"encoding"`
	Delimiter string                `json:"delimiter"`
	Headers   []string              `json:"headers"`
	Mapping   ContractImportMapping `json:"mapping"`
	// Unmapped names the required fields without a column or a choice; the
	// rows are read once each has one.
	Unmapped []string `json:"unmapped"`
	// Schools are the schools of the Träger with their modules, for the
	// choice of a file without those columns.
	Schools []ContractImportSchool `json:"schools"`
	// DebtorNumberScope tells whom a debtor number of the Träger names:
	// ContractDebtorNumberPerPayer or ContractDebtorNumberPerChild.
	DebtorNumberScope string `json:"debtor_number_scope"`
	// Fingerprint names the rows and what the import does with each; the
	// import takes over only the preview it was given.
	Fingerprint string               `json:"fingerprint,omitempty"`
	Applied     bool                 `json:"applied"`
	Counts      ContractImportCounts `json:"counts"`
	Rows        []ContractImportRow  `json:"rows"`
}

// ContractImportEntry is one import in the log: when, by whom, which file
// and how many contracts it took over.
type ContractImportEntry struct {
	File       string    `json:"file"`
	AccountID  *int64    `json:"account_id"`
	ImportedAt time.Time `json:"imported_at"`
	Contracts  int       `json:"contracts"`
}

// ContractImports is the import of existing contracts. It reads each school
// in that school's tenant transaction and writes in the Träger transaction,
// one after the other, so it opens its own units and must be called outside
// any transaction.
type ContractImports interface {
	// ImportContracts reads a file with the mapping and answers every row;
	// with Apply it stores the ready rows in one go and records each with
	// the name and the line of the file.
	ImportContracts(ctx context.Context, reviewer Reviewer, request ContractImportRequest) (ContractImport, error)
	// ContractImportLog lists the imports of the Träger, newest first.
	ContractImportLog(ctx context.Context, reviewer Reviewer) ([]ContractImportEntry, error)
}

// ImportedDebtorNumber is the debtor number a row of an import file names
// for a child of a family, with the last day its contract needs it (zero for
// an open contract).
type ImportedDebtorNumber struct {
	Line           int
	FamilyEmail    string
	ChildFirstName string
	ChildLastName  string
	ChildBirthDate *calendar.Date
	DebtorNumber   string
	Until          calendar.Date
	// ContractID is the contract the import stored for the row; 0 in a
	// preview.
	ContractID int64
}

// ImportedDebtorNumberDecision is what the Elternabrechnung does with the
// number of one row: the number as it would store it and one of the
// ContractDebtorNumber statuses.
type ImportedDebtorNumberDecision struct {
	Line         int
	DebtorNumber string
	Status       string
}

// ContractDebtorNumbers is the root-bound port to the debtor numbers the
// Elternabrechnung keeps per payer or per child (#7007). Both operations run
// in the caller's Träger transaction.
type ContractDebtorNumbers interface {
	// DebtorNumberScope tells whom a number of the Träger names:
	// ContractDebtorNumberPerPayer or ContractDebtorNumberPerChild.
	DebtorNumberScope(ctx context.Context, organizationID int64) (string, error)
	// DecideImportedDebtorNumbers answers per number what an import does
	// with it, and with save takes the new ones over for the families of
	// the contracts just stored, recording each in its trail.
	DecideImportedDebtorNumbers(ctx context.Context, reviewer Reviewer, numbers []ImportedDebtorNumber, save bool) ([]ImportedDebtorNumberDecision, error)
}
