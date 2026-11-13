package application

import (
	"strconv"
	"strings"

	"github.com/moto-ogs/project-phoenix/modules/parentbilling"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
	"github.com/moto-ogs/project-phoenix/sharedkernel/sepa"
)

// The check of one row of a mandate import (#6198). A row is invalid when
// a field is missing or no value of a mandate, a duplicate when the Träger
// has the same mandate, a conflict when it contradicts a mandate of the
// Träger or an earlier row, and to clarify when the file does not say
// whether the mandate was collected with before. The import never guesses
// that and never overwrites a mandate.

// importColumnIndex is the column of each mapped field.
type importColumnIndex struct {
	fields              map[string]int
	payerKey            string
	creditorFromTraeger bool
}

// importPayer is a contract of the Träger the payer column can name.
type importPayer struct {
	contractID          int64
	family              string
	firstName, lastName string
	until               calendar.Date
	// child is the child of the contract, for its own debtor number
	// (#7007).
	child parentbilling.FamilyChild
}

type importPayers struct {
	byContract map[int64]importPayer
	byFamily   map[string]importPayer
	// children counts the children under contract of each family, by
	// ChildKey: a family of one child names its child by its address too.
	children map[string]map[string]bool
}

// importCandidate is a checked row with what the answer leaves out: the
// compact IBAN, the family and the creditor identifier.
type importCandidate struct {
	row        parentbilling.SEPAMandateImportRow
	iban       string
	family     string
	creditorID string
	// child is the child the row names when the Träger keeps a number per
	// child (#7007), empty when the row names no single child.
	child parentbilling.FamilyChild
	// perChild marks a row whose number goes to the child, not the family.
	perChild bool
}

// Problems that make a row a conflict or a row to clarify; every other
// problem makes it invalid.
var (
	importConflictCodes = []string{parentbilling.ImportProblemCreditorMismatch, parentbilling.ImportProblemReferenceTaken,
		parentbilling.ImportProblemAccountDiffers,
		parentbilling.ImportProblemReferenceRepeated, parentbilling.ImportProblemPayerHasMandate, parentbilling.ImportProblemPayerRepeated}
	importClarifyCodes = []string{parentbilling.ImportProblemSequenceUnknown, parentbilling.ImportProblemSequenceConflict}
	importKnownCodes   = append(append([]string{}, importConflictCodes...), importClarifyCodes...)
)

// importCheck checks the rows of one file in order.
type importCheck struct {
	columns     importColumnIndex
	creditorID  string
	payers      importPayers
	byReference map[string]parentbilling.SEPAMandate
	inForce     map[string]parentbilling.SEPAMandate
	today       calendar.Date
	sequences   map[int]string
	references  map[string]bool
	families    map[string]bool
	// accounts holds the opened account of each stored mandate a row of
	// the file names by its reference, only in memory (importAccounts).
	accounts map[int64]importAccount
}

// importAccount is the opened IBAN and account holder of a stored mandate.
type importAccount struct {
	iban, holder string
}

func newImportCheck(columns importColumnIndex, creditorID string, payers importPayers, existing []parentbilling.SEPAMandate,
	today calendar.Date, sequences map[int]string,
) *importCheck {
	check := &importCheck{columns: columns, creditorID: creditorID, payers: payers, today: today, sequences: sequences,
		byReference: map[string]parentbilling.SEPAMandate{}, inForce: map[string]parentbilling.SEPAMandate{},
		references: map[string]bool{}, families: map[string]bool{}, accounts: map[int64]importAccount{}}
	for _, mandate := range existing {
		check.byReference[mandate.Reference] = mandate
		if mandate.InForce() {
			check.inForce[mandate.FamilyEmail] = mandate
		}
	}
	return check
}

func (c *importCheck) cell(cells []string, field string) string {
	position, ok := c.columns.fields[field]
	if !ok || position >= len(cells) {
		return ""
	}
	return cells[position]
}

// textCell reads a column the answer repeats and the mandate keeps
// readable. An IBAN there (a column mapped by mistake) is refused and never
// repeated: an IBAN leaves moto only masked and is stored only sealed
// (ADR 0171).
func (c *importCheck) textCell(row *parentbilling.SEPAMandateImportRow, cells []string, field string) (string, bool) {
	value := c.cell(cells, field)
	if containsIBAN(value) {
		addProblem(row, field, parentbilling.ImportProblemInvalid)
		return "", true
	}
	return value, false
}

func addProblem(row *parentbilling.SEPAMandateImportRow, field, problem string) {
	row.Problems = append(row.Problems, parentbilling.SEPAMandateImportProblem{Field: field, Code: problem})
}

// row checks one row of the file.
func (c *importCheck) row(cells []string, line int) importCandidate {
	candidate := importCandidate{row: parentbilling.SEPAMandateImportRow{Line: line,
		Problems: []parentbilling.SEPAMandateImportProblem{}, Hints: []string{}}}
	c.readMandate(&candidate, cells)
	c.readAccount(&candidate, cells)
	c.readPayer(&candidate, cells)
	c.readCreditor(&candidate, cells)
	c.readSequence(&candidate, cells)
	c.classify(&candidate)
	return candidate
}

// readMandate reads the reference and the signing date.
func (c *importCheck) readMandate(candidate *importCandidate, cells []string) {
	row := &candidate.row
	var refused bool
	row.Reference, refused = c.textCell(row, cells, parentbilling.ImportFieldReference)
	switch {
	case refused:
	case row.Reference == "":
		addProblem(row, parentbilling.ImportFieldReference, parentbilling.ImportProblemMissing)
	case !sepa.ValidMandateReference(row.Reference):
		addProblem(row, parentbilling.ImportFieldReference, parentbilling.ImportProblemInvalid)
	}
	row.SignedOn = c.readDate(row, cells, parentbilling.ImportFieldSignedOn, true)
}

// readDate reads a date column that may not lie in the future.
func (c *importCheck) readDate(row *parentbilling.SEPAMandateImportRow, cells []string, field string, required bool) *calendar.Date {
	value := c.cell(cells, field)
	if value == "" {
		if required {
			addProblem(row, field, parentbilling.ImportProblemMissing)
		}
		return nil
	}
	date, ok := parseImportDate(value)
	switch {
	case !ok:
		addProblem(row, field, parentbilling.ImportProblemInvalid)
		return nil
	case date.After(c.today):
		addProblem(row, field, parentbilling.ImportProblemFuture)
		return nil
	}
	return &date
}

// readAccount reads the account holder, the IBAN and the BIC. Only a
// valid IBAN shows, masked; an invalid one is never repeated.
func (c *importCheck) readAccount(candidate *importCandidate, cells []string) {
	row := &candidate.row
	var refused bool
	row.AccountHolder, refused = c.textCell(row, cells, parentbilling.ImportFieldAccountHolder)
	switch length := len([]rune(row.AccountHolder)); {
	case refused:
	case length == 0:
		addProblem(row, parentbilling.ImportFieldAccountHolder, parentbilling.ImportProblemMissing)
	case length > parentbilling.MaxAccountHolderLength:
		addProblem(row, parentbilling.ImportFieldAccountHolder, parentbilling.ImportProblemTooLong)
	}
	iban := sepa.Compact(c.cell(cells, parentbilling.ImportFieldIBAN))
	row.BIC = sepa.Compact(c.cell(cells, parentbilling.ImportFieldBIC))
	switch {
	case iban == "":
		addProblem(row, parentbilling.ImportFieldIBAN, parentbilling.ImportProblemMissing)
	case !sepa.ValidIBAN(iban):
		addProblem(row, parentbilling.ImportFieldIBAN, parentbilling.ImportProblemInvalid)
	default:
		candidate.iban, row.IBANMasked = iban, sepa.MaskIBAN(iban)
		if row.BIC == "" && sepa.NeedsBIC(iban) {
			addProblem(row, parentbilling.ImportFieldBIC, parentbilling.ImportProblemBICRequired)
		}
	}
	if row.BIC != "" && !sepa.ValidBIC(row.BIC) {
		addProblem(row, parentbilling.ImportFieldBIC, parentbilling.ImportProblemInvalid)
		row.BIC = ""
	}
}

// readPayer finds the contract the payer column names: a contract of the
// Träger by its number or the family by its mail address.
func (c *importCheck) readPayer(candidate *importCandidate, cells []string) {
	row := &candidate.row
	value := c.cell(cells, parentbilling.ImportFieldPayer)
	if value == "" {
		addProblem(row, parentbilling.ImportFieldPayer, parentbilling.ImportProblemMissing)
		return
	}
	var payer importPayer
	var found bool
	if c.columns.payerKey == parentbilling.PayerByContract {
		id, err := strconv.ParseInt(strings.TrimPrefix(value, "#"), 10, 64)
		payer, found = c.payers.byContract[id]
		found = found && err == nil
	} else {
		payer, found = c.payers.byFamily[parentbilling.FamilyKey(value)]
	}
	if !found {
		addProblem(row, parentbilling.ImportFieldPayer, parentbilling.ImportProblemPayerUnknown)
		return
	}
	row.ContractID, row.PayerFirstName, row.PayerLastName = payer.contractID, payer.firstName, payer.lastName
	if payer.family == "" {
		addProblem(row, parentbilling.ImportFieldPayer, parentbilling.ImportProblemFamilyUnknown)
	}
	candidate.family = payer.family
	// The contract names its child; the address only a family of one child.
	if c.columns.payerKey == parentbilling.PayerByContract || len(c.payers.children[payer.family]) == 1 {
		candidate.child = payer.child
	}
}

// readCreditor reads the creditor identifier a mandate was given to: the
// column, or the Träger's when the office chose that for the file. It must
// be the Träger's.
func (c *importCheck) readCreditor(candidate *importCandidate, cells []string) {
	value := sepa.Compact(c.cell(cells, parentbilling.ImportFieldCreditorID))
	switch {
	case value == "" && c.columns.creditorFromTraeger:
		candidate.creditorID = c.creditorID
	case value == "":
		addProblem(&candidate.row, parentbilling.ImportFieldCreditorID, parentbilling.ImportProblemMissing)
	case !sepa.ValidCreditorID(value):
		addProblem(&candidate.row, parentbilling.ImportFieldCreditorID, parentbilling.ImportProblemInvalid)
	case value != c.creditorID:
		addProblem(&candidate.row, parentbilling.ImportFieldCreditorID, parentbilling.ImportProblemCreditorMismatch)
	default:
		candidate.creditorID = value
	}
}

// readSequence reads whether the mandate was collected with before: the
// sequence column, a last collection, or the office's answer for the line.
func (c *importCheck) readSequence(candidate *importCandidate, cells []string) {
	row := &candidate.row
	row.LastCollectedOn = c.readDate(row, cells, parentbilling.ImportFieldLastCollected, false)
	if row.LastCollectedOn != nil && row.SignedOn != nil && row.LastCollectedOn.Before(*row.SignedOn) {
		addProblem(row, parentbilling.ImportFieldLastCollected, parentbilling.ImportProblemBeforeSigned)
	}
	sequence, unclear := c.fileSequence(row, cells)
	if answer, ok := c.sequences[row.Line]; ok && unclear != "" &&
		(answer == parentbilling.SequenceFirst || answer == parentbilling.SequenceRecurring) {
		sequence, unclear = answer, ""
		if answer == parentbilling.SequenceFirst {
			row.LastCollectedOn = nil
		}
	}
	if unclear != "" {
		addProblem(row, parentbilling.ImportFieldSequence, unclear)
		return
	}
	row.Sequence = sequence
	if sequence == parentbilling.SequenceRecurring && row.LastCollectedOn == nil {
		row.Hints = append(row.Hints, parentbilling.ImportHintLastCollectionUnknown)
	}
	c.checkLapse(row)
}

// fileSequence is the sequence the file tells, or why it stays unclear.
func (c *importCheck) fileSequence(row *parentbilling.SEPAMandateImportRow, cells []string) (string, string) {
	value := c.cell(cells, parentbilling.ImportFieldSequence)
	sequence, oneOff, known := importSequence(value)
	switch {
	case oneOff:
		addProblem(row, parentbilling.ImportFieldSequence, parentbilling.ImportProblemOneOff)
		return "", ""
	case value != "" && !known:
		return "", parentbilling.ImportProblemSequenceUnknown
	case sequence == parentbilling.SequenceFirst && row.LastCollectedOn != nil:
		return "", parentbilling.ImportProblemSequenceConflict
	case sequence == "" && row.LastCollectedOn != nil:
		// A last collection proves the mandate was used: RCUR.
		return parentbilling.SequenceRecurring, ""
	case sequence == "":
		return "", parentbilling.ImportProblemSequenceUnknown
	}
	return sequence, ""
}

// checkLapse refuses a mandate that would no longer hold today by the rule
// of the debit file (#4697): 36 months after the last collection, after
// the import for a mandate used elsewhere whose last collection the file
// does not tell, or after the signature for one never collected with.
func (c *importCheck) checkLapse(row *parentbilling.SEPAMandateImportRow) {
	if row.SignedOn == nil {
		return
	}
	usedOn := parentbilling.MandateUsedOn(row.Sequence, row.LastCollectedOn, c.today)
	if !parentbilling.MandateExpiresOn(*row.SignedOn, usedOn).After(c.today) {
		addProblem(row, parentbilling.ImportFieldSignedOn, parentbilling.ImportProblemExpired)
	}
}

// classify gives the row its status: invalid before duplicate before
// conflict before clarify.
func (c *importCheck) classify(candidate *importCandidate) {
	row := &candidate.row
	if hasProblemOutside(row.Problems, importKnownCodes) {
		row.Status = parentbilling.ImportRowInvalid
		return
	}
	if existing, ok := c.byReference[row.Reference]; ok && c.sameMandate(existing, candidate) {
		if field := c.accountDiffers(existing, candidate); field != "" {
			// The same mandate with another account is a conflict the office
			// resolves; the import neither skips nor overwrites it.
			addProblem(row, field, parentbilling.ImportProblemAccountDiffers)
			row.Status = parentbilling.ImportRowConflict
			return
		}
		row.Status, row.MandateID, row.Problems = parentbilling.ImportRowDuplicate, existing.ID, []parentbilling.SEPAMandateImportProblem{}
		return
	}
	c.checkConflicts(candidate)
	switch {
	case hasProblemIn(row.Problems, importConflictCodes):
		row.Status = parentbilling.ImportRowConflict
	case hasProblemIn(row.Problems, importClarifyCodes):
		row.Status = parentbilling.ImportRowClarify
	default:
		row.Status = parentbilling.ImportRowReady
	}
}

// accountDiffers names the account field in which a stored mandate differs
// from the row, empty when IBAN and account holder are the same. A stored
// mandate whose account was not opened counts as different: the import
// never calls a mandate a duplicate it could not compare.
func (c *importCheck) accountDiffers(existing parentbilling.SEPAMandate, candidate *importCandidate) string {
	account, ok := c.accounts[existing.ID]
	switch {
	case !ok || account.iban != candidate.iban:
		return parentbilling.ImportFieldIBAN
	case account.holder != candidate.row.AccountHolder:
		return parentbilling.ImportFieldAccountHolder
	}
	return ""
}

// sameMandate tells whether a stored mandate is the row: same reference,
// family, signing date and creditor. A repeated import skips it.
func (c *importCheck) sameMandate(existing parentbilling.SEPAMandate, candidate *importCandidate) bool {
	return candidate.family != "" && existing.FamilyEmail == candidate.family && candidate.row.SignedOn != nil &&
		existing.SignedOn == *candidate.row.SignedOn && existing.CreditorID == candidate.creditorID
}

// checkConflicts compares the row with the mandates of the Träger and the
// rows before it that the import can take over.
func (c *importCheck) checkConflicts(candidate *importCandidate) {
	row := &candidate.row
	if _, taken := c.byReference[row.Reference]; taken {
		addProblem(row, parentbilling.ImportFieldReference, parentbilling.ImportProblemReferenceTaken)
	} else if c.references[row.Reference] {
		addProblem(row, parentbilling.ImportFieldReference, parentbilling.ImportProblemReferenceRepeated)
	}
	if current, ok := c.inForce[candidate.family]; ok && current.Reference != row.Reference {
		addProblem(row, parentbilling.ImportFieldPayer, parentbilling.ImportProblemPayerHasMandate)
	} else if c.families[candidate.family] {
		addProblem(row, parentbilling.ImportFieldPayer, parentbilling.ImportProblemPayerRepeated)
	}
	if !hasProblemIn(row.Problems, importConflictCodes) {
		// Only a row the import can take over claims its reference and
		// family against the rows after it.
		c.references[row.Reference], c.families[candidate.family] = true, true
	}
}

func hasProblemIn(problems []parentbilling.SEPAMandateImportProblem, codes []string) bool {
	for _, problem := range problems {
		if containsString(codes, problem.Code) {
			return true
		}
	}
	return false
}

func hasProblemOutside(problems []parentbilling.SEPAMandateImportProblem, codes []string) bool {
	for _, problem := range problems {
		if !containsString(codes, problem.Code) {
			return true
		}
	}
	return false
}
