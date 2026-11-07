package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/uptrace/bun/dialect/pgdialect"

	"github.com/moto-nrw/project-phoenix/modules/carecontract"
	"github.com/moto-nrw/project-phoenix/modules/carecontract/internal/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The signature of a contract (#4679): links, signatures, reminders and the
// conclusion of the contract, all in the Träger transaction.

var _ ports.SignatureStore = (*Store)(nil)

// CurrentDocument reads the frozen PDF of the selected contract version.
func (Store) CurrentDocument(ctx context.Context, organizationID, contractID, versionID int64) (ports.StoredDocument, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return ports.StoredDocument{}, err
	}
	var row struct {
		ID        int64  `bun:"id"`
		VersionID int64  `bun:"contract_version_id"`
		SHA256    string `bun:"pdf_sha256"`
		Filename  string `bun:"filename"`
		PDF       []byte `bun:"pdf"`
	}
	err = db.NewRaw(`SELECT d.id, d.contract_version_id, d.pdf_sha256, d.filename, d.pdf
		FROM care_contract.contract_documents AS d
		WHERE d.organization_id = ? AND d.contract_id = ? AND d.contract_version_id = ?`,
		organizationID, contractID, versionID).Scan(ctx, &row)
	if errors.Is(err, sql.ErrNoRows) {
		return ports.StoredDocument{}, carecontract.ErrContractNotSignable
	}
	if err != nil {
		return ports.StoredDocument{}, fmt.Errorf("care contract postgres: read current document: %w", err)
	}
	return ports.StoredDocument{ID: row.ID, VersionID: row.VersionID, SHA256: row.SHA256,
		File: carecontract.File{Filename: row.Filename, ContentType: "application/pdf", Data: row.PDF}}, nil
}

// InsertSigningLink stores the SHA-256 of a new mail link with what it was
// sent with.
func (Store) InsertSigningLink(ctx context.Context, organizationID, contractID, documentID int64, tokenSHA256 string,
	sent ports.SentState, expiresAt time.Time) error {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO care_contract.contract_signing_links (organization_id, contract_id,
			contract_document_id, token_sha256, sent_document_sha256, sent_version_sha256, expires_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		organizationID, contractID, documentID, tokenSHA256, sent.DocumentSHA256, sent.VersionSHA256, expiresAt); err != nil {
		return fmt.Errorf("care contract postgres: insert signing link: %w", err)
	}
	return nil
}

// SigningLink reads the link of a token and locks it; an unknown token
// answers ErrSigningLinkInvalid.
func (Store) SigningLink(ctx context.Context, organizationID int64, tokenSHA256 string) (ports.SigningLinkRecord, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return ports.SigningLinkRecord{}, err
	}
	var rows []signingLinkRow
	if err := db.NewRaw(`SELECT `+signingLinkColumns+`
		FROM care_contract.contract_signing_links WHERE organization_id = ? AND token_sha256 = ? FOR UPDATE`,
		organizationID, tokenSHA256).Scan(ctx, &rows); err != nil {
		return ports.SigningLinkRecord{}, fmt.Errorf("care contract postgres: read signing link: %w", err)
	}
	if len(rows) == 0 {
		return ports.SigningLinkRecord{}, carecontract.ErrSigningLinkInvalid
	}
	return rows[0].record(), nil
}

// LatestSigningLink reads the newest link of a contract.
func (Store) LatestSigningLink(ctx context.Context, organizationID, contractID int64) (ports.SigningLinkRecord, bool, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return ports.SigningLinkRecord{}, false, err
	}
	var rows []signingLinkRow
	if err := db.NewRaw(`SELECT `+signingLinkColumns+`
		FROM care_contract.contract_signing_links WHERE organization_id = ? AND contract_id = ?
		ORDER BY created_at DESC, id DESC LIMIT 1`, organizationID, contractID).Scan(ctx, &rows); err != nil {
		return ports.SigningLinkRecord{}, false, fmt.Errorf("care contract postgres: read latest signing link: %w", err)
	}
	if len(rows) == 0 {
		return ports.SigningLinkRecord{}, false, nil
	}
	return rows[0].record(), true, nil
}

const signingLinkColumns = `id, contract_id, contract_document_id, sent_document_sha256, sent_version_sha256, expires_at, used_at`

type signingLinkRow struct {
	ID             int64      `bun:"id"`
	ContractID     int64      `bun:"contract_id"`
	DocumentID     int64      `bun:"contract_document_id"`
	DocumentSHA256 string     `bun:"sent_document_sha256"`
	VersionSHA256  string     `bun:"sent_version_sha256"`
	ExpiresAt      time.Time  `bun:"expires_at"`
	UsedAt         *time.Time `bun:"used_at"`
}

func (r signingLinkRow) record() ports.SigningLinkRecord {
	return ports.SigningLinkRecord{ID: r.ID, ContractID: r.ContractID, DocumentID: r.DocumentID,
		Sent:      ports.SentState{DocumentSHA256: r.DocumentSHA256, VersionSHA256: r.VersionSHA256},
		ExpiresAt: r.ExpiresAt, UsedAt: r.UsedAt}
}

// UseSigningLink closes an open, unexpired link; any other answers
// ErrSigningLinkInvalid, so a link signs once.
func (Store) UseSigningLink(ctx context.Context, organizationID, linkID int64, at time.Time) error {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return err
	}
	result, err := db.ExecContext(ctx, `UPDATE care_contract.contract_signing_links SET used_at = ?
		WHERE organization_id = ? AND id = ? AND used_at IS NULL AND expires_at > ?`, at, organizationID, linkID, at)
	if err != nil {
		return fmt.Errorf("care contract postgres: use signing link: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("care contract postgres: count used signing links: %w", err)
	}
	if affected != 1 {
		return carecontract.ErrSigningLinkInvalid
	}
	return nil
}

// InsertSignature stores the conclusion of a version; a second one for the
// same version answers ErrContractNotSignable.
func (Store) InsertSignature(ctx context.Context, organizationID int64, s ports.NewSignature) error {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return err
	}
	var paperName, paperSHA256 *string
	if s.PaperPDF != nil {
		paperName, paperSHA256 = &s.PaperFilename, &s.PaperSHA256
	}
	var id int64
	err = db.NewRaw(`INSERT INTO care_contract.contract_signatures (organization_id, contract_id,
			contract_version_id, contract_document_id, method, channel, audit_signature_id, signer_name, signer_account_id,
			signing_link_id, version_sha256, document_sha256, withdrawal_notice_at, early_start_requested, paper_filename,
			paper_pdf, paper_pdf_sha256, signed_on, signed_at, recorded_by_account_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (contract_version_id) DO NOTHING RETURNING id`,
		organizationID, s.ContractID, s.VersionID, s.DocumentID, s.Method, s.Channel, s.AuditSignatureID, s.SignerName,
		s.SignerAccountID, s.SigningLinkID, s.VersionSHA256, s.DocumentSHA256, s.WithdrawalNoticeAt, s.EarlyStartRequested,
		paperName, s.PaperPDF, paperSHA256, s.SignedOn, s.SignedAt, s.RecordedByAccountID).Scan(ctx, &id)
	if errors.Is(err, sql.ErrNoRows) {
		return carecontract.ErrContractNotSignable
	}
	if err != nil {
		return fmt.Errorf("care contract postgres: insert signature: %w", err)
	}
	return nil
}

// Signature reads the signature of a version; nil when it has none.
func (Store) Signature(ctx context.Context, organizationID, contractID, versionID int64) (*ports.StoredSignature, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	var rows []struct {
		VersionID           int64         `bun:"contract_version_id"`
		DocumentID          *int64        `bun:"contract_document_id"`
		Method              string        `bun:"method"`
		Channel             string        `bun:"channel"`
		AuditSignatureID    *int64        `bun:"audit_signature_id"`
		SignerName          string        `bun:"signer_name"`
		VersionSHA256       string        `bun:"version_sha256"`
		DocumentSHA256      string        `bun:"document_sha256"`
		WithdrawalNoticeAt  *time.Time    `bun:"withdrawal_notice_at"`
		EarlyStartRequested bool          `bun:"early_start_requested"`
		PaperFilename       *string       `bun:"paper_filename"`
		PaperSHA256         *string       `bun:"paper_pdf_sha256"`
		PaperIntact         bool          `bun:"paper_intact"`
		SignedOn            calendar.Date `bun:"signed_on,type:date"`
		SignedAt            time.Time     `bun:"signed_at"`
	}
	if err := db.NewRaw(`SELECT contract_version_id, contract_document_id, method, channel, audit_signature_id, signer_name,
			version_sha256, document_sha256, withdrawal_notice_at, early_start_requested, paper_filename, paper_pdf_sha256,
			COALESCE(paper_pdf_sha256 = encode(sha256(paper_pdf), 'hex'), false) AS paper_intact,
			signed_on, signed_at
		FROM care_contract.contract_signatures WHERE organization_id = ? AND contract_id = ? AND contract_version_id = ?`,
		organizationID, contractID, versionID).Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("care contract postgres: read signature: %w", err)
	}
	if len(rows) == 0 {
		return nil, nil
	}
	row := rows[0]
	stored := &ports.StoredSignature{
		VersionID: row.VersionID, DocumentID: row.DocumentID, Method: row.Method, Channel: row.Channel,
		AuditSignatureID: row.AuditSignatureID, SignerName: row.SignerName, VersionSHA256: row.VersionSHA256,
		DocumentSHA256: row.DocumentSHA256, WithdrawalNoticeAt: row.WithdrawalNoticeAt,
		EarlyStartRequested: row.EarlyStartRequested, SignedOn: row.SignedOn, SignedAt: row.SignedAt, PaperIntact: row.PaperIntact,
	}
	if row.PaperFilename != nil {
		stored.PaperFilename = *row.PaperFilename
	}
	if row.PaperSHA256 != nil {
		stored.PaperSHA256 = *row.PaperSHA256
	}
	return stored, nil
}

// PaperDocument reads the uploaded PDF of a contract signed on paper.
func (Store) PaperDocument(ctx context.Context, organizationID, contractID int64) (carecontract.File, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return carecontract.File{}, err
	}
	var row documentRow
	err = db.NewRaw(`SELECT paper_filename AS filename, paper_pdf AS pdf FROM care_contract.contract_signatures
		WHERE organization_id = ? AND contract_id = ? AND paper_pdf IS NOT NULL ORDER BY id DESC LIMIT 1`,
		organizationID, contractID).Scan(ctx, &row)
	return row.file(err, carecontract.ErrContractNotFound)
}

// Reminders reads the reminder mails of a contract, oldest first.
func (Store) Reminders(ctx context.Context, organizationID, contractID int64) ([]carecontract.Reminder, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	var rows []struct {
		Number    int       `bun:"reminder_number"`
		Recipient string    `bun:"recipient_email"`
		SentAt    time.Time `bun:"sent_at"`
	}
	if err := db.NewRaw(`SELECT reminder_number, recipient_email, sent_at FROM care_contract.contract_reminders
		WHERE organization_id = ? AND contract_id = ? ORDER BY reminder_number`, organizationID, contractID).Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("care contract postgres: read reminders: %w", err)
	}
	reminders := make([]carecontract.Reminder, len(rows))
	for i, row := range rows {
		reminders[i] = carecontract.Reminder{Number: row.Number, Recipient: row.Recipient, SentAt: row.SentAt}
	}
	return reminders, nil
}

// DueReminders lists the sent contracts without signature whose last mail
// or reminder was sent before lastContactBefore and that have fewer than
// maxReminders reminders, with the address of their last mail. A nonzero
// contractID restricts the query to that contract for revalidation under its lock.
func (Store) DueReminders(ctx context.Context, organizationID, contractID int64, lastContactBefore time.Time, maxReminders int) ([]ports.ReminderCandidate, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	var rows []struct {
		ContractID int64  `bun:"contract_id"`
		Number     int    `bun:"number"`
		Recipient  string `bun:"recipient"`
	}
	if err := db.NewRaw(`SELECT c.id AS contract_id, r.sent + 1 AS number, m.recipient_email AS recipient
		FROM care_contract.contracts AS c
		JOIN LATERAL (SELECT recipient_email, sent_at FROM care_contract.contract_mailings AS cm
			WHERE cm.organization_id = c.organization_id AND cm.contract_id = c.id
			ORDER BY cm.sent_at DESC, cm.id DESC LIMIT 1) AS m ON true
		JOIN LATERAL (SELECT COUNT(*)::int AS sent, MAX(cr.sent_at) AS last FROM care_contract.contract_reminders AS cr
			WHERE cr.organization_id = c.organization_id AND cr.contract_id = c.id) AS r ON true
		WHERE c.organization_id = ? AND (? = 0 OR c.id = ?) AND c.status = ? AND r.sent < ?
			AND GREATEST(m.sent_at, COALESCE(r.last, m.sent_at)) <= ?
		ORDER BY c.id`, organizationID, contractID, contractID, carecontract.StatusSent, maxReminders, lastContactBefore).Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("care contract postgres: read due reminders: %w", err)
	}
	candidates := make([]ports.ReminderCandidate, len(rows))
	for i, row := range rows {
		candidates[i] = ports.ReminderCandidate{ContractID: row.ContractID, Number: row.Number, Recipient: row.Recipient}
	}
	return candidates, nil
}

// InsertReminder records a reminder mail.
func (Store) InsertReminder(ctx context.Context, organizationID, contractID int64, number int, outboxID int64, recipient string) error {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO care_contract.contract_reminders (organization_id, contract_id,
			reminder_number, outbox_id, recipient_email) VALUES (?, ?, ?, ?, ?)`,
		organizationID, contractID, number, outboxID, recipient); err != nil {
		return fmt.Errorf("care contract postgres: insert reminder: %w", err)
	}
	return nil
}

// InboxContractAddressed tells whether the contract was sent to the
// reader's address and its child is one the reader may act for.
func (Store) InboxContractAddressed(ctx context.Context, organizationID, contractID int64, reader carecontract.InboxReader) (bool, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return false, err
	}
	schools, students := inboxChildScope(reader.Children)
	var found bool
	if err := db.NewRaw(`SELECT EXISTS (SELECT 1 FROM care_contract.contract_mailings AS m
		JOIN care_contract.contracts AS c ON c.organization_id = m.organization_id AND c.id = m.contract_id
		WHERE m.organization_id = ? AND m.contract_id = ? AND lower(m.recipient_email) = lower(?) AND `+inboxChildScopeSQL+`)`,
		organizationID, contractID, reader.Email, pgdialect.Array(schools), pgdialect.Array(students)).Scan(ctx, &found); err != nil {
		return false, fmt.Errorf("care contract postgres: check inbox contract: %w", err)
	}
	return found, nil
}
