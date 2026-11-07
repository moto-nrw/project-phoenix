// Package postgres persists the Betreuungsvertrag of a Träger. Its tables
// carry organization_id and a Träger policy on app.current_org_id (ADR 0086
// section 5, ADR 0105): every statement here runs in the Träger transaction
// of the Träger it names, and names it once more in its own filter.
package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"

	"github.com/moto-nrw/project-phoenix/modules/carecontract"
	"github.com/moto-nrw/project-phoenix/modules/carecontract/internal/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// Store persists the contracts of a Träger in its Träger transaction.
type Store struct{}

var _ ports.Store = (*Store)(nil)

// NewStore builds the store.
func NewStore() *Store { return &Store{} }

// traegerDB returns the Träger transaction of organizationID. A school,
// administrative or foreign Träger transaction is refused.
func traegerDB(ctx context.Context, organizationID int64) (bun.IDB, error) {
	if organizationID <= 0 || tenant.TraegerTransactionFromContext(ctx) != organizationID {
		return nil, errors.New("care contract postgres: the Träger transaction of this Träger is required")
	}
	transaction, ok := tenant.TransactionFromContext(ctx)
	if !ok {
		return nil, errors.New("care contract postgres: transaction is required")
	}
	tx, ok := transaction.(bun.Tx)
	if !ok {
		return nil, fmt.Errorf("care contract postgres: unsupported transaction %T", transaction)
	}
	return tx, nil
}

type templateRow struct {
	ID                     int64     `bun:"id"`
	SchoolID               int64     `bun:"school_id"`
	CareOfferingID         int64     `bun:"care_offering_id"`
	TermKind               string    `bun:"term_kind"`
	TermMonths             *int      `bun:"term_months"`
	NoticeMonths           int       `bun:"notice_months"`
	NoticeTo               string    `bun:"notice_to"`
	SpecialTermination     bool      `bun:"special_termination"`
	SpecialTerminationNote string    `bun:"special_termination_note"`
	StatuteRequired        bool      `bun:"statute_required"`
	StatuteTitle           string    `bun:"statute_title"`
	StatuteURL             string    `bun:"statute_url"`
	ContractPartner        string    `bun:"contract_partner"`
	UpdatedAt              time.Time `bun:"updated_at"`
}

func (r templateRow) template() carecontract.Template {
	return carecontract.Template{ID: r.ID, UpdatedAt: r.UpdatedAt, TemplateInput: carecontract.TemplateInput{
		TermKind: r.TermKind, TermMonths: r.TermMonths, NoticeMonths: r.NoticeMonths, NoticeTo: r.NoticeTo,
		SpecialTermination: r.SpecialTermination, SpecialTerminationNote: r.SpecialTerminationNote,
		StatuteRequired: r.StatuteRequired, StatuteTitle: r.StatuteTitle, StatuteURL: r.StatuteURL,
		ContractPartner: r.ContractPartner,
	}}
}

const templateColumns = `id, school_id, care_offering_id, term_kind, term_months, notice_months, notice_to,
	special_termination, special_termination_note, statute_required, statute_title, statute_url,
	contract_partner, updated_at`

// Templates reads every template of the Träger.
func (Store) Templates(ctx context.Context, organizationID int64) ([]ports.TemplateRow, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	var rows []templateRow
	if err := db.NewRaw(`SELECT `+templateColumns+` FROM care_contract.contract_templates
		WHERE organization_id = ? ORDER BY school_id, care_offering_id`, organizationID).Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("care contract postgres: templates: %w", err)
	}
	result := make([]ports.TemplateRow, len(rows))
	for i, row := range rows {
		result[i] = ports.TemplateRow{SchoolID: row.SchoolID, OfferingID: row.CareOfferingID, Template: row.template()}
	}
	return result, nil
}

// UpsertTemplate creates or replaces the template of one offering.
func (Store) UpsertTemplate(ctx context.Context, organizationID, accountID, schoolID, offeringID int64, input carecontract.TemplateInput) (carecontract.Template, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return carecontract.Template{}, err
	}
	var row templateRow
	err = db.NewRaw(`INSERT INTO care_contract.contract_templates (organization_id, school_id, care_offering_id,
			term_kind, term_months, notice_months, notice_to, special_termination, special_termination_note,
			statute_required, statute_title, statute_url, contract_partner, updated_by_account_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (organization_id, school_id, care_offering_id) DO UPDATE SET
			term_kind = EXCLUDED.term_kind, term_months = EXCLUDED.term_months,
			notice_months = EXCLUDED.notice_months, notice_to = EXCLUDED.notice_to,
			special_termination = EXCLUDED.special_termination,
			special_termination_note = EXCLUDED.special_termination_note,
			statute_required = EXCLUDED.statute_required, statute_title = EXCLUDED.statute_title,
			statute_url = EXCLUDED.statute_url, contract_partner = EXCLUDED.contract_partner,
			updated_by_account_id = EXCLUDED.updated_by_account_id
		RETURNING `+templateColumns,
		organizationID, schoolID, offeringID, input.TermKind, input.TermMonths, input.NoticeMonths, input.NoticeTo,
		input.SpecialTermination, input.SpecialTerminationNote, input.StatuteRequired, input.StatuteTitle,
		input.StatuteURL, input.ContractPartner, accountID).Scan(ctx, &row)
	if err != nil {
		return carecontract.Template{}, fmt.Errorf("care contract postgres: save template: %w", err)
	}
	return row.template(), nil
}

// ContractKeys reads the keys of the Träger's contracts at the schools.
func (Store) ContractKeys(ctx context.Context, organizationID int64, schoolIDs []int64) (map[ports.ContractKey]bool, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	var rows []struct {
		SchoolID       int64 `bun:"school_id"`
		RequestChildID int64 `bun:"request_child_id"`
	}
	if err := db.NewRaw(`SELECT school_id, request_child_id FROM care_contract.contracts
		WHERE organization_id = ? AND school_id = ANY(?)`, organizationID, pgdialect.Array(schoolIDs)).Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("care contract postgres: contract keys: %w", err)
	}
	keys := make(map[ports.ContractKey]bool, len(rows))
	for _, row := range rows {
		keys[ports.ContractKey{SchoolID: row.SchoolID, RequestChildID: row.RequestChildID}] = true
	}
	return keys, nil
}

// InsertContract stores a draft with its first version, the version's
// services and its party. A second contract for the same application child
// is ErrContractExists.
func (Store) InsertContract(ctx context.Context, organizationID, accountID int64, contract ports.NewContract) (int64, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return 0, err
	}
	var id int64
	err = db.NewRaw(`INSERT INTO care_contract.contracts (organization_id, school_id, request_child_id,
			student_id, created_by_account_id)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT (school_id, request_child_id) DO NOTHING
		RETURNING id`, organizationID, contract.Key.SchoolID, contract.Key.RequestChildID,
		contract.StudentID, accountRef(accountID)).Scan(ctx, &id)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, carecontract.ErrContractExists
	}
	if err != nil {
		return 0, fmt.Errorf("care contract postgres: insert contract: %w", err)
	}
	if _, err := insertVersion(ctx, db, organizationID, id, contract.Version); err != nil {
		return 0, err
	}
	party := contract.Party
	if _, err := db.ExecContext(ctx, `INSERT INTO care_contract.contract_parties (organization_id, contract_id,
			first_name, last_name, email, account_id) VALUES (?, ?, ?, ?, ?, ?)`,
		organizationID, id, party.FirstName, party.LastName, party.Email, party.AccountID); err != nil {
		return 0, fmt.Errorf("care contract postgres: insert party: %w", err)
	}
	return id, nil
}

// insertServices stores the booked services of a version in one statement;
// an empty end time stays NULL.
func insertServices(ctx context.Context, db bun.IDB, organizationID, versionID int64, services []carecontract.Service) error {
	if len(services) == 0 {
		return nil
	}
	weekdays, offerings := make([]string, len(services)), make([]int64, len(services))
	names, ends, lastDays := make([]string, len(services)), make([]string, len(services)), make([]string, len(services))
	for i, service := range services {
		weekdays[i], offerings[i], names[i], ends[i] = service.Weekday, service.CareOfferingID, service.ModuleName, service.EndsAt
		if service.LastDay != nil {
			lastDays[i] = service.LastDay.String()
		}
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO care_contract.contract_version_services
			(organization_id, contract_version_id, weekday, care_offering_id, module_name, ends_at, last_day)
		SELECT ?, ?, s.weekday, s.care_offering_id, s.module_name, NULLIF(s.ends_at, '')::time, NULLIF(s.last_day, '')::date
		FROM unnest(?::text[], ?::bigint[], ?::text[], ?::text[], ?::text[])
			AS s(weekday, care_offering_id, module_name, ends_at, last_day)`,
		organizationID, versionID, pgdialect.Array(weekdays), pgdialect.Array(offerings), pgdialect.Array(names),
		pgdialect.Array(ends), pgdialect.Array(lastDays)); err != nil {
		return fmt.Errorf("care contract postgres: insert services: %w", err)
	}
	return nil
}

// moduleNamesSQL lists the modules of version v in the order of their first
// weekday.
const moduleNamesSQL = `ARRAY(SELECT s.module_name FROM care_contract.contract_version_services AS s
		WHERE s.organization_id = v.organization_id AND s.contract_version_id = v.id
		GROUP BY s.module_name
		ORDER BY MIN(array_position(ARRAY['mon','tue','wed','thu','fri','sat','sun']::text[], s.weekday::text)), s.module_name)`

// summarySQL reads a contract with its current version (currentVersionOrderSQL) and its first
// party; the filter follows.
const summarySQL = `SELECT c.id, c.status, c.school_id, c.created_at, c.sent_at, c.concluded_at, c.conclusion_method,
	v.school_name, v.child_first_name, v.child_last_name, ` + moduleNamesSQL + ` AS module_names, v.effective_from,
	COALESCE(p.first_name || ' ' || p.last_name, '') AS party_name, COALESCE(p.email, '') AS party_email
	FROM care_contract.contracts AS c
	JOIN LATERAL (SELECT * FROM care_contract.contract_versions AS v
		WHERE v.organization_id = c.organization_id AND v.contract_id = c.id
		ORDER BY ` + currentVersionOrderSQL + ` LIMIT 1) AS v ON true
	LEFT JOIN LATERAL (SELECT * FROM care_contract.contract_parties AS cp
		WHERE cp.organization_id = c.organization_id AND cp.contract_id = c.id
		ORDER BY cp.id LIMIT 1) AS p ON true
	WHERE c.organization_id = ?`

// listContractsSQL is the list with its optional filters: status, school
// and a search over the names and the address.
const listContractsSQL = summarySQL + ` AND (? = '' OR c.status = ?) AND (? = 0 OR c.school_id = ?)
	AND (? = '' OR (v.child_first_name || ' ' || v.child_last_name || ' ' || COALESCE(p.first_name || ' ' || p.last_name, '')
		|| ' ' || COALESCE(p.email, '') || ' ' || array_to_string(` + moduleNamesSQL + `, ' ')) ILIKE ?)
	ORDER BY c.created_at DESC, c.id DESC LIMIT ? OFFSET ?`

// contractSQL is one contract.
const contractSQL = summarySQL + ` AND c.id = ?`

type summaryRow struct {
	ID             int64         `bun:"id"`
	Status         string        `bun:"status"`
	SchoolID       int64         `bun:"school_id"`
	CreatedAt      time.Time     `bun:"created_at"`
	SentAt         *time.Time    `bun:"sent_at"`
	ConcludedAt    *time.Time    `bun:"concluded_at"`
	Method         *string       `bun:"conclusion_method"`
	SchoolName     string        `bun:"school_name"`
	ChildFirstName string        `bun:"child_first_name"`
	ChildLastName  string        `bun:"child_last_name"`
	ModuleNames    []string      `bun:"module_names,array"`
	EffectiveFrom  calendar.Date `bun:"effective_from,type:date"`
	PartyName      string        `bun:"party_name"`
	PartyEmail     string        `bun:"party_email"`
}

func (r summaryRow) summary() carecontract.ContractSummary {
	return carecontract.ContractSummary{
		ID: r.ID, Status: r.Status, SchoolID: r.SchoolID, SchoolName: r.SchoolName,
		ChildFirstName: r.ChildFirstName, ChildLastName: r.ChildLastName, ModuleNames: nonNil(r.ModuleNames),
		PartyName: strings.TrimSpace(r.PartyName), PartyEmail: r.PartyEmail, EffectiveFrom: r.EffectiveFrom,
		CreatedAt: r.CreatedAt, SentAt: r.SentAt, Concluded: r.concluded(),
	}
}

func (r summaryRow) concluded() *carecontract.Concluded {
	if r.ConcludedAt == nil || r.Method == nil {
		return nil
	}
	return &carecontract.Concluded{Method: *r.Method, At: *r.ConcludedAt}
}

// accountRef is the acting account, or NULL for the automatic conclusion,
// which acts for no account.
func accountRef(accountID int64) *int64 {
	if accountID <= 0 {
		return nil
	}
	return &accountID
}

// ListContracts lists the contracts of the filter, newest first.
func (Store) ListContracts(ctx context.Context, organizationID int64, filter carecontract.ContractFilter) ([]carecontract.ContractSummary, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	search := ""
	if filter.Search != "" {
		search = "%" + likeEscape(filter.Search) + "%"
	}
	var rows []summaryRow
	if err := db.NewRaw(listContractsSQL, organizationID, filter.Status, filter.Status, filter.SchoolID, filter.SchoolID,
		search, search, filter.Limit+1, filter.Offset).Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("care contract postgres: list contracts: %w", err)
	}
	result := make([]carecontract.ContractSummary, len(rows))
	for i, row := range rows {
		result[i] = row.summary()
	}
	return result, nil
}

func nonNil(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}

func likeEscape(value string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(value)
}

// CountContracts counts the Träger's contracts by status.
func (Store) CountContracts(ctx context.Context, organizationID int64) (map[string]int, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	var rows []struct {
		Status string `bun:"status"`
		Count  int    `bun:"count"`
	}
	if err := db.NewRaw(`SELECT status, COUNT(*) AS count FROM care_contract.contracts
		WHERE organization_id = ? GROUP BY status`, organizationID).Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("care contract postgres: count contracts: %w", err)
	}
	counts := map[string]int{}
	for _, row := range rows {
		counts[row.Status] = row.Count
	}
	return counts, nil
}

// InsertEvents appends entries to the trail in one statement.
func (Store) InsertEvents(ctx context.Context, organizationID int64, events []ports.Event) error {
	if len(events) == 0 {
		return nil
	}
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return err
	}
	contractIDs, schoolIDs := make([]*int64, len(events)), make([]*int64, len(events))
	accountIDs, actions, details := make([]*int64, len(events)), make([]string, len(events)), make([]string, len(events))
	for i, event := range events {
		encoded, err := json.Marshal(event.Details)
		if err != nil {
			return fmt.Errorf("care contract postgres: event details: %w", err)
		}
		contractIDs[i], schoolIDs[i], accountIDs[i], actions[i], details[i] = event.ContractID, event.SchoolID, accountRef(event.AccountID), event.Action, string(encoded)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO care_contract.contract_events
			(organization_id, contract_id, school_id, account_id, action, details)
		SELECT ?, e.contract_id, e.school_id, e.account_id, e.action, e.details::jsonb
		FROM unnest(?::bigint[], ?::bigint[], ?::bigint[], ?::text[], ?::text[])
			AS e(contract_id, school_id, account_id, action, details)`,
		organizationID, pgdialect.Array(contractIDs), pgdialect.Array(schoolIDs), pgdialect.Array(accountIDs),
		pgdialect.Array(actions), pgdialect.Array(details)); err != nil {
		return fmt.Errorf("care contract postgres: insert events: %w", err)
	}
	return nil
}
