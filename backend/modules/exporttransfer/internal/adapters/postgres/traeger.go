package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"

	"github.com/moto-ogs/project-phoenix/modules/exporttransfer/internal/domain"
)

// The Träger scope (ADR 0218, ADR 0222): config.traeger_export_recipients,
// config.traeger_export_recipient_formats and audit.traeger_export_transfers. Every statement runs in the Träger
// transaction of the organization it names and filters by that Träger
// itself: below row level security the filter alone keeps a Träger to its
// rows.

// TraegerDatabase yields the transaction of the Träger transaction of
// organizationID, or an error outside it.
type TraegerDatabase func(ctx context.Context, organizationID int64) (bun.IDB, error)

// NewTraegerDatabase binds the transaction and the Träger of the context.
func NewTraegerDatabase(current func(context.Context) (any, bool), traeger func(context.Context) int64) TraegerDatabase {
	return func(ctx context.Context, organizationID int64) (bun.IDB, error) {
		if organizationID <= 0 || traeger(ctx) != organizationID {
			return nil, errors.New("export transfer postgres: the Träger transaction of this Träger is required")
		}
		transaction, ok := current(ctx)
		if !ok {
			return nil, errors.New("export transfer postgres: transaction is required")
		}
		tx, ok := transaction.(bun.Tx)
		if !ok {
			return nil, fmt.Errorf("export transfer postgres: unsupported transaction %T", transaction)
		}
		return tx, nil
	}
}

// TraegerStore persists the recipients, their formats and the journal of a
// Träger.
type TraegerStore struct{ database TraegerDatabase }

func NewTraegerStore(database TraegerDatabase) *TraegerStore {
	return &TraegerStore{database: database}
}

// recipientRow mirrors config.traeger_export_recipients with the formats
// assigned to the recipient.
type recipientRow struct {
	ID                 int64     `bun:"id"`
	OrganizationID     int64     `bun:"organization_id"`
	Name               string    `bun:"name"`
	Enabled            bool      `bun:"enabled"`
	Host               string    `bun:"host"`
	Port               int       `bun:"port"`
	Username           string    `bun:"username"`
	SealedPassword     []byte    `bun:"password_sealed"`
	RemoteDirectory    string    `bun:"remote_directory"`
	HostKeyFingerprint string    `bun:"host_key_fingerprint"`
	UpdatedAt          time.Time `bun:"updated_at"`
	Formats            []string  `bun:"formats,array"`
	// The newest handover of the recipient, when the read joins it.
	LastID              *int64     `bun:"last_id"`
	LastRecipientName   *string    `bun:"last_recipient_name"`
	LastCreatedAt       *time.Time `bun:"last_created_at"`
	LastFormat          *string    `bun:"last_format"`
	LastFilename        *string    `bun:"last_filename"`
	LastByteSize        *int64     `bun:"last_byte_size"`
	LastTargetHost      *string    `bun:"last_target_host"`
	LastTargetDirectory *string    `bun:"last_target_directory"`
	LastStatus          *string    `bun:"last_status"`
	LastReason          *string    `bun:"last_reason"`
	LastDispatchState   *string    `bun:"last_dispatch_state"`
	LastDispatchExpired *bool      `bun:"last_dispatch_expired"`
}

// recipientColumns reads a recipient with its formats in the order of
// domain.Formats; both filters name the Träger of the recipient.
const recipientColumns = `recipient.id, recipient.organization_id, recipient.name, recipient.enabled, recipient.host, recipient.port,
	recipient.username, recipient.password_sealed, recipient.remote_directory, recipient.host_key_fingerprint, recipient.updated_at,
	ARRAY(SELECT assigned.export_format FROM config.traeger_export_recipient_formats AS assigned
		WHERE assigned.organization_id = recipient.organization_id AND assigned.recipient_id = recipient.id
		ORDER BY array_position(ARRAY['sollstellung', 'datev', 'pain008']::varchar[], assigned.export_format)) AS formats`

const (
	selectRecipient = `SELECT ` + recipientColumns + ` FROM config.traeger_export_recipients AS recipient
		WHERE recipient.organization_id = ? AND recipient.id = ?`
	lockRecipient = selectRecipient + ` FOR UPDATE OF recipient`
	// selectRecipients joins the newest handover of each recipient: its
	// error state stays visible until the next handover.
	selectRecipients = `SELECT ` + recipientColumns + `, last.id AS last_id, last.recipient_name AS last_recipient_name,
		last.created_at AS last_created_at,
		last.export_format AS last_format, last.filename AS last_filename, last.byte_size AS last_byte_size,
		last.target_host AS last_target_host, last.target_directory AS last_target_directory, last.status AS last_status,
		last.failure_reason AS last_reason, last.dispatch_state AS last_dispatch_state,
		(last.dispatch_state IN ('pending', 'dispatching') AND last.dispatch_expires_at <= clock_timestamp()) AS last_dispatch_expired
		FROM config.traeger_export_recipients AS recipient
		LEFT JOIN LATERAL (SELECT journal.* FROM audit.traeger_export_transfers AS journal
			WHERE journal.organization_id = recipient.organization_id AND journal.recipient_id = recipient.id
			ORDER BY journal.created_at DESC, journal.id DESC LIMIT 1) AS last ON true
		WHERE recipient.organization_id = ?
		ORDER BY recipient.name COLLATE "und-x-icu", recipient.id`
	selectRecipientForFormat = `SELECT ` + recipientColumns + ` FROM config.traeger_export_recipient_formats AS assignment
		JOIN config.traeger_export_recipients AS recipient
			ON recipient.organization_id = assignment.organization_id AND recipient.id = assignment.recipient_id
		WHERE assignment.organization_id = ? AND assignment.export_format = ?`
	lockRecipientsAfter = `SELECT ` + recipientColumns + ` FROM config.traeger_export_recipients AS recipient
		WHERE recipient.organization_id = ? AND recipient.id > ? ORDER BY recipient.id LIMIT ? FOR UPDATE OF recipient`
)

func storedRecipient(row recipientRow) domain.StoredRecipient {
	recipient := domain.StoredRecipient{ID: row.ID, OrganizationID: row.OrganizationID, Name: row.Name, Enabled: row.Enabled,
		Host: row.Host, Port: row.Port, Username: row.Username, SealedPassword: row.SealedPassword, RemoteDirectory: row.RemoteDirectory,
		HostKeyFingerprint: row.HostKeyFingerprint, UpdatedAt: row.UpdatedAt, Formats: row.Formats}
	if recipient.Formats == nil {
		recipient.Formats = []string{}
	}
	if row.LastID != nil {
		id := row.ID
		recipient.LastHandover = &domain.TraegerJournalRow{ID: *row.LastID, CreatedAt: *row.LastCreatedAt, RecipientID: &id,
			RecipientName: *row.LastRecipientName, Format: *row.LastFormat, Filename: *row.LastFilename, ByteSize: *row.LastByteSize,
			TargetHost: *row.LastTargetHost, TargetDirectory: *row.LastTargetDirectory, Status: *row.LastStatus, Reason: *row.LastReason,
			DispatchState: *row.LastDispatchState, DispatchExpired: row.LastDispatchExpired != nil && *row.LastDispatchExpired}
	}
	return recipient
}

// Recipients reads every recipient of the Träger with its formats and its
// newest handover.
func (s *TraegerStore) Recipients(ctx context.Context, organizationID int64) ([]domain.StoredRecipient, error) {
	db, err := s.database(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	var rows []recipientRow
	if err := db.NewRaw(selectRecipients, organizationID).Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("export transfer postgres: read the recipients: %w", err)
	}
	recipients := make([]domain.StoredRecipient, 0, len(rows))
	for _, row := range rows {
		recipients = append(recipients, storedRecipient(row))
	}
	return recipients, nil
}

// Recipient reads one recipient of the Träger.
func (s *TraegerStore) Recipient(ctx context.Context, organizationID, recipientID int64) (domain.StoredRecipient, bool, error) {
	db, err := s.database(ctx, organizationID)
	if err != nil {
		return domain.StoredRecipient{}, false, err
	}
	var row recipientRow
	return oneRecipient(row, db.NewRaw(selectRecipient, organizationID, recipientID).Scan(ctx, &row))
}

// LockRecipient reads one recipient of the Träger FOR UPDATE.
func (s *TraegerStore) LockRecipient(ctx context.Context, organizationID, recipientID int64) (domain.StoredRecipient, bool, error) {
	db, err := s.database(ctx, organizationID)
	if err != nil {
		return domain.StoredRecipient{}, false, err
	}
	var row recipientRow
	return oneRecipient(row, db.NewRaw(lockRecipient, organizationID, recipientID).Scan(ctx, &row))
}

// RecipientForFormat reads the recipient the format of the Träger goes to.
func (s *TraegerStore) RecipientForFormat(ctx context.Context, organizationID int64, format string) (domain.StoredRecipient, bool, error) {
	db, err := s.database(ctx, organizationID)
	if err != nil {
		return domain.StoredRecipient{}, false, err
	}
	var row recipientRow
	return oneRecipient(row, db.NewRaw(selectRecipientForFormat, organizationID, format).Scan(ctx, &row))
}

func oneRecipient(row recipientRow, err error) (domain.StoredRecipient, bool, error) {
	if errors.Is(err, sql.ErrNoRows) {
		return domain.StoredRecipient{}, false, nil
	}
	if err != nil {
		return domain.StoredRecipient{}, false, fmt.Errorf("export transfer postgres: read the recipient: %w", err)
	}
	return storedRecipient(row), true, nil
}

// LockRecipientsAfter reads the next recipients of the Träger FOR UPDATE.
func (s *TraegerStore) LockRecipientsAfter(ctx context.Context, organizationID, cursor int64, limit int) ([]domain.StoredRecipient, error) {
	db, err := s.database(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	var rows []recipientRow
	if err := db.NewRaw(lockRecipientsAfter, organizationID, cursor, limit).Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("export transfer postgres: lock the recipients: %w", err)
	}
	recipients := make([]domain.StoredRecipient, 0, len(rows))
	for _, row := range rows {
		recipients = append(recipients, storedRecipient(row))
	}
	return recipients, nil
}

func actorOf(actorAccountID int64) *int64 {
	if actorAccountID <= 0 {
		return nil
	}
	return &actorAccountID
}

// InsertRecipient adds a recipient to the Träger and answers its id. A
// name another recipient of the Träger has, in any case, inserts nothing
// and answers domain.ErrNameTaken; the unique index decides.
func (s *TraegerStore) InsertRecipient(ctx context.Context, recipient domain.StoredRecipient, actorAccountID int64) (int64, error) {
	db, err := s.database(ctx, recipient.OrganizationID)
	if err != nil {
		return 0, err
	}
	var id int64
	err = db.NewRaw(`INSERT INTO config.traeger_export_recipients
		(organization_id, name, enabled, host, port, username, password_sealed, remote_directory, host_key_fingerprint, updated_by_account_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (organization_id, lower(name COLLATE "und-x-icu")) DO NOTHING RETURNING id`,
		recipient.OrganizationID, recipient.Name, recipient.Enabled, recipient.Host, recipient.Port, recipient.Username,
		recipient.SealedPassword, recipient.RemoteDirectory, recipient.HostKeyFingerprint, actorOf(actorAccountID)).Scan(ctx, &id)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, domain.ErrNameTaken
	}
	if err != nil {
		return 0, fmt.Errorf("export transfer postgres: add the recipient: %w", err)
	}
	return id, nil
}

// UpdateRecipient replaces the values of one recipient of the Träger. It
// answers false for a recipient the Träger does not have and
// domain.ErrNameTaken for a name another of its recipients has.
func (s *TraegerStore) UpdateRecipient(ctx context.Context, recipient domain.StoredRecipient, actorAccountID int64) (bool, error) {
	db, err := s.database(ctx, recipient.OrganizationID)
	if err != nil {
		return false, err
	}
	var outcome struct {
		Found bool `bun:"found"`
		Taken bool `bun:"taken"`
	}
	if err := db.NewRaw(`WITH taken AS (
			SELECT 1 FROM config.traeger_export_recipients AS other
			WHERE other.organization_id = ?0 AND lower(other.name COLLATE "und-x-icu") = lower(?2 COLLATE "und-x-icu")
				AND other.id <> ?1),
		updated AS (
			UPDATE config.traeger_export_recipients SET name = ?2, enabled = ?3, host = ?4, port = ?5, username = ?6,
				password_sealed = ?7, remote_directory = ?8, host_key_fingerprint = ?9, updated_by_account_id = ?10,
				updated_at = clock_timestamp()
			WHERE organization_id = ?0 AND id = ?1 AND NOT EXISTS (SELECT 1 FROM taken) RETURNING id)
		SELECT EXISTS (SELECT 1 FROM config.traeger_export_recipients WHERE organization_id = ?0 AND id = ?1) AS found,
			EXISTS (SELECT 1 FROM taken) AS taken`,
		recipient.OrganizationID, recipient.ID, recipient.Name, recipient.Enabled, recipient.Host, recipient.Port, recipient.Username,
		recipient.SealedPassword, recipient.RemoteDirectory, recipient.HostKeyFingerprint, actorOf(actorAccountID)).Scan(ctx, &outcome); err != nil {
		return false, fmt.Errorf("export transfer postgres: save the recipient: %w", err)
	}
	if outcome.Found && outcome.Taken {
		return false, domain.ErrNameTaken
	}
	return outcome.Found, nil
}

// DeleteRecipient removes one recipient of the Träger; its formats go with
// it and its journal rows keep their name without the reference.
func (s *TraegerStore) DeleteRecipient(ctx context.Context, organizationID, recipientID int64) (bool, error) {
	db, err := s.database(ctx, organizationID)
	if err != nil {
		return false, err
	}
	result, err := db.NewRaw(`DELETE FROM config.traeger_export_recipients WHERE organization_id = ? AND id = ?`,
		organizationID, recipientID).Exec(ctx)
	if err != nil {
		return false, fmt.Errorf("export transfer postgres: delete the recipient: %w", err)
	}
	return affected(result)
}

func affected(result sql.Result) (bool, error) {
	n, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("export transfer postgres: count the changed rows: %w", err)
	}
	return n > 0, nil
}

// ReplaceSealedPasswords writes only the ciphertexts of a rekey batch, in
// one statement; no other column and no timestamp changes. ids and sealed
// pair up by position.
func (s *TraegerStore) ReplaceSealedPasswords(ctx context.Context, organizationID int64, ids []int64, sealed [][]byte) error {
	if len(ids) != len(sealed) {
		return errors.New("export transfer postgres: every rewrapped password needs its recipient")
	}
	if len(ids) == 0 {
		return nil
	}
	db, err := s.database(ctx, organizationID)
	if err != nil {
		return err
	}
	if _, err := db.NewRaw(`UPDATE config.traeger_export_recipients AS recipient SET password_sealed = rewrapped.sealed
		FROM unnest(?::bigint[], ?::bytea[]) AS rewrapped (id, sealed)
		WHERE recipient.organization_id = ? AND recipient.id = rewrapped.id`,
		pgdialect.Array(ids), pgdialect.Array(sealed), organizationID).Exec(ctx); err != nil {
		return fmt.Errorf("export transfer postgres: rewrap the recipient passwords: %w", err)
	}
	return nil
}

// AssignFormat sends the format to one recipient of the Träger, or leaves
// it to the download for recipientID 0. A recipient of another Träger
// matches no row and answers false; the composite reference refuses it too.
func (s *TraegerStore) AssignFormat(ctx context.Context, organizationID int64, format string, recipientID, actorAccountID int64) (bool, error) {
	db, err := s.database(ctx, organizationID)
	if err != nil {
		return false, err
	}
	if recipientID == 0 {
		if _, err := db.NewRaw(`DELETE FROM config.traeger_export_recipient_formats WHERE organization_id = ? AND export_format = ?`,
			organizationID, format).Exec(ctx); err != nil {
			return false, fmt.Errorf("export transfer postgres: leave the format to the download: %w", err)
		}
		return true, nil
	}
	result, err := db.NewRaw(`INSERT INTO config.traeger_export_recipient_formats (organization_id, export_format, recipient_id, updated_by_account_id)
		SELECT recipient.organization_id, ?, recipient.id, ? FROM config.traeger_export_recipients AS recipient
		WHERE recipient.organization_id = ? AND recipient.id = ?
		ON CONFLICT (organization_id, export_format) DO UPDATE SET recipient_id = EXCLUDED.recipient_id,
			updated_by_account_id = EXCLUDED.updated_by_account_id, updated_at = clock_timestamp()`,
		format, actorOf(actorAccountID), organizationID, recipientID).Exec(ctx)
	if err != nil {
		return false, fmt.Errorf("export transfer postgres: assign the format: %w", err)
	}
	return affected(result)
}

// Begin records the intent of a handover with its two-minute fence.
func (s *TraegerStore) Begin(ctx context.Context, entry domain.TraegerJournalEntry) (int64, error) {
	db, err := s.database(ctx, entry.OrganizationID)
	if err != nil {
		return 0, err
	}
	if entry.Reason == "" {
		return 0, errors.New("export transfer postgres: an unfinished attempt must name a reason")
	}
	if entry.RecipientID <= 0 {
		return 0, errors.New("export transfer postgres: an attempt must name its recipient")
	}
	var id int64
	if err := db.NewRaw(`INSERT INTO audit.traeger_export_transfers
		(organization_id, recipient_id, recipient_name, actor_account_id, export_kind, export_format, filename, byte_size,
		 target_host, target_port, target_directory, status, failure_reason, dispatch_state, dispatch_expires_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'failed', ?, 'pending', clock_timestamp() + INTERVAL '2 minutes') RETURNING id`,
		entry.OrganizationID, entry.RecipientID, entry.RecipientName, actorOf(entry.ActorAccountID), entry.Kind, entry.Format, entry.Filename, entry.ByteSize, entry.TargetHost, entry.TargetPort,
		entry.TargetDirectory, entry.Reason).Scan(ctx, &id); err != nil {
		return 0, fmt.Errorf("export transfer postgres: record the Träger intent: %w", err)
	}
	return id, nil
}

// Claim installs the dispatch fence for exactly this committed intent.
func (s *TraegerStore) Claim(ctx context.Context, organizationID, id int64) (string, error) {
	db, err := s.database(ctx, organizationID)
	if err != nil {
		return "", err
	}
	var token string
	if err := db.NewRaw(`SELECT audit.claim_traeger_export_transfer(?, ?)`, organizationID, id).Scan(ctx, &token); err != nil {
		return "", err
	}
	if token == "" {
		return "", errors.New("export transfer intent cannot be dispatched")
	}
	return token, nil
}

// Complete settles exactly one live fence.
func (s *TraegerStore) Complete(ctx context.Context, organizationID, id int64, token string, success bool, reason string) error {
	db, err := s.database(ctx, organizationID)
	if err != nil {
		return err
	}
	var done bool
	if err := db.NewRaw(`SELECT audit.complete_traeger_export_transfer(?, ?, ?, ?, ?)`, organizationID, id, token, success, reason).
		Scan(ctx, &done); err != nil {
		return err
	}
	if !done {
		return errors.New("export transfer dispatch fence expired")
	}
	return nil
}

// List reads the newest attempts of the Träger, of one recipient or of all
// for recipientID 0.
func (s *TraegerStore) List(ctx context.Context, organizationID, recipientID int64, limit int) ([]domain.TraegerJournalRow, error) {
	db, err := s.database(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	var rows []domain.TraegerJournalRow
	if err := db.NewRaw(`SELECT id, created_at, recipient_id, recipient_name, export_format AS format, filename, byte_size,
			target_host, target_directory, status, failure_reason AS reason, dispatch_state,
			(dispatch_state IN ('pending', 'dispatching') AND dispatch_expires_at <= clock_timestamp()) AS dispatch_expired
		FROM audit.traeger_export_transfers WHERE organization_id = ? AND (? = 0 OR recipient_id = ?)
		ORDER BY created_at DESC, id DESC LIMIT ?`, organizationID, recipientID, recipientID, limit).Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("export transfer postgres: read the Träger journal: %w", err)
	}
	return rows, nil
}
