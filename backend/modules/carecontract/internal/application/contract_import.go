package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/moto-ogs/project-phoenix/modules/carecontract"
	"github.com/moto-ogs/project-phoenix/modules/carecontract/internal/ports"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
	"github.com/moto-ogs/project-phoenix/sharedkernel/textnorm"
)

// The import of existing contracts (#4723, ADR 0232). A look at a file reads
// it, maps its columns, checks every row against the schools, the modules,
// the conditions and the running contracts of the Träger and answers what
// the import would do; with Apply it stores the ready rows in one Träger
// transaction. It reads each school's offerings in that school's tenant
// transaction first (no personal data), then works in the Träger
// transaction, then records in the access log of every school whose
// contracts it showed or stored. The file is never stored: the trail of
// each imported contract keeps the name of the file and the line.

const accessViewImport = "care_contract_import"

// ImportStore persists what the import reads and writes, in the Träger
// transaction of organizationID.
type ImportStore interface {
	Templates(ctx context.Context, organizationID int64) ([]TemplateRow, error)
	ContractChildren(ctx context.Context, organizationID int64, schoolIDs []int64) ([]ports.ContractChild, error)
	InsertImportedContracts(ctx context.Context, organizationID, accountID int64, contracts []ports.ImportedContract) (map[int]int64, error)
	ContractImportEvents(ctx context.Context, organizationID int64) ([]carecontract.ContractImportEntry, error)
	InsertEvents(ctx context.Context, organizationID int64, events []Event) error
}

// ImportPorts are the root-bound ports of the import.
type ImportPorts struct {
	Schools      carecontract.TraegerSchools
	Applications carecontract.SchoolApplications
	Bookings     carecontract.SchoolBookings
	Access       carecontract.SchoolAccessLog
	Runtime      carecontract.Runtime
	// Numbers keeps the debtor numbers of the rows (#7007).
	Numbers carecontract.ContractDebtorNumbers
	// Fingerprint names content by its checksum, for the confirmation of a
	// preview.
	Fingerprint func([]byte) string
}

// ContractImports serves carecontract.ContractImports.
type ContractImports struct {
	store ImportStore
	ports ImportPorts
	today func() calendar.Date
}

var _ carecontract.ContractImports = (*ContractImports)(nil)

func NewContractImports(store ImportStore, ports ImportPorts, today func() calendar.Date) *ContractImports {
	if store == nil || ports.Schools == nil || ports.Applications == nil || ports.Bookings == nil || ports.Access == nil ||
		ports.Runtime == nil || ports.Numbers == nil || ports.Fingerprint == nil || today == nil {
		panic("care contract: the store, the schools, the applications, the bookings, the access log, the runtime, the debtor numbers, the fingerprint and the clock are required")
	}
	return &ContractImports{store: store, ports: ports, today: today}
}

// readContractImportTable decodes and splits the file. It refuses binary
// content, a file without a header and a data row, repeated headers and
// more than MaxContractImportRows rows.
func readContractImportTable(content []byte, delimiter string) (textnorm.CSVTable, error) {
	if len(content) > carecontract.MaxContractImportFileBytes {
		return textnorm.CSVTable{}, carecontract.ErrContractImportFileTooLarge
	}
	table, err := textnorm.ReadCSVTable(content, delimiter, carecontract.MaxContractImportRows)
	if errors.Is(err, textnorm.ErrCSVTooManyRows) {
		return textnorm.CSVTable{}, carecontract.ErrContractImportFileTooLarge
	}
	if err != nil || len(table.Headers) < 2 || len(table.Rows) == 0 || textnorm.RepeatedCSVHeader(table.Headers) {
		return textnorm.CSVTable{}, carecontract.ErrContractImportFileInvalid
	}
	return table, nil
}

// ImportContracts reads the file, answers every row and, with Apply, stores
// the ready rows.
func (i *ContractImports) ImportContracts(ctx context.Context, reviewer carecontract.Reviewer, request carecontract.ContractImportRequest) (carecontract.ContractImport, error) {
	if err := checkReviewer(reviewer); err != nil {
		return carecontract.ContractImport{}, err
	}
	table, err := readContractImportTable(request.Content, request.Delimiter)
	if err != nil {
		return carecontract.ContractImport{}, err
	}
	schools, err := i.importSchools(ctx, reviewer.OrganizationID)
	if err != nil {
		return carecontract.ContractImport{}, err
	}
	result := carecontract.ContractImport{Encoding: table.Encoding, Delimiter: table.Delimiter, Headers: table.Headers,
		Rows: []carecontract.ContractImportRow{}, Unmapped: []string{}}
	result.Mapping = suggestContractImportMapping(table.Headers)
	if request.Mapping != nil {
		result.Mapping = *request.Mapping
	}
	result.Mapping = settleContractImportMapping(result.Mapping, schools)
	index, unmapped, err := resolveContractImportMapping(table.Headers, result.Mapping, schools)
	if err != nil {
		return carecontract.ContractImport{}, err
	}
	result.Unmapped = unmapped
	if len(unmapped) > 0 && request.Apply {
		fields := map[string]string{}
		for _, field := range unmapped {
			fields[field] = "required"
		}
		return carecontract.ContractImport{}, &carecontract.ValidationError{Fields: fields}
	}
	var shown map[int64][]int64
	err = i.ports.Runtime.WithinTraeger(ctx, reviewer.OrganizationID, func(txCtx context.Context) error {
		var err error
		shown, err = i.evaluate(txCtx, reviewer, request, table, index, schools, &result)
		return err
	})
	if err != nil {
		return carecontract.ContractImport{}, err
	}
	if err := i.logSchools(ctx, reviewer, shown); err != nil {
		return carecontract.ContractImport{}, err
	}
	return result, nil
}

// evaluate fills the answer in the Träger transaction: the schools with
// their modules, the rows and, with Apply, the stored contracts. It answers
// per school the contracts whose child the answer names.
func (i *ContractImports) evaluate(ctx context.Context, reviewer carecontract.Reviewer, request carecontract.ContractImportRequest,
	table textnorm.CSVTable, index contractImportIndex, schools []importSchool, result *carecontract.ContractImport) (map[int64][]int64, error) {
	organizationID := reviewer.OrganizationID
	rows, err := i.store.Templates(ctx, organizationID)
	if err != nil {
		return nil, fmt.Errorf("care contract: read the conditions: %w", err)
	}
	templates, _ := indexTemplates(rows)
	result.Schools = importSchoolChoices(schools, templates)
	if result.DebtorNumberScope, err = i.ports.Numbers.DebtorNumberScope(ctx, organizationID); err != nil {
		return nil, fmt.Errorf("care contract: read whom the debtor numbers name: %w", err)
	}
	if len(result.Unmapped) > 0 {
		return nil, nil
	}
	ids := make([]int64, len(schools))
	for n, school := range schools {
		ids[n] = school.school.ID
	}
	running, err := i.store.ContractChildren(ctx, organizationID, ids)
	if err != nil {
		return nil, err
	}
	check := newContractImportCheck(index, schools, templates, running, i.today())
	candidates := make([]importCandidate, len(table.Rows))
	for n, cells := range table.Rows {
		candidates[n] = check.candidate(cells, table.Lines[n])
	}
	if err := i.decideNumbers(ctx, reviewer, index, candidates, false); err != nil {
		return nil, err
	}
	result.Fingerprint = i.ports.Fingerprint(contractImportState(result.Mapping, result.DebtorNumberScope, candidates))
	if request.Apply {
		if request.Fingerprint != result.Fingerprint {
			return nil, carecontract.ErrContractImportChanged
		}
		if err := i.apply(ctx, reviewer, importFileName(request.FileName), index, candidates); err != nil {
			return nil, err
		}
		result.Applied = true
	}
	shown := map[int64][]int64{}
	for _, candidate := range candidates {
		row := candidate.row
		result.Rows = append(result.Rows, row)
		countContractImportRow(&result.Counts, row)
		if row.ContractID > 0 {
			shown[row.SchoolID] = append(shown[row.SchoolID], row.ContractID)
		}
	}
	return shown, trail(ctx, i.store, organizationID, reviewer.AccountID, 0, 0, ActionRead,
		map[string]any{"view": "import", "rows": len(candidates), "applied": result.Applied})
}

// apply stores the ready rows, records each with the file and its line and
// takes their debtor numbers over.
func (i *ContractImports) apply(ctx context.Context, reviewer carecontract.Reviewer, file string, index contractImportIndex, candidates []importCandidate) error {
	var contracts []ports.ImportedContract
	for _, candidate := range candidates {
		if candidate.row.Status == carecontract.ContractImportReady {
			contracts = append(contracts, candidate.contract)
		}
	}
	stored, err := i.store.InsertImportedContracts(ctx, reviewer.OrganizationID, reviewer.AccountID, contracts)
	if err != nil {
		return err
	}
	events := make([]Event, 0, len(contracts))
	for n := range candidates {
		candidate := &candidates[n]
		id, ok := stored[candidate.row.Line]
		if !ok {
			continue
		}
		candidate.row.Status, candidate.row.ContractID = carecontract.ContractImportImported, id
		contractID, schoolID := id, candidate.contract.SchoolID
		events = append(events, Event{ContractID: &contractID, SchoolID: &schoolID, AccountID: reviewer.AccountID, Action: ActionCreate,
			Details: map[string]any{ports.ImportActionDetail: file, "import_line": candidate.row.Line,
				"method": carecontract.MethodPaper, "care_offering_ids": versionOfferings(candidate.contract.Version)}})
	}
	if err := i.store.InsertEvents(ctx, reviewer.OrganizationID, events); err != nil {
		return fmt.Errorf("care contract: record the import: %w", err)
	}
	return i.decideNumbers(ctx, reviewer, index, candidates, true)
}

func versionOfferings(version carecontract.Version) []int64 {
	ids := []int64{}
	for _, service := range version.Services {
		if !slices.Contains(ids, service.CareOfferingID) {
			ids = append(ids, service.CareOfferingID)
		}
	}
	return ids
}

// decideNumbers asks the Elternabrechnung what it does with the debtor
// number of each row that takes a contract over, and with save lets it
// store them for the contracts just stored.
func (i *ContractImports) decideNumbers(ctx context.Context, reviewer carecontract.Reviewer, index contractImportIndex,
	candidates []importCandidate, save bool) error {
	if !index.has(carecontract.ContractImportFieldDebtorNumber) {
		return nil
	}
	wanted := carecontract.ContractImportReady
	if save {
		wanted = carecontract.ContractImportImported
	}
	var numbers []carecontract.ImportedDebtorNumber
	at := map[int]int{}
	for n, candidate := range candidates {
		row := candidate.row
		if row.Status != wanted || row.DebtorNumber == "" {
			continue
		}
		at[row.Line] = n
		numbers = append(numbers, carecontract.ImportedDebtorNumber{Line: row.Line, FamilyEmail: row.PartyEmail,
			ChildFirstName: row.ChildFirstName, ChildLastName: row.ChildLastName, ChildBirthDate: row.ChildBirthDate,
			DebtorNumber: row.DebtorNumber, Until: candidate.until, ContractID: row.ContractID})
	}
	if len(numbers) == 0 {
		return nil
	}
	decisions, err := i.ports.Numbers.DecideImportedDebtorNumbers(ctx, reviewer, numbers, save)
	if err != nil {
		return fmt.Errorf("care contract: debtor numbers of the import: %w", err)
	}
	for _, decision := range decisions {
		if n, ok := at[decision.Line]; ok {
			candidates[n].row.DebtorNumber, candidates[n].row.DebtorNumberStatus = decision.DebtorNumber, decision.Status
		}
	}
	return nil
}

func countContractImportRow(counts *carecontract.ContractImportCounts, row carecontract.ContractImportRow) {
	switch row.Status {
	case carecontract.ContractImportReady:
		counts.Ready++
	case carecontract.ContractImportDuplicate:
		counts.Duplicate++
	case carecontract.ContractImportConflict:
		counts.Conflict++
	case carecontract.ContractImportInvalid:
		counts.Invalid++
	case carecontract.ContractImportImported:
		counts.Imported++
	}
	switch row.DebtorNumberStatus {
	case "":
	case carecontract.ContractDebtorNumberNew:
		counts.DebtorNumbers++
	case carecontract.ContractDebtorNumberSame:
	default:
		counts.DebtorConflicts++
	}
}

// contractImportState names the mapping, whom a debtor number goes to and
// every row with what the import does with it: an import takes over only
// the preview it was given, by the fingerprint of this state.
func contractImportState(mapping carecontract.ContractImportMapping, scope string, candidates []importCandidate) []byte {
	type stateRow struct {
		Row      carecontract.ContractImportRow `json:"row"`
		Contract ports.ImportedContract         `json:"contract"`
	}
	rows := make([]stateRow, len(candidates))
	for n, candidate := range candidates {
		rows[n] = stateRow{Row: candidate.row, Contract: candidate.contract}
	}
	// The values are plain data: encoding them cannot fail.
	encoded, _ := json.Marshal(struct {
		Mapping carecontract.ContractImportMapping `json:"mapping"`
		Scope   string                             `json:"scope"`
		Rows    []stateRow                         `json:"rows"`
	}{mapping, scope, rows})
	return encoded
}

// importFileName is the name the trail keeps: without a path, at most
// MaxContractImportFileName characters.
func importFileName(name string) string {
	name = strings.TrimSpace(filepath.Base(strings.ReplaceAll(name, "\\", "/")))
	if name == "" || name == "." || name == "/" {
		return "import.csv"
	}
	if utf8.RuneCountInString(name) > carecontract.MaxContractImportFileName {
		name = string([]rune(name)[:carecontract.MaxContractImportFileName])
	}
	return name
}

// importSchools reads the offerings of every school of the Träger in that
// school's tenant transaction: the modules a file can name. They are master
// data of the school, no personal data.
func (i *ContractImports) importSchools(ctx context.Context, organizationID int64) ([]importSchool, error) {
	schools, err := i.ports.Schools.TraegerSchools(ctx, organizationID)
	if err != nil {
		return nil, fmt.Errorf("care contract: schools: %w", err)
	}
	today := i.today()
	result := make([]importSchool, len(schools))
	for n, school := range schools {
		result[n].school = school
		err := i.ports.Runtime.WithinSchool(ctx, school.ID, func(txCtx context.Context) error {
			offerings, err := i.ports.Bookings.Offerings(txCtx, nil)
			if err != nil || len(offerings) == 0 {
				return err
			}
			result[n].offerings, err = i.ports.Applications.NamePhases(txCtx, offerings)
			return err
		})
		if err != nil {
			return nil, fmt.Errorf("care contract: read offerings of school %d: %w", school.ID, err)
		}
		for _, offering := range result[n].offerings {
			if offering.ServiceEnd == nil || !offering.ServiceEnd.Before(today) {
				result[n].modules = append(result[n].modules, offering)
			}
		}
	}
	return result, nil
}

// importSchoolChoices are the schools with their modules as the answer
// names them, each module name once per school.
func importSchoolChoices(schools []importSchool, templates templateIndex) []carecontract.ContractImportSchool {
	choices := make([]carecontract.ContractImportSchool, len(schools))
	for n, school := range schools {
		choices[n] = carecontract.ContractImportSchool{ID: school.school.ID, Name: school.school.Name, Modules: []carecontract.ContractImportModule{}}
		at := map[string]int{}
		for _, module := range school.modules {
			_, has := templates[[2]int64{school.school.ID, module.ID}]
			key := textnorm.HeaderKey(module.Name)
			if position, seen := at[key]; seen {
				choices[n].Modules[position].HasConditions = choices[n].Modules[position].HasConditions && has
				continue
			}
			at[key] = len(choices[n].Modules)
			choices[n].Modules = append(choices[n].Modules, carecontract.ContractImportModule{ID: module.ID, Name: module.Name,
				PhaseName: module.PhaseName, HasConditions: has})
		}
		slices.SortStableFunc(choices[n].Modules, func(a, b carecontract.ContractImportModule) int { return strings.Compare(a.Name, b.Name) })
	}
	slices.SortStableFunc(choices, func(a, b carecontract.ContractImportSchool) int { return strings.Compare(a.Name, b.Name) })
	return choices
}

// logSchools records in the access log of each school that the office saw
// which of its contracts a file names, or stored new ones there (ADR 0084
// section 7).
func (i *ContractImports) logSchools(ctx context.Context, reviewer carecontract.Reviewer, bySchool map[int64][]int64) error {
	schools := make([]int64, 0, len(bySchool))
	for school := range bySchool {
		schools = append(schools, school)
	}
	slices.Sort(schools)
	for _, school := range schools {
		ids := bySchool[school]
		err := i.ports.Runtime.WithinSchool(ctx, school, func(txCtx context.Context) error {
			return i.ports.Access.RecordAccess(txCtx, carecontract.SchoolAccess{
				OrganizationID: reviewer.OrganizationID, AccountID: reviewer.AccountID,
				Data: carecontract.SchoolAccessContracts, View: accessViewImport, Rows: len(ids), ContractIDs: ids,
			})
		})
		if err != nil {
			return fmt.Errorf("care contract: access log of school %d: %w", school, err)
		}
	}
	return nil
}

// ContractImportLog lists the imports of the Träger, newest first.
func (i *ContractImports) ContractImportLog(ctx context.Context, reviewer carecontract.Reviewer) ([]carecontract.ContractImportEntry, error) {
	if err := checkReviewer(reviewer); err != nil {
		return nil, err
	}
	var entries []carecontract.ContractImportEntry
	err := i.ports.Runtime.WithinTraeger(ctx, reviewer.OrganizationID, func(txCtx context.Context) error {
		var err error
		entries, err = i.store.ContractImportEvents(txCtx, reviewer.OrganizationID)
		return err
	})
	if err != nil {
		return nil, err
	}
	return entries, nil
}
