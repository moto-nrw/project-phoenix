package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"

	"github.com/moto-nrw/project-phoenix/modules/carecontract"
	"github.com/moto-nrw/project-phoenix/modules/carecontract/internal/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

type versionRow struct {
	bun.BaseModel `bun:"table:care_contract.contract_versions"`

	ID                     int64          `bun:"id"`
	VersionNumber          int            `bun:"version_number"`
	Status                 string         `bun:"status"`
	EffectiveFrom          calendar.Date  `bun:"effective_from,type:date"`
	EndsOn                 *calendar.Date `bun:"ends_on,type:date"`
	ChildFirstName         string         `bun:"child_first_name"`
	ChildLastName          string         `bun:"child_last_name"`
	ChildBirthDate         *calendar.Date `bun:"child_birth_date,type:date"`
	SchoolName             string         `bun:"school_name"`
	PhaseName              string         `bun:"phase_name"`
	PriceCents             *int64         `bun:"price_cents"`
	Services               string         `bun:"services"`
	TermKind               string         `bun:"term_kind"`
	TermMonths             *int           `bun:"term_months"`
	NoticeMonths           int            `bun:"notice_months"`
	NoticeTo               string         `bun:"notice_to"`
	SpecialTermination     bool           `bun:"special_termination"`
	SpecialTerminationNote string         `bun:"special_termination_note"`
	StatuteRequired        bool           `bun:"statute_required"`
	StatuteTitle           string         `bun:"statute_title"`
	StatuteURL             string         `bun:"statute_url"`
	ContractPartner        string         `bun:"contract_partner"`
	CreatedAt              time.Time      `bun:"created_at"`
}

const versionColumns = `v.id, v.version_number, v.status, v.effective_from, v.ends_on, v.child_first_name,
	v.child_last_name, v.child_birth_date, v.school_name, v.phase_name, v.price_cents, v.term_kind, v.term_months,
	v.notice_months, v.notice_to, v.special_termination, v.special_termination_note, v.statute_required,
	v.statute_title, v.statute_url, v.contract_partner, v.created_at`

// servicesSQL reads the services of version v as one JSON array, by weekday
// and end time, in the same statement as the version.
const servicesSQL = `COALESCE((SELECT json_agg(json_build_object('weekday', s.weekday,
		'care_offering_id', s.care_offering_id, 'module_name', s.module_name,
		'ends_at', COALESCE(to_char(s.ends_at, 'HH24:MI'), ''),
		'last_day', to_char(s.last_day, 'YYYY-MM-DD'))
		ORDER BY array_position(ARRAY['mon','tue','wed','thu','fri','sat','sun']::text[], s.weekday::text),
			s.ends_at NULLS LAST, s.module_name)
	FROM care_contract.contract_version_services AS s
	WHERE s.organization_id = v.organization_id AND s.contract_version_id = v.id), '[]')::text`

// currentVersionSQL reads the newest version of a contract with its
// services.
const currentVersionSQL = `SELECT ` + versionColumns + `, ` + servicesSQL + ` AS services
	FROM care_contract.contract_versions AS v
	WHERE v.organization_id = ? AND v.contract_id = ? ORDER BY v.version_number DESC LIMIT 1`

func (r versionRow) version() (carecontract.Version, error) {
	services := []carecontract.Service{}
	if err := json.Unmarshal([]byte(r.Services), &services); err != nil {
		return carecontract.Version{}, fmt.Errorf("care contract postgres: read services: %w", err)
	}
	return carecontract.Version{
		ID: r.ID, Number: r.VersionNumber, Status: r.Status, EffectiveFrom: r.EffectiveFrom, EndsOn: r.EndsOn,
		ChildFirstName: r.ChildFirstName, ChildLastName: r.ChildLastName, ChildBirthDate: r.ChildBirthDate,
		SchoolName: r.SchoolName, PhaseName: r.PhaseName, Services: services, PriceCents: r.PriceCents,
		CreatedAt: r.CreatedAt,
		Terms: carecontract.TemplateInput{
			TermKind: r.TermKind, TermMonths: r.TermMonths, NoticeMonths: r.NoticeMonths, NoticeTo: r.NoticeTo,
			SpecialTermination: r.SpecialTermination, SpecialTerminationNote: r.SpecialTerminationNote,
			StatuteRequired: r.StatuteRequired, StatuteTitle: r.StatuteTitle, StatuteURL: r.StatuteURL,
			ContractPartner: r.ContractPartner,
		},
	}, nil
}

// Contract reads one contract of the Träger with its current version, its
// parties and its stored document. forUpdate locks the contract row first.
func (Store) Contract(ctx context.Context, organizationID, contractID int64, forUpdate bool) (ports.ContractRecord, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return ports.ContractRecord{}, err
	}
	if forUpdate {
		var locked int64
		err := db.NewRaw(`SELECT id FROM care_contract.contracts WHERE organization_id = ? AND id = ? FOR UPDATE`,
			organizationID, contractID).Scan(ctx, &locked)
		if errors.Is(err, sql.ErrNoRows) {
			return ports.ContractRecord{}, carecontract.ErrContractNotFound
		}
		if err != nil {
			return ports.ContractRecord{}, fmt.Errorf("care contract postgres: lock contract: %w", err)
		}
	}
	var summary summaryRow
	err = db.NewRaw(contractSQL, organizationID, contractID).Scan(ctx, &summary)
	if errors.Is(err, sql.ErrNoRows) {
		return ports.ContractRecord{}, carecontract.ErrContractNotFound
	}
	if err != nil {
		return ports.ContractRecord{}, fmt.Errorf("care contract postgres: read contract: %w", err)
	}
	var version versionRow
	if err := db.NewRaw(currentVersionSQL, organizationID, contractID).Scan(ctx, &version); err != nil {
		return ports.ContractRecord{}, fmt.Errorf("care contract postgres: read version: %w", err)
	}
	var parties []struct {
		FirstName string `bun:"first_name"`
		LastName  string `bun:"last_name"`
		Email     string `bun:"email"`
		AccountID *int64 `bun:"account_id"`
	}
	if err := db.NewRaw(`SELECT first_name, last_name, email, account_id FROM care_contract.contract_parties
		WHERE organization_id = ? AND contract_id = ? ORDER BY id`, organizationID, contractID).Scan(ctx, &parties); err != nil {
		return ports.ContractRecord{}, fmt.Errorf("care contract postgres: read parties: %w", err)
	}
	current, err := version.version()
	if err != nil {
		return ports.ContractRecord{}, err
	}
	record := ports.ContractRecord{Summary: summary.summary(), Version: current, Parties: []carecontract.Party{}}
	for _, party := range parties {
		record.Parties = append(record.Parties, carecontract.Party{
			FirstName: party.FirstName, LastName: party.LastName, Email: party.Email, HasAccount: party.AccountID != nil,
		})
	}
	var documentIDs []int64
	if err := db.NewRaw(`SELECT id FROM care_contract.contract_documents WHERE organization_id = ? AND contract_version_id = ?`,
		organizationID, version.ID).Scan(ctx, &documentIDs); err != nil {
		return ports.ContractRecord{}, fmt.Errorf("care contract postgres: read document: %w", err)
	}
	if len(documentIDs) > 0 {
		record.DocumentID = &documentIDs[0]
	}
	return record, nil
}

// Mailings reads the sends of a contract, newest first.
func (Store) Mailings(ctx context.Context, organizationID, contractID int64) ([]ports.StoredMailing, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	var rows []struct {
		OutboxID       int64     `bun:"outbox_id"`
		RecipientEmail string    `bun:"recipient_email"`
		SentAt         time.Time `bun:"sent_at"`
	}
	if err := db.NewRaw(`SELECT outbox_id, recipient_email, sent_at FROM care_contract.contract_mailings
		WHERE organization_id = ? AND contract_id = ? ORDER BY sent_at DESC, id DESC`, organizationID, contractID).Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("care contract postgres: read mailings: %w", err)
	}
	result := make([]ports.StoredMailing, len(rows))
	for i, row := range rows {
		result[i] = ports.StoredMailing{OutboxID: row.OutboxID, Recipient: row.RecipientEmail, SentAt: row.SentAt}
	}
	return result, nil
}

// InsertDocument stores the PDF of the contract's current version.
func (Store) InsertDocument(ctx context.Context, organizationID, accountID int64, record ports.ContractRecord, templateVersion int, file carecontract.File) (int64, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return 0, err
	}
	var id int64
	if err := db.NewRaw(`INSERT INTO care_contract.contract_documents (organization_id, contract_id, contract_version_id,
			template_version, filename, pdf, created_by_account_id)
		VALUES (?, ?, ?, ?, ?, ?, ?) RETURNING id`, organizationID, record.Summary.ID, record.Version.ID, templateVersion,
		file.Filename, file.Data, accountID).Scan(ctx, &id); err != nil {
		return 0, fmt.Errorf("care contract postgres: insert document: %w", err)
	}
	return id, nil
}

// Document reads the stored PDF of the contract's current version.
func (Store) Document(ctx context.Context, organizationID, contractID int64) (carecontract.File, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return carecontract.File{}, err
	}
	var row documentRow
	err = db.NewRaw(`SELECT d.filename, d.pdf FROM care_contract.contract_documents AS d
		WHERE d.organization_id = ? AND d.contract_id = ? ORDER BY d.id DESC LIMIT 1`, organizationID, contractID).Scan(ctx, &row)
	return row.file(err, carecontract.ErrContractNotFound)
}

type documentRow struct {
	Filename string `bun:"filename"`
	PDF      []byte `bun:"pdf"`
}

func (r documentRow) file(err error, notFound error) (carecontract.File, error) {
	if errors.Is(err, sql.ErrNoRows) {
		return carecontract.File{}, notFound
	}
	if err != nil {
		return carecontract.File{}, fmt.Errorf("care contract postgres: read document: %w", err)
	}
	return carecontract.File{Filename: r.Filename, ContentType: "application/pdf", Data: r.PDF}, nil
}

// InsertMailing records one send of a document.
func (Store) InsertMailing(ctx context.Context, organizationID, accountID, contractID, documentID, outboxID int64, recipient string) error {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO care_contract.contract_mailings (organization_id, contract_id,
			contract_document_id, outbox_id, recipient_email, sent_by_account_id) VALUES (?, ?, ?, ?, ?, ?)`,
		organizationID, contractID, documentID, outboxID, recipient, accountID); err != nil {
		return fmt.Errorf("care contract postgres: insert mailing: %w", err)
	}
	return nil
}

// MarkSent moves a draft to versendet.
func (Store) MarkSent(ctx context.Context, organizationID, contractID int64, at time.Time) error {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return err
	}
	result, err := db.ExecContext(ctx, `UPDATE care_contract.contracts SET status = ?, sent_at = ?
		WHERE organization_id = ? AND id = ? AND status = ?`,
		carecontract.StatusSent, at, organizationID, contractID, carecontract.StatusDraft)
	if err != nil {
		return fmt.Errorf("care contract postgres: mark sent: %w", err)
	}
	if affected, err := result.RowsAffected(); err == nil && affected != 1 {
		return carecontract.ErrContractNotDraft
	}
	return nil
}

// DeleteDraft removes a draft; the policy refuses any other contract.
func (Store) DeleteDraft(ctx context.Context, organizationID, contractID int64) (bool, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return false, err
	}
	result, err := db.ExecContext(ctx, `DELETE FROM care_contract.contracts WHERE organization_id = ? AND id = ? AND status = ?`,
		organizationID, contractID, carecontract.StatusDraft)
	if err != nil {
		return false, fmt.Errorf("care contract postgres: delete draft: %w", err)
	}
	affected, err := result.RowsAffected()
	return affected == 1, err
}

// inboxSQL reads the contracts sent to an address, with the first send to
// it.
const inboxSQL = `SELECT c.id, v.child_first_name, v.child_last_name, ` + moduleNamesSQL + ` AS module_names, v.school_name,
	v.effective_from, m.first_sent AS sent_at, c.status,
	(SELECT MAX(s.signed_at) FROM care_contract.contract_signatures AS s
		WHERE s.organization_id = c.organization_id AND s.contract_id = c.id) AS signed_at
	FROM care_contract.contracts AS c
	JOIN (SELECT contract_id, MIN(sent_at) AS first_sent FROM care_contract.contract_mailings
		WHERE organization_id = ? AND lower(recipient_email) = lower(?) GROUP BY contract_id) AS m ON m.contract_id = c.id
	JOIN LATERAL (SELECT * FROM care_contract.contract_versions AS cv
		WHERE cv.organization_id = c.organization_id AND cv.contract_id = c.id
		ORDER BY cv.version_number DESC LIMIT 1) AS v ON true
	WHERE c.organization_id = ? AND ` + inboxChildScopeSQL + `
	ORDER BY m.first_sent DESC, c.id DESC`

// InboxContracts lists the contracts sent to the address, newest first.
func (Store) InboxContracts(ctx context.Context, organizationID int64, reader carecontract.InboxReader) ([]carecontract.InboxContract, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	var rows []struct {
		ID             int64         `bun:"id"`
		ChildFirstName string        `bun:"child_first_name"`
		ChildLastName  string        `bun:"child_last_name"`
		ModuleNames    []string      `bun:"module_names,array"`
		SchoolName     string        `bun:"school_name"`
		EffectiveFrom  calendar.Date `bun:"effective_from,type:date"`
		SentAt         time.Time     `bun:"sent_at"`
		Status         string        `bun:"status"`
		SignedAt       *time.Time    `bun:"signed_at"`
	}
	schools, students := inboxChildScope(reader.Children)
	if err := db.NewRaw(inboxSQL, organizationID, reader.Email, organizationID, pgdialect.Array(schools), pgdialect.Array(students)).
		Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("care contract postgres: read inbox: %w", err)
	}
	result := make([]carecontract.InboxContract, len(rows))
	for i, row := range rows {
		result[i] = carecontract.InboxContract{
			ID: row.ID, ChildNames: []string{row.ChildFirstName}, ModuleNames: nonNil(row.ModuleNames),
			SchoolName: row.SchoolName, SentAt: row.SentAt, EffectiveOn: row.EffectiveFrom,
			Status: row.Status, SignedAt: row.SignedAt,
		}
	}
	return result, nil
}

// InboxDocument reads the PDF of a contract sent to the address.
func (Store) InboxDocument(ctx context.Context, organizationID, contractID int64, reader carecontract.InboxReader) (carecontract.File, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return carecontract.File{}, err
	}
	var row documentRow
	schools, students := inboxChildScope(reader.Children)
	err = db.NewRaw(`SELECT d.filename, d.pdf FROM care_contract.contract_mailings AS m
		JOIN care_contract.contract_documents AS d ON d.organization_id = m.organization_id AND d.id = m.contract_document_id
		JOIN care_contract.contracts AS c ON c.organization_id = m.organization_id AND c.id = m.contract_id
		WHERE m.organization_id = ? AND m.contract_id = ? AND lower(m.recipient_email) = lower(?) AND `+inboxChildScopeSQL+`
		ORDER BY m.sent_at DESC, m.id DESC LIMIT 1`, organizationID, contractID, reader.Email, pgdialect.Array(schools), pgdialect.Array(students)).Scan(ctx, &row)
	return row.file(err, carecontract.ErrInboxDocumentNotFound)
}

// inboxChildScopeSQL binds school and student as pairs, never as independent
// lists. An empty scope or an unassigned contract child grants no access.
const inboxChildScopeSQL = `EXISTS (SELECT 1 FROM unnest(?::bigint[], ?::bigint[]) AS allowed(school_id, student_id)
 WHERE allowed.school_id = c.school_id AND allowed.student_id = c.student_id)`

func inboxChildScope(children map[int64][]int64) ([]int64, []int64) {
	schools, students := []int64{}, []int64{}
	for school, permitted := range children {
		for _, student := range permitted {
			schools = append(schools, school)
			students = append(students, student)
		}
	}
	return schools, students
}
