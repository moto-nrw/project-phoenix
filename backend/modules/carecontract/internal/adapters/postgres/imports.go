package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/uptrace/bun/dialect/pgdialect"

	"github.com/moto-ogs/project-phoenix/modules/carecontract"
	"github.com/moto-ogs/project-phoenix/modules/carecontract/internal/ports"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

// The persistence of the import of existing contracts (#4723). Every
// statement runs in the Träger transaction and names the Träger in its own
// filter; none depends on the number of rows of the file.

// ContractChildren reads the child of every contract at the schools that
// was neither withdrawn nor ended, as its newest version names it.
func (Store) ContractChildren(ctx context.Context, organizationID int64, schoolIDs []int64) ([]ports.ContractChild, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	var rows []struct {
		ContractID int64          `bun:"contract_id"`
		SchoolID   int64          `bun:"school_id"`
		Status     string         `bun:"status"`
		FirstName  string         `bun:"child_first_name"`
		LastName   string         `bun:"child_last_name"`
		BirthDate  *calendar.Date `bun:"child_birth_date,type:date"`
		EndsOn     *calendar.Date `bun:"ends_on,type:date"`
	}
	if err := db.NewRaw(`SELECT DISTINCT ON (c.id) c.id AS contract_id, c.school_id, c.status,
			v.child_first_name, v.child_last_name, v.child_birth_date, COALESCE(c.ends_on, v.ends_on) AS ends_on
		FROM care_contract.contracts AS c
		JOIN care_contract.contract_versions AS v ON v.organization_id = c.organization_id AND v.contract_id = c.id
		WHERE c.organization_id = ? AND c.school_id = ANY(?) AND c.status NOT IN (?, ?)
		ORDER BY c.id, v.version_number DESC`, organizationID, pgdialect.Array(schoolIDs),
		carecontract.StatusWithdrawn, carecontract.StatusEnded).Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("care contract postgres: read the children of the contracts: %w", err)
	}
	children := make([]ports.ContractChild, len(rows))
	for i, row := range rows {
		children[i] = ports.ContractChild{ContractID: row.ContractID, SchoolID: row.SchoolID, Status: row.Status,
			FirstName: row.FirstName, LastName: row.LastName, BirthDate: row.BirthDate, EndsOn: row.EndsOn}
	}
	return children, nil
}

// importedRecord is one imported contract as the insert statement reads it.
type importedRecord struct {
	Line                   int                    `json:"line"`
	SchoolID               int64                  `json:"school_id"`
	ImportKey              string                 `json:"import_origin"`
	ConcludedAt            string                 `json:"concluded_at"`
	EffectiveFrom          calendar.Date          `json:"effective_from"`
	EndsOn                 *calendar.Date         `json:"ends_on"`
	ChildFirstName         string                 `json:"child_first_name"`
	ChildLastName          string                 `json:"child_last_name"`
	ChildBirthDate         *calendar.Date         `json:"child_birth_date"`
	SchoolName             string                 `json:"school_name"`
	PhaseName              string                 `json:"phase_name"`
	PriceCents             *int64                 `json:"price_cents"`
	TermKind               string                 `json:"term_kind"`
	TermMonths             *int                   `json:"term_months"`
	NoticeMonths           int                    `json:"notice_months"`
	NoticeTo               string                 `json:"notice_to"`
	SpecialTermination     bool                   `json:"special_termination"`
	SpecialTerminationNote string                 `json:"special_termination_note"`
	StatuteRequired        bool                   `json:"statute_required"`
	StatuteTitle           string                 `json:"statute_title"`
	StatuteURL             string                 `json:"statute_url"`
	ContractPartner        string                 `json:"contract_partner"`
	PartyFirstName         string                 `json:"party_first_name"`
	PartyLastName          string                 `json:"party_last_name"`
	PartyEmail             string                 `json:"party_email"`
	Services               []carecontract.Service `json:"services"`
}

func importedRecordOf(contract ports.ImportedContract) importedRecord {
	version, terms := contract.Version, contract.Version.Terms
	return importedRecord{
		Line: contract.Line, SchoolID: contract.SchoolID, ImportKey: contract.ImportKey,
		ConcludedAt:   contract.ConcludedAt.UTC().Format(time.RFC3339Nano),
		EffectiveFrom: version.EffectiveFrom, EndsOn: version.EndsOn, ChildFirstName: version.ChildFirstName,
		ChildLastName: version.ChildLastName, ChildBirthDate: version.ChildBirthDate, SchoolName: version.SchoolName,
		PhaseName: version.PhaseName, PriceCents: version.PriceCents, TermKind: terms.TermKind, TermMonths: terms.TermMonths,
		NoticeMonths: terms.NoticeMonths, NoticeTo: terms.NoticeTo, SpecialTermination: terms.SpecialTermination,
		SpecialTerminationNote: terms.SpecialTerminationNote, StatuteRequired: terms.StatuteRequired,
		StatuteTitle: terms.StatuteTitle, StatuteURL: terms.StatuteURL, ContractPartner: terms.ContractPartner,
		PartyFirstName: contract.Party.FirstName, PartyLastName: contract.Party.LastName, PartyEmail: contract.Party.Email,
		Services: version.Services,
	}
}

// insertImportedContractsSQL stores every contract of the file with its
// first version, that version's services and its party in one statement. A
// contract whose import key the school already has is left out, so the
// caller sees fewer contracts than it gave. The contract starts as a draft,
// as the Träger policy demands of every insert.
const insertImportedContractsSQL = `WITH input AS (
	SELECT * FROM jsonb_to_recordset(?::jsonb) AS i(line int, school_id bigint, import_origin text, effective_from date,
		ends_on date, child_first_name text, child_last_name text, child_birth_date date, school_name text, phase_name text,
		price_cents bigint, term_kind text, term_months smallint, notice_months smallint, notice_to text,
		special_termination boolean, special_termination_note text, statute_required boolean, statute_title text,
		statute_url text, contract_partner text, party_first_name text, party_last_name text, party_email text, services jsonb)
), stored AS (
	INSERT INTO care_contract.contracts (organization_id, school_id, import_origin, created_by_account_id)
	SELECT ?, i.school_id, i.import_origin, ? FROM input AS i ORDER BY i.school_id, i.import_origin
	ON CONFLICT (organization_id, school_id, import_origin) WHERE import_origin IS NOT NULL DO NOTHING
	RETURNING id, school_id, import_origin
), contract AS (
	SELECT s.id, i.* FROM stored AS s JOIN input AS i ON i.school_id = s.school_id AND i.import_origin = s.import_origin
), version AS (
	INSERT INTO care_contract.contract_versions (organization_id, contract_id, version_number, status, effective_from,
		ends_on, child_first_name, child_last_name, child_birth_date, school_name, phase_name, price_cents, term_kind,
		term_months, notice_months, notice_to, special_termination, special_termination_note, statute_required,
		statute_title, statute_url, contract_partner, source)
	SELECT ?, c.id, 1, ?, c.effective_from, c.ends_on, c.child_first_name, c.child_last_name, c.child_birth_date,
		c.school_name, c.phase_name, c.price_cents, c.term_kind, c.term_months, c.notice_months, c.notice_to,
		c.special_termination, c.special_termination_note, c.statute_required, c.statute_title, c.statute_url,
		c.contract_partner, ?
	FROM contract AS c
	RETURNING id, contract_id
), service AS (
	INSERT INTO care_contract.contract_version_services (organization_id, contract_version_id, weekday, care_offering_id,
		module_name, ends_at, last_day)
	SELECT ?, v.id, s.weekday, s.care_offering_id, s.module_name, NULLIF(s.ends_at, '')::time, s.last_day
	FROM contract AS c
	JOIN version AS v ON v.contract_id = c.id
	CROSS JOIN LATERAL jsonb_to_recordset(c.services) AS s(weekday text, care_offering_id bigint, module_name text,
		ends_at text, last_day date)
), party AS (
	INSERT INTO care_contract.contract_parties (organization_id, contract_id, first_name, last_name, email)
	SELECT ?, c.id, c.party_first_name, c.party_last_name, c.party_email FROM contract AS c
)
SELECT c.line, c.id AS contract_id FROM contract AS c`

// concludeImportedContractsSQL concludes the contracts the statement above
// stored: the parents hold them on paper, so each counts as sent and
// concluded at the instant its row names.
const concludeImportedContractsSQL = `UPDATE care_contract.contracts AS c
	SET status = ?, sent_at = u.concluded_at, concluded_at = u.concluded_at, conclusion_method = ?
	FROM jsonb_to_recordset(?::jsonb) AS u(contract_id bigint, concluded_at timestamptz)
	WHERE c.organization_id = ? AND c.id = u.contract_id AND c.status = ?`

// InsertImportedContracts stores the contracts of an import file, each
// concluded on paper with its first version in force, and answers the
// contract of each line. A contract whose key the school has already makes
// it ErrContractImportChanged: another import stored it since the preview.
func (Store) InsertImportedContracts(ctx context.Context, organizationID, accountID int64, contracts []ports.ImportedContract) (map[int]int64, error) {
	stored := map[int]int64{}
	if len(contracts) == 0 {
		return stored, nil
	}
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	records := make([]importedRecord, len(contracts))
	concluded := make(map[int]string, len(contracts))
	for i, contract := range contracts {
		records[i] = importedRecordOf(contract)
		concluded[contract.Line] = records[i].ConcludedAt
	}
	encoded, err := json.Marshal(records)
	if err != nil {
		return nil, fmt.Errorf("care contract postgres: encode imported contracts: %w", err)
	}
	var rows []struct {
		Line       int   `bun:"line"`
		ContractID int64 `bun:"contract_id"`
	}
	if err := db.NewRaw(insertImportedContractsSQL, string(encoded), organizationID, accountRef(accountID),
		organizationID, carecontract.VersionEffective, carecontract.SourceImport, organizationID, organizationID).Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("care contract postgres: insert imported contracts: %w", err)
	}
	if len(rows) != len(contracts) {
		return nil, carecontract.ErrContractImportChanged
	}
	type conclusion struct {
		ContractID  int64  `json:"contract_id"`
		ConcludedAt string `json:"concluded_at"`
	}
	conclusions := make([]conclusion, len(rows))
	for i, row := range rows {
		stored[row.Line] = row.ContractID
		conclusions[i] = conclusion{ContractID: row.ContractID, ConcludedAt: concluded[row.Line]}
	}
	encoded, err = json.Marshal(conclusions)
	if err != nil {
		return nil, fmt.Errorf("care contract postgres: encode conclusions: %w", err)
	}
	if _, err := db.ExecContext(ctx, concludeImportedContractsSQL, carecontract.StatusActive, carecontract.MethodPaper,
		string(encoded), organizationID, carecontract.StatusDraft); err != nil {
		return nil, fmt.Errorf("care contract postgres: conclude imported contracts: %w", err)
	}
	return stored, nil
}

// ContractImportEvents lists the imports of the Träger from the trail of the
// contracts they stored, newest first: one entry per file, account and
// instant.
func (Store) ContractImportEvents(ctx context.Context, organizationID int64) ([]carecontract.ContractImportEntry, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	var rows []struct {
		File       string    `bun:"file"`
		AccountID  *int64    `bun:"account_id"`
		ImportedAt time.Time `bun:"imported_at"`
		Contracts  int       `bun:"contracts"`
	}
	if err := db.NewRaw(`SELECT e.details->>? AS file, e.account_id, e.created_at AS imported_at, COUNT(*) AS contracts
		FROM care_contract.contract_events AS e
		WHERE e.organization_id = ? AND e.action = ? AND e.details->>? IS NOT NULL
		GROUP BY e.details->>?, e.account_id, e.created_at
		ORDER BY e.created_at DESC, file LIMIT 100`, ports.ImportActionDetail, organizationID, ports.ActionCreate,
		ports.ImportActionDetail, ports.ImportActionDetail).Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("care contract postgres: read the imports: %w", err)
	}
	entries := make([]carecontract.ContractImportEntry, len(rows))
	for i, row := range rows {
		entries[i] = carecontract.ContractImportEntry{File: row.File, AccountID: row.AccountID, ImportedAt: row.ImportedAt, Contracts: row.Contracts}
	}
	return entries, nil
}
