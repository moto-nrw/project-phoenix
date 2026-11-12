package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/modules/exporttransfer/internal/domain"
)

// The Träger scope (ADR 0217): config.traeger_export_recipients and
// audit.traeger_export_transfers. Every statement runs in the Träger
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

// TraegerStore persists the recipient and the journal of a Träger.
type TraegerStore struct{ database TraegerDatabase }

func NewTraegerStore(database TraegerDatabase) *TraegerStore {
	return &TraegerStore{database: database}
}

// recipientRow mirrors config.traeger_export_recipients.
type recipientRow struct {
	OrganizationID     int64     `bun:"organization_id"`
	Enabled            bool      `bun:"enabled"`
	Host               string    `bun:"host"`
	Port               int       `bun:"port"`
	Username           string    `bun:"username"`
	SealedPassword     []byte    `bun:"password_sealed"`
	RemoteDirectory    string    `bun:"remote_directory"`
	HostKeyFingerprint string    `bun:"host_key_fingerprint"`
	UpdatedAt          time.Time `bun:"updated_at"`
}

const (
	selectRecipient = `SELECT organization_id, enabled, host, port, username, password_sealed, remote_directory,
		host_key_fingerprint, updated_at FROM config.traeger_export_recipients WHERE organization_id = ?`
	lockRecipient = `SELECT organization_id, enabled, host, port, username, password_sealed, remote_directory,
		host_key_fingerprint, updated_at FROM config.traeger_export_recipients WHERE organization_id = ? FOR UPDATE`
)

func (s *TraegerStore) readRecipient(ctx context.Context, organizationID int64, query string) (domain.StoredRecipient, bool, error) {
	db, err := s.database(ctx, organizationID)
	if err != nil {
		return domain.StoredRecipient{}, false, err
	}
	var row recipientRow
	err = db.NewRaw(query, organizationID).Scan(ctx, &row)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.StoredRecipient{}, false, nil
	}
	if err != nil {
		return domain.StoredRecipient{}, false, fmt.Errorf("export transfer postgres: read the recipient: %w", err)
	}
	return domain.StoredRecipient(row), true, nil
}

// Recipient reads the recipient of the Träger.
func (s *TraegerStore) Recipient(ctx context.Context, organizationID int64) (domain.StoredRecipient, bool, error) {
	return s.readRecipient(ctx, organizationID, selectRecipient)
}

// LockRecipient reads the recipient FOR UPDATE.
func (s *TraegerStore) LockRecipient(ctx context.Context, organizationID int64) (domain.StoredRecipient, bool, error) {
	return s.readRecipient(ctx, organizationID, lockRecipient)
}

// SaveRecipient inserts or replaces the recipient of the Träger.
func (s *TraegerStore) SaveRecipient(ctx context.Context, recipient domain.StoredRecipient, actorAccountID int64) error {
	db, err := s.database(ctx, recipient.OrganizationID)
	if err != nil {
		return err
	}
	var actor *int64
	if actorAccountID > 0 {
		actor = &actorAccountID
	}
	if _, err := db.NewRaw(`INSERT INTO config.traeger_export_recipients
		(organization_id, enabled, host, port, username, password_sealed, remote_directory, host_key_fingerprint, updated_by_account_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (organization_id) DO UPDATE SET enabled = EXCLUDED.enabled, host = EXCLUDED.host, port = EXCLUDED.port,
			username = EXCLUDED.username, password_sealed = EXCLUDED.password_sealed, remote_directory = EXCLUDED.remote_directory,
			host_key_fingerprint = EXCLUDED.host_key_fingerprint, updated_by_account_id = EXCLUDED.updated_by_account_id,
			updated_at = clock_timestamp()`,
		recipient.OrganizationID, recipient.Enabled, recipient.Host, recipient.Port, recipient.Username, recipient.SealedPassword,
		recipient.RemoteDirectory, recipient.HostKeyFingerprint, actor).Exec(ctx); err != nil {
		return fmt.Errorf("export transfer postgres: save the recipient: %w", err)
	}
	return nil
}

// DeleteRecipient removes the recipient of the Träger.
func (s *TraegerStore) DeleteRecipient(ctx context.Context, organizationID int64) error {
	db, err := s.database(ctx, organizationID)
	if err != nil {
		return err
	}
	if _, err := db.NewRaw(`DELETE FROM config.traeger_export_recipients WHERE organization_id = ?`, organizationID).Exec(ctx); err != nil {
		return fmt.Errorf("export transfer postgres: delete the recipient: %w", err)
	}
	return nil
}

// ReplaceSealedPassword writes only the ciphertext of the rekey; no other
// column and no timestamp changes.
func (s *TraegerStore) ReplaceSealedPassword(ctx context.Context, organizationID int64, sealed []byte) error {
	db, err := s.database(ctx, organizationID)
	if err != nil {
		return err
	}
	if _, err := db.NewRaw(`UPDATE config.traeger_export_recipients SET password_sealed = ? WHERE organization_id = ?`,
		sealed, organizationID).Exec(ctx); err != nil {
		return fmt.Errorf("export transfer postgres: rewrap the recipient password: %w", err)
	}
	return nil
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
	var actor *int64
	if entry.ActorAccountID > 0 {
		actor = &entry.ActorAccountID
	}
	var id int64
	if err := db.NewRaw(`INSERT INTO audit.traeger_export_transfers
		(organization_id, actor_account_id, export_kind, export_format, filename, byte_size, target_host, target_port, target_directory,
		 status, failure_reason, dispatch_state, dispatch_expires_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 'failed', ?, 'pending', clock_timestamp() + INTERVAL '2 minutes') RETURNING id`,
		entry.OrganizationID, actor, entry.Kind, entry.Format, entry.Filename, entry.ByteSize, entry.TargetHost, entry.TargetPort,
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

// List reads the newest attempts of the Träger.
func (s *TraegerStore) List(ctx context.Context, organizationID int64, limit int) ([]domain.TraegerJournalRow, error) {
	db, err := s.database(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	var rows []domain.TraegerJournalRow
	if err := db.NewRaw(`SELECT id, created_at, export_format AS format, filename, byte_size, target_host, target_directory,
			status, failure_reason AS reason, dispatch_state,
			(dispatch_state IN ('pending', 'dispatching') AND dispatch_expires_at <= clock_timestamp()) AS dispatch_expired
		FROM audit.traeger_export_transfers WHERE organization_id = ?
		ORDER BY created_at DESC, id DESC LIMIT ?`, organizationID, limit).Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("export transfer postgres: read the Träger journal: %w", err)
	}
	return rows, nil
}
