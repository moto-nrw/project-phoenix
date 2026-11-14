package application

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"unicode"

	validation "github.com/go-ozzo/ozzo-validation"

	"github.com/moto-ogs/project-phoenix/modules/parentbilling"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
	"github.com/moto-ogs/project-phoenix/sharedkernel/sepa"
)

// The import of existing SEPA mandates (#6198). A look at a file reads it,
// maps its columns, checks every row against the Träger's creditor, its
// contracts and its mandates and answers what the import would do; with
// Apply it stores the ready rows through the same validation, sealing and
// store as a mandate the office enters, in the caller's Träger transaction.
// The file is never stored: the trail of each imported mandate keeps the
// name of the file and the line.

// MandateImportStore records and lists the imports. Every method runs in
// the Träger transaction of organizationID.
type MandateImportStore interface {
	// InsertSEPAMandateImportEvents records the creation of each imported
	// mandate with the name of the file and its line, in one statement.
	InsertSEPAMandateImportEvents(ctx context.Context, organizationID, accountID int64, file string,
		imported []parentbilling.ImportedSEPAMandate) error
	// SEPAMandateImportEvents reads the creation entry of each imported
	// mandate, newest first, as an entry of one mandate.
	SEPAMandateImportEvents(ctx context.Context, organizationID int64) ([]parentbilling.SEPAMandateImportEntry, error)
	// FamilyDebtorNumbers reads the debtor number of each family that has
	// one (#4693).
	FamilyDebtorNumbers(ctx context.Context, organizationID int64) (map[string]string, error)
	// SetFamilyDebtorNumbers gives each family its debtor number where it
	// has none and records the change in the family's trail.
	SetFamilyDebtorNumbers(ctx context.Context, organizationID, accountID int64, numbers []parentbilling.FamilyDebtorNumber) error
	// ChildNumberStore keeps the numbers of the children (#7007).
	ChildNumberStore
	// FamilyBillings reads the families of the Träger with their numbers,
	// InsertFamilyBillingEvents records a read in their trail (#7009).
	FamilyBillings(ctx context.Context, organizationID int64) ([]parentbilling.FamilyBilling, error)
	InsertFamilyBillingEvents(ctx context.Context, organizationID, accountID int64, action string, families []parentbilling.FamilyBilling) error
}

// importRequiredFields are the fields every import needs a column for.
var importRequiredFields = []string{parentbilling.ImportFieldReference, parentbilling.ImportFieldSignedOn,
	parentbilling.ImportFieldIBAN, parentbilling.ImportFieldAccountHolder, parentbilling.ImportFieldPayer}

// ImportSEPAMandates reads the file, answers every row and, with Apply,
// stores the ready rows and the clarified ones.
func (m *SEPAMandates) ImportSEPAMandates(ctx context.Context, reviewer parentbilling.HolidayReviewer,
	request parentbilling.SEPAMandateImportRequest,
) (parentbilling.SEPAMandateImport, error) {
	if err := validReviewer(reviewer); err != nil {
		return parentbilling.SEPAMandateImport{}, err
	}
	table, err := readImportTable(request.Content, request.Delimiter)
	if err != nil {
		return parentbilling.SEPAMandateImport{}, err
	}
	result := parentbilling.SEPAMandateImport{Encoding: table.encoding, Delimiter: table.delimiter, Headers: table.headers,
		Rows: []parentbilling.SEPAMandateImportRow{}, Unmapped: []string{}}
	result.Mapping = suggestImportMapping(table.headers)
	if request.Mapping != nil {
		result.Mapping = *request.Mapping
	}
	creditor, err := creditorOf(ctx, m.store, reviewer.OrganizationID)
	if err != nil {
		return parentbilling.SEPAMandateImport{}, err
	}
	if creditor.CreditorID == "" {
		return parentbilling.SEPAMandateImport{}, parentbilling.ErrSEPACreditorMissing
	}
	result.CreditorID = creditor.CreditorID
	bindCreditorChoice(&result.Mapping, creditor.CreditorID)
	columns, unmapped, err := importColumns(table.headers, result.Mapping)
	if err != nil {
		return parentbilling.SEPAMandateImport{}, err
	}
	result.Unmapped = unmapped
	if len(unmapped) > 0 {
		if request.Apply {
			return parentbilling.SEPAMandateImport{}, unmappedError(unmapped)
		}
		return result, nil
	}
	candidates, err := m.evaluateImport(ctx, reviewer, table, columns, creditor.CreditorID, request.Sequences)
	if err != nil {
		return parentbilling.SEPAMandateImport{}, err
	}
	// A row without a mandate still gives its debtor number (#7007): the
	// confirmation covers it and the import takes it over.
	result.Fingerprint = importFingerprint(result.Mapping, creditor.CreditorID, candidates)
	if request.Apply {
		if err := m.applyConfirmed(ctx, reviewer, request, result.Fingerprint, candidates); err != nil {
			return parentbilling.SEPAMandateImport{}, err
		}
		result.Applied = true
	}
	shown, withoutMandate := importPreviewRows(candidates)
	for _, candidate := range shown {
		result.Rows = append(result.Rows, candidate.row)
		countImportRow(&result.Counts, candidate.row.Status)
	}
	countDebtorNumbers(&result.Counts, candidates)
	result.Counts.WithoutMandate = withoutMandate
	return result, nil
}

// importPreviewRows omits mandate-free rows unless their debtor number
// needs attention. A copy marks these as conflicts only in the answer;
// their empty mandate status still prevents validation or mandate writes.
func importPreviewRows(candidates []importCandidate) ([]importCandidate, int) {
	shown := make([]importCandidate, 0, len(candidates))
	withoutMandate := 0
	for _, candidate := range candidates {
		if candidate.row.Status == "" {
			withoutMandate++
			if !debtorNumberConflict(candidate.row.DebtorNumberStatus) {
				continue
			}
			candidate.row.Status = parentbilling.ImportRowConflict
		}
		shown = append(shown, candidate)
	}
	return shown, withoutMandate
}

// bindCreditorChoice keeps the office's choice that the file holds the
// Träger's mandates only while the Träger has the creditor identifier the
// office confirmed. After a change of the identifier the choice is void and
// the office confirms the new one: a mandate binds the creditor it was given
// to (ADR 0171).
func bindCreditorChoice(mapping *parentbilling.SEPAMandateImportMapping, creditorID string) {
	if mapping.CreditorFromTraeger && sepa.Compact(mapping.ConfirmedCreditorID) == creditorID {
		mapping.ConfirmedCreditorID = creditorID
		return
	}
	mapping.CreditorFromTraeger, mapping.ConfirmedCreditorID = false, ""
}

// importColumns finds the column of each mapped field. A header the file
// does not have or an unknown payer key refuses the mapping; unmapped
// names the required fields without a column.
func importColumns(headers []string, mapping parentbilling.SEPAMandateImportMapping) (importColumnIndex, []string, error) {
	errs := validation.Errors{}
	switch mapping.PayerKey {
	case parentbilling.PayerByEmail, parentbilling.PayerByContract, parentbilling.PayerByDebtorNumber:
	default:
		errs["payer_key"] = errors.New("must be email, contract_id or debtor_number")
	}
	index := importColumnIndex{creditorFromTraeger: mapping.CreditorFromTraeger, payerKey: mapping.PayerKey, fields: map[string]int{}}
	for field, header := range mappingHeaders(mapping.Columns) {
		if header == "" {
			continue
		}
		position := slices.Index(headers, header)
		if position < 0 {
			errs[field] = errors.New("names no column of the file")
			continue
		}
		index.fields[field] = position
	}
	if len(errs) > 0 {
		return importColumnIndex{}, nil, errs
	}
	unmapped := []string{}
	for _, field := range importRequiredFields {
		if _, ok := index.fields[field]; !ok {
			unmapped = append(unmapped, field)
		}
	}
	if _, ok := index.fields[parentbilling.ImportFieldCreditorID]; !ok && !mapping.CreditorFromTraeger {
		// A mandate binds the creditor it was given to: without the column
		// the office has to say that the file holds the Träger's mandates.
		unmapped = append(unmapped, parentbilling.ImportFieldCreditorID)
	}
	return index, unmapped, nil
}

func mappingHeaders(columns parentbilling.SEPAMandateImportColumns) map[string]string {
	return map[string]string{
		parentbilling.ImportFieldReference: columns.Reference, parentbilling.ImportFieldSignedOn: columns.SignedOn,
		parentbilling.ImportFieldIBAN: columns.IBAN, parentbilling.ImportFieldBIC: columns.BIC,
		parentbilling.ImportFieldAccountHolder: columns.AccountHolder, parentbilling.ImportFieldPayer: columns.Payer,
		parentbilling.ImportFieldCreditorID: columns.CreditorID, parentbilling.ImportFieldSequence: columns.Sequence,
		parentbilling.ImportFieldLastCollected: columns.LastCollected, parentbilling.ImportFieldDebtorNumber: columns.DebtorNumber,
	}
}

func unmappedError(unmapped []string) error {
	errs := validation.Errors{}
	for _, field := range unmapped {
		errs[field] = errors.New("needs a column")
	}
	return errs
}

func countImportRow(counts *parentbilling.SEPAMandateImportCounts, status string) {
	switch status {
	case parentbilling.ImportRowReady:
		counts.Ready++
	case parentbilling.ImportRowClarify:
		counts.Clarify++
	case parentbilling.ImportRowDuplicate:
		counts.Duplicate++
	case parentbilling.ImportRowSameMandate:
		counts.SameMandate++
	case parentbilling.ImportRowConflict:
		counts.Conflict++
	case parentbilling.ImportRowInvalid:
		counts.Invalid++
	case parentbilling.ImportRowImported:
		counts.Imported++
	}
}

// evaluateImport reads the payers and mandates of the Träger once, opens
// the accounts of the stored mandates the file names again, and checks
// every row against them and against the rows before it.
func (m *SEPAMandates) evaluateImport(ctx context.Context, reviewer parentbilling.HolidayReviewer, table importTable,
	columns importColumnIndex, creditorID string, sequences map[int]string,
) ([]importCandidate, error) {
	payers, err := m.importPayers(ctx, reviewer.OrganizationID)
	if err != nil {
		return nil, err
	}
	if columns.payerKey == parentbilling.PayerByDebtorNumber {
		if err := m.indexPayerNumbers(ctx, reviewer.OrganizationID, &payers); err != nil {
			return nil, err
		}
	}
	existing, err := m.store.SEPAMandates(ctx, reviewer.OrganizationID, "")
	if err != nil {
		return nil, err
	}
	accounts, err := m.importAccounts(ctx, reviewer, table, columns, existing)
	if err != nil {
		return nil, err
	}
	check := newImportCheck(columns, creditorID, payers, existing, m.today(), sequences)
	check.accounts = accounts
	candidates := make([]importCandidate, len(table.rows))
	for i, cells := range table.rows {
		candidates[i] = check.candidate(cells, table.lines[i])
	}
	if _, mapped := columns.fields[parentbilling.ImportFieldDebtorNumber]; mapped {
		if err := m.assignImportedNumbers(ctx, reviewer, candidates, table, columns); err != nil {
			return nil, err
		}
	}
	if err := m.recordPayerNumberReads(ctx, reviewer, payers, candidates); err != nil {
		return nil, err
	}
	return candidates, nil
}

// Contracts the payer column can name: in force from a year ago up to
// four years ahead, read a year at a time (VersionsInForce answers at most
// 400 days).
const (
	importContractYearsBack  = 1
	importContractYearsAhead = 4
)

// contractVersionsAround reads the versions of the contracts of the Träger
// in force from a year before today up to four years ahead, a year at a
// time: the contracts a file or the list of families can name.
func contractVersionsAround(ctx context.Context, contracts parentbilling.CareContracts, organizationID int64, today calendar.Date) (
	[]parentbilling.CareContractVersion, error,
) {
	var versions []parentbilling.CareContractVersion
	for year := -importContractYearsBack; year < importContractYearsAhead; year++ {
		from := calendar.DateFromTime(today.BerlinMidnight().AddDate(year, 0, 0))
		to := calendar.DateFromTime(today.BerlinMidnight().AddDate(year+1, 0, -1))
		chunk, err := contracts.VersionsInForce(ctx, organizationID, from, to)
		if err != nil {
			return nil, fmt.Errorf("parent billing: read the contracts: %w", err)
		}
		versions = append(versions, chunk...)
	}
	return versions, nil
}

// countChild counts the child of the payer's contract under its family and
// keeps the latest end of its contracts.
func (p importPayers) countChild(payer importPayer) {
	key := parentbilling.ChildKey(payer.child.FirstName, payer.child.LastName, payer.child.BirthDate)
	if payer.family == "" || key == "" {
		return
	}
	if p.children[payer.family] == nil {
		p.children[payer.family] = map[string]bool{}
	}
	p.children[payer.family][key] = true
	child := parentbilling.ChildDebtorNumberKey(payer.family, key)
	if payer.until.After(p.childUntil[child]) {
		p.childUntil[child] = payer.until
	}
	if current, ok := p.byChild[child]; !ok || payer.until.After(current.until) {
		p.byChild[child] = payer
	}
}

// importPayers indexes the contracts of the Träger in force in the import
// window by number and by family; a family with several contracts takes
// the one in force longest, then the oldest.
func (m *SEPAMandates) importPayers(ctx context.Context, organizationID int64) (importPayers, error) {
	versions, err := contractVersionsAround(ctx, m.contracts, organizationID, m.today())
	if err != nil {
		return importPayers{}, err
	}
	until := map[int64]calendar.Date{}
	children := map[int64]parentbilling.FamilyChild{}
	ids := []int64{}
	for _, version := range versions {
		if _, seen := until[version.ContractID]; !seen {
			ids = append(ids, version.ContractID)
			children[version.ContractID] = parentbilling.FamilyChild{FirstName: version.ChildFirstName,
				LastName: version.ChildLastName, BirthDate: version.ChildBirthDate}
		}
		if version.Until.After(until[version.ContractID]) || until[version.ContractID].IsZero() {
			until[version.ContractID] = version.Until
		}
	}
	slices.Sort(ids)
	recipients, err := m.contracts.Recipients(ctx, organizationID, ids)
	if err != nil {
		return importPayers{}, fmt.Errorf("parent billing: read the contract parties for the import: %w", err)
	}
	payers := importPayers{byContract: map[int64]importPayer{}, byFamily: map[string]importPayer{}, children: map[string]map[string]bool{},
		childUntil: map[string]calendar.Date{}, byChild: map[string]importPayer{}}
	for _, id := range ids {
		recipient := recipients[id]
		payer := importPayer{contractID: id, family: parentbilling.FamilyKey(recipient.Email),
			firstName: recipient.FirstName, lastName: recipient.LastName, until: until[id], child: children[id]}
		payers.byContract[id] = payer
		payers.countChild(payer)
		if current, ok := payers.byFamily[payer.family]; payer.family != "" && (!ok || payer.until.After(current.until)) {
			payers.byFamily[payer.family] = payer
		}
	}
	return payers, nil
}

// applyConfirmed takes over exactly the preview the office confirmed: a
// revoked mandate, a new contract or another answer since then changes the
// rows, and the office looks again (ErrImportChanged).
func (m *SEPAMandates) applyConfirmed(ctx context.Context, reviewer parentbilling.HolidayReviewer,
	request parentbilling.SEPAMandateImportRequest, fingerprint string, candidates []importCandidate,
) error {
	if fingerprint == "" || request.Fingerprint != fingerprint {
		return parentbilling.ErrImportChanged
	}
	if err := m.store.SetFamilyDebtorNumbers(ctx, reviewer.OrganizationID, reviewer.AccountID, debtorNumbersToSet(candidates)); err != nil {
		return fmt.Errorf("parent billing: take over the debtor numbers: %w", err)
	}
	if err := m.saveChildNumbers(ctx, reviewer, candidates); err != nil {
		return err
	}
	return m.applyImport(ctx, reviewer, importFileName(request.FileName), candidates)
}

// applyImport stores the ready rows in line order through the owner's
// validation and sealing and records them with the name of the file.
func (m *SEPAMandates) applyImport(ctx context.Context, reviewer parentbilling.HolidayReviewer, file string, candidates []importCandidate) error {
	imported := []parentbilling.ImportedSEPAMandate{}
	for i := range candidates {
		if candidates[i].row.Status != parentbilling.ImportRowReady {
			continue
		}
		stored, err := m.storeImported(ctx, reviewer, candidates[i])
		if err != nil {
			return err
		}
		candidates[i].row.Status, candidates[i].row.MandateID = parentbilling.ImportRowImported, stored.ID
		imported = append(imported, parentbilling.ImportedSEPAMandate{Mandate: stored, Line: candidates[i].row.Line})
	}
	if len(imported) == 0 {
		return nil
	}
	if err := m.store.InsertSEPAMandateImportEvents(ctx, reviewer.OrganizationID, reviewer.AccountID, file, imported); err != nil {
		return fmt.Errorf("parent billing: record the imported SEPA mandates: %w", err)
	}
	return nil
}

// storeImported validates one row like a mandate the office enters, seals
// its account and stores it with its last collection.
func (m *SEPAMandates) storeImported(ctx context.Context, reviewer parentbilling.HolidayReviewer, candidate importCandidate) (parentbilling.SEPAMandate, error) {
	row := candidate.row
	mandate, iban, err := m.normalizeMandate(parentbilling.SEPAMandateInput{ContractID: row.ContractID, Reference: row.Reference,
		SignedOn: *row.SignedOn, Kind: parentbilling.MandateRecurring, Sequence: row.Sequence, AccountHolder: row.AccountHolder,
		IBAN: candidate.iban, BIC: row.BIC, Source: parentbilling.MandateFromImport})
	if err != nil {
		// The check of the row answers the same rules; reaching this is a
		// defect, and the import stores nothing.
		return parentbilling.SEPAMandate{}, fmt.Errorf("parent billing: line %d passed the import check but not the mandate: %w", row.Line, err)
	}
	mandate.FamilyEmail, mandate.ContractID, mandate.CreditorID, mandate.LastCollectedOn = candidate.family, row.ContractID,
		candidate.creditorID, row.LastCollectedOn
	sealed, err := m.cipher.Seal([]byte(iban), ibanContext(reviewer.OrganizationID, mandate.Reference))
	if err != nil {
		return parentbilling.SEPAMandate{}, fmt.Errorf("parent billing: seal the IBAN: %w", err)
	}
	holder, err := m.cipher.Seal([]byte(mandate.AccountHolder), holderContext(reviewer.OrganizationID, mandate.Reference))
	if err != nil {
		return parentbilling.SEPAMandate{}, fmt.Errorf("parent billing: seal the account holder: %w", err)
	}
	return m.store.InsertSEPAMandate(ctx, reviewer.OrganizationID, reviewer.AccountID,
		parentbilling.SealedSEPAMandate{Mandate: mandate, IBAN: sealed, Holder: holder})
}

// SEPAMandateImportLog lists the imports of the Träger: the mandates one
// account took over from one file in one transaction are one import.
func (m *SEPAMandates) SEPAMandateImportLog(ctx context.Context, reviewer parentbilling.HolidayReviewer) ([]parentbilling.SEPAMandateImportEntry, error) {
	if err := validReviewer(reviewer); err != nil {
		return nil, err
	}
	events, err := m.store.SEPAMandateImportEvents(ctx, reviewer.OrganizationID)
	if err != nil {
		return nil, err
	}
	entries := []parentbilling.SEPAMandateImportEntry{}
	for _, event := range events {
		if last := len(entries) - 1; last >= 0 && sameImport(entries[last], event) {
			entries[last].Mandates++
			continue
		}
		entries = append(entries, event)
	}
	return entries, nil
}

func sameImport(a, b parentbilling.SEPAMandateImportEntry) bool {
	sameAccount := (a.AccountID == nil && b.AccountID == nil) || (a.AccountID != nil && b.AccountID != nil && *a.AccountID == *b.AccountID)
	return a.File == b.File && a.ImportedAt.Equal(b.ImportedAt) && sameAccount
}

// importFileName is the name the trail keeps: the base name without
// control characters, at most MaxImportFileNameLength characters, "import.csv"
// when nothing remains.
func importFileName(name string) string {
	if i := strings.LastIndexAny(name, `/\`); i >= 0 {
		name = name[i+1:]
	}
	name = strings.TrimSpace(strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, name))
	if runes := []rune(name); len(runes) > parentbilling.MaxImportFileNameLength {
		name = string(runes[:parentbilling.MaxImportFileNameLength])
	}
	if name == "" || containsIBAN(name) {
		// A name that carries an IBAN would leave it readable in the trail.
		return "import.csv"
	}
	return name
}
