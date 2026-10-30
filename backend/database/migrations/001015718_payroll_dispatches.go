package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	payrollDispatchesVersion     = "1.15.718"
	payrollDispatchesDescription = "Export ans Lohnbüro: automatischer Versand der Monatsdatei nach dem Monatsabschluss, Weg und Monat im Übertragungsprotokoll (#4805)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     payrollDispatchesVersion,
		Description: payrollDispatchesDescription,
		DependsOn: []string{
			studentAccidentReportsVersion, // the last migration before this one
			visitRevisionEventsVersion,    // the newest reinstall of the erasure functions
			exportTransferDispatchVersion, // the transfer journal this extends
		},
	})
	Migrations.MustRegister(payrollDispatchesUp, payrollDispatchesDown)
}

// payrollDispatchesUpSQL creates the durable payroll dispatch (#4805).
//
// One row is one file for the payroll office: the first send of a closed
// month (sequence 0) or a correction after a reopened month was closed again
// (sequence 1, 2, ...). The month close inserts the row in its own
// transaction; the worker builds and sends the file after the commit.
//
//   - channel, format and recipient are snapshotted at the close, so a later
//     settings change does not redirect a file that is already on its way.
//   - attempts counts transport attempts. A failed SFTP transfer returns the
//     row to pending with a later next_attempt_at; after the fourth attempt
//     (the first and three retries) it is failed.
//   - An e-mail dispatch keeps the file in file_data for the 14-day download
//     link. The link is stored only as its SHA-256 fingerprint; the download
//     sets downloaded_at and clears the file, so it can be fetched once. The
//     worker also clears files whose link expired.
//   - tenant_id has no foreign key, like the accident report keys of
//     1.15.717: a key onto platform.schools would change the school erasure's
//     FK fingerprint. The row leaves with the school through the erasure
//     functions installed below.
//
// The transfer journal gains the channel, the month and the e-mail recipient,
// so every send, SFTP or e-mail, is in the one existing protocol.
const payrollDispatchesUpSQL = `
		CREATE TABLE IF NOT EXISTS active.payroll_dispatches (
			id BIGSERIAL PRIMARY KEY,
			tenant_id BIGINT NOT NULL,
			year SMALLINT NOT NULL CHECK (year BETWEEN 2000 AND 2100),
			month SMALLINT NOT NULL CHECK (month BETWEEN 1 AND 12),
			sequence INTEGER NOT NULL CHECK (sequence >= 0),
			channel TEXT NOT NULL CHECK (channel IN ('sftp', 'email')),
			export_format TEXT NOT NULL CHECK (export_format IN ('csv', 'xlsx', 'datev_lodas', 'datev_lug')),
			recipient_email TEXT NOT NULL DEFAULT '',
			actor_account_id BIGINT NOT NULL CHECK (actor_account_id > 0),
			actor_name TEXT NOT NULL DEFAULT '',
			state TEXT NOT NULL DEFAULT 'pending' CHECK (state IN ('pending', 'mailing', 'sent', 'blocked', 'failed')),
			attempts INTEGER NOT NULL DEFAULT 0 CHECK (attempts >= 0),
			next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			lease_token TEXT,
			lease_expires_at TIMESTAMPTZ,
			reason TEXT NOT NULL DEFAULT '',
			filename TEXT NOT NULL DEFAULT '',
			content_type TEXT NOT NULL DEFAULT '',
			byte_size BIGINT NOT NULL DEFAULT 0,
			file_data BYTEA,
			download_token_hash TEXT,
			download_expires_at TIMESTAMPTZ,
			downloaded_at TIMESTAMPTZ,
			mail_outbox_id BIGINT,
			finished_at TIMESTAMPTZ,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			CONSTRAINT uq_payroll_dispatches_month_sequence UNIQUE (tenant_id, year, month, sequence),
			CONSTRAINT chk_payroll_dispatches_lease CHECK ((lease_token IS NULL) = (lease_expires_at IS NULL)),
			CONSTRAINT chk_payroll_dispatches_email CHECK (channel <> 'email' OR recipient_email <> '' OR state = 'blocked'),
			CONSTRAINT chk_payroll_dispatches_link CHECK ((download_token_hash IS NULL) = (download_expires_at IS NULL)),
			CONSTRAINT chk_payroll_dispatches_file CHECK (file_data IS NULL OR download_token_hash IS NOT NULL)
		);

		CREATE UNIQUE INDEX IF NOT EXISTS uq_payroll_dispatches_download_token
			ON active.payroll_dispatches (download_token_hash) WHERE download_token_hash IS NOT NULL;
		CREATE INDEX IF NOT EXISTS idx_payroll_dispatches_due
			ON active.payroll_dispatches (tenant_id, next_attempt_at, id) WHERE state IN ('pending', 'mailing');
		CREATE INDEX IF NOT EXISTS idx_payroll_dispatches_latest
			ON active.payroll_dispatches (tenant_id, created_at DESC, id DESC);

		GRANT SELECT, INSERT, UPDATE, DELETE ON active.payroll_dispatches TO phoenix_tenant;
		GRANT USAGE ON SEQUENCE active.payroll_dispatches_id_seq TO phoenix_tenant;
		GRANT ALL ON active.payroll_dispatches TO phoenix_admin;
		GRANT ALL ON SEQUENCE active.payroll_dispatches_id_seq TO phoenix_admin;

		ALTER TABLE audit.export_transfers
			ADD COLUMN IF NOT EXISTS export_channel TEXT NOT NULL DEFAULT 'sftp',
			ADD COLUMN IF NOT EXISTS export_period TEXT NOT NULL DEFAULT '',
			ADD COLUMN IF NOT EXISTS target_email TEXT NOT NULL DEFAULT '';
		ALTER TABLE audit.export_transfers DROP CONSTRAINT IF EXISTS chk_export_transfers_channel;
		ALTER TABLE audit.export_transfers ADD CONSTRAINT chk_export_transfers_channel
			CHECK (export_channel IN ('sftp', 'email') AND (export_channel = 'sftp' OR target_email <> ''));

		COMMENT ON TABLE active.payroll_dispatches IS
			'Export ans Lohnbüro (#4805): one automatic send of a closed month to the payroll office, first file or correction.';
`

const payrollDispatchesDownSQL = `
		ALTER TABLE audit.export_transfers DROP CONSTRAINT IF EXISTS chk_export_transfers_channel;
		ALTER TABLE audit.export_transfers
			DROP COLUMN IF EXISTS target_email,
			DROP COLUMN IF EXISTS export_period,
			DROP COLUMN IF EXISTS export_channel;
		DROP TABLE IF EXISTS active.payroll_dispatches;
`

func payrollDispatchesUp(ctx context.Context, db *bun.DB) error {
	return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, payrollDispatchesUpSQL); err != nil {
			return fmt.Errorf("create payroll dispatches: %w", err)
		}
		if err := provisionTenantRLS(ctx, tx, "active.payroll_dispatches"); err != nil {
			return err
		}
		for _, step := range []func(context.Context, bun.Tx) error{
			installPayrollDispatchErasurePlan, installPayrollDispatchErasurePreflight, installPayrollDispatchErasureResidual,
			installPayrollDispatchErasureDelete, installPayrollDispatchErasurePreview,
		} {
			if err := step(ctx, tx); err != nil {
				return fmt.Errorf("install the erasure with payroll dispatches: %w", err)
			}
		}
		return nil
	})
}

// payrollDispatchesDown reinstalls the 1.15.705 erasure before it drops the
// table, so the installed plan never names a missing table. E-mail sends in
// the journal cannot be expressed without the new columns, so they go too.
func payrollDispatchesDown(ctx context.Context, db *bun.DB) error {
	return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		for _, step := range []func(context.Context, bun.Tx) error{
			installVisitRevisionErasurePlan, installVisitRevisionErasurePreflight, installVisitRevisionErasureResidual,
			installVisitRevisionErasureDelete, installVisitRevisionErasurePreview,
		} {
			if err := step(ctx, tx); err != nil {
				return fmt.Errorf("reinstall the erasure without payroll dispatches: %w", err)
			}
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM audit.export_transfers WHERE export_channel = 'email'`); err != nil {
			return fmt.Errorf("remove e-mail sends from the transfer journal: %w", err)
		}
		if _, err := tx.ExecContext(ctx, payrollDispatchesDownSQL); err != nil {
			return fmt.Errorf("drop payroll dispatches: %w", err)
		}
		return nil
	})
}
