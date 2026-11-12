package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const traegerExportTransferVersion = "1.15.953"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     traegerExportTransferVersion,
		Description: "Export Transfer mit Träger-Scope: Empfänger, Journal und Übergaben im Exportprotokoll (#6824, ADR 0218)",
		DependsOn: []string{
			demoVisitorErasureFamilyDebtorNumbersVersion, // the newest migration on development (1.15.951)
			exportTransferDispatchVersion,                // the school journal and fence this one mirrors
			parentBillingDatevExportFilesVersion,         // the export protocol whose files are handed over
		},
	})
	Migrations.MustRegister(traegerExportTransferUp, traegerExportTransferDown)
}

// traegerExportTransferUpSQL adds the Träger scope of Export Transfer
// (ADR 0218). All three tables follow ADR 0105: organization_id, no
// tenant_id, FORCE RLS on app.current_org_id, gone with the Träger.
//
//   - config.traeger_export_recipients (Export Transfer): the one recipient
//     of a Träger. The password exists only sealed with the bank ring; no
//     column holds it in plain text.
//   - audit.traeger_export_transfers (Export Transfer): one row per attempt,
//     with the one-shot dispatch fence of the school journal (1.15.463). The
//     tenant role only inserts and reads; claim and completion run through
//     two functions that check the Träger of the transaction. No credential
//     and no file content.
//   - parent_billing.export_file_transfers (Elternabrechnung): the handover
//     of a file of the export protocol, pending from the transaction that
//     made the file until the one that completes the journal.
const traegerExportTransferUpSQL = `
CREATE TABLE config.traeger_export_recipients (
	organization_id BIGINT PRIMARY KEY REFERENCES platform.organizations(id) ON DELETE CASCADE,
	enabled BOOLEAN NOT NULL DEFAULT false,
	host VARCHAR(253) NOT NULL DEFAULT '',
	port INTEGER NOT NULL DEFAULT 22,
	username VARCHAR(128) NOT NULL DEFAULT '',
	password_sealed BYTEA,
	remote_directory VARCHAR(512) NOT NULL DEFAULT '',
	host_key_fingerprint VARCHAR(64) NOT NULL DEFAULT '',
	updated_by_account_id BIGINT REFERENCES auth.accounts(id) ON DELETE SET NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	CONSTRAINT chk_traeger_export_recipients_port CHECK (port BETWEEN 1 AND 65535),
	CONSTRAINT chk_traeger_export_recipients_text CHECK (
		host !~ '[[:space:][:cntrl:]]' AND username !~ '[[:cntrl:]]'
		AND remote_directory !~ '[[:cntrl:]]' AND host_key_fingerprint !~ '[[:space:][:cntrl:]]'),
	CONSTRAINT chk_traeger_export_recipients_password CHECK (password_sealed IS NULL OR octet_length(password_sealed) > 0)
);
COMMENT ON TABLE config.traeger_export_recipients IS
	'The recipient of the finance exports of a Träger (#6824, ADR 0218): SFTP address, user, sealed password, directory and host key fingerprint.';
COMMENT ON COLUMN config.traeger_export_recipients.password_sealed IS
	'Password sealed with the bank ring, bound to the column and the Träger; never answered, never logged.';

CREATE TABLE audit.traeger_export_transfers (
	id BIGSERIAL PRIMARY KEY,
	organization_id BIGINT NOT NULL REFERENCES platform.organizations(id) ON DELETE CASCADE,
	actor_account_id BIGINT REFERENCES auth.accounts(id) ON DELETE SET NULL,
	export_kind TEXT NOT NULL,
	export_format TEXT NOT NULL,
	filename TEXT NOT NULL,
	byte_size BIGINT NOT NULL,
	target_host TEXT NOT NULL,
	target_port INTEGER NOT NULL,
	target_directory TEXT NOT NULL,
	status TEXT NOT NULL CHECK (status IN ('success', 'failed')),
	failure_reason TEXT NOT NULL DEFAULT '',
	dispatch_state TEXT NOT NULL CHECK (dispatch_state IN ('terminal', 'pending', 'dispatching', 'unknown')),
	dispatch_token TEXT,
	dispatch_expires_at TIMESTAMPTZ,
	created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	CONSTRAINT chk_traeger_export_transfers_size CHECK (byte_size >= 0),
	CONSTRAINT chk_traeger_export_transfers_reason CHECK ((status = 'success') = (failure_reason = '')),
	CONSTRAINT chk_traeger_export_transfers_fence CHECK (
		(dispatch_state = 'terminal' AND dispatch_token IS NULL) OR
		(dispatch_state IN ('pending', 'unknown') AND dispatch_token IS NULL AND dispatch_expires_at IS NOT NULL) OR
		(dispatch_state = 'dispatching' AND dispatch_token IS NOT NULL AND dispatch_expires_at IS NOT NULL))
);
CREATE INDEX idx_traeger_export_transfers_organization ON audit.traeger_export_transfers (organization_id, created_at DESC, id DESC);
CREATE INDEX idx_traeger_export_transfers_expiry ON audit.traeger_export_transfers (dispatch_expires_at, id)
	WHERE dispatch_state IN ('pending', 'dispatching');
COMMENT ON TABLE audit.traeger_export_transfers IS
	'Journal of the handovers of finance exports to the recipient of a Träger (#6824, ADR 0218): who, which file, where, outcome; no credentials, no content.';

-- The only changes of a committed attempt, as in 1.15.463: claim the fence
-- once, then settle it once. Both check the Träger of the transaction.
CREATE FUNCTION audit.claim_traeger_export_transfer(p_organization BIGINT, p_id BIGINT) RETURNS TEXT
LANGUAGE plpgsql SECURITY DEFINER SET search_path = pg_catalog, audit AS $$
DECLARE token TEXT;
BEGIN
	IF p_organization IS NULL OR p_organization IS DISTINCT FROM NULLIF(current_setting('app.current_org_id', true), '')::BIGINT THEN
		RAISE EXCEPTION 'the Träger does not match the transaction' USING ERRCODE = '42501';
	END IF;
	token := public.uuid_generate_v4()::text;
	UPDATE audit.traeger_export_transfers SET dispatch_state = 'dispatching', dispatch_token = token,
		dispatch_expires_at = clock_timestamp() + INTERVAL '2 minutes', updated_at = clock_timestamp()
	WHERE organization_id = p_organization AND id = p_id AND dispatch_state = 'pending' AND dispatch_expires_at > clock_timestamp();
	IF NOT FOUND THEN RETURN ''; END IF;
	RETURN token;
END; $$;

CREATE FUNCTION audit.complete_traeger_export_transfer(p_organization BIGINT, p_id BIGINT, p_token TEXT, p_success BOOLEAN, p_reason TEXT)
RETURNS BOOLEAN
LANGUAGE plpgsql SECURITY DEFINER SET search_path = pg_catalog, audit AS $$
BEGIN
	IF p_organization IS NULL OR p_organization IS DISTINCT FROM NULLIF(current_setting('app.current_org_id', true), '')::BIGINT THEN
		RAISE EXCEPTION 'the Träger does not match the transaction' USING ERRCODE = '42501';
	END IF;
	IF p_success IS NULL OR p_reason IS NULL OR (p_success AND p_reason <> '') OR (NOT p_success AND p_reason NOT IN
		('address_denied', 'host_key_mismatch', 'authentication_rejected', 'connection_failed', 'upload_failed', 'file_too_large', 'internal_error')) THEN
		RAISE EXCEPTION 'invalid transfer outcome' USING ERRCODE = '22023';
	END IF;
	UPDATE audit.traeger_export_transfers SET dispatch_state = 'terminal', dispatch_token = NULL,
		status = CASE WHEN p_success THEN 'success' ELSE 'failed' END, failure_reason = p_reason, updated_at = clock_timestamp()
	WHERE organization_id = p_organization AND id = p_id AND dispatch_state = 'dispatching'
		AND dispatch_token = p_token AND dispatch_expires_at > clock_timestamp();
	RETURN FOUND;
END; $$;

CREATE TABLE parent_billing.export_file_transfers (
	organization_id BIGINT NOT NULL,
	export_file_id BIGINT NOT NULL,
	target_host VARCHAR(253) NOT NULL,
	target_directory VARCHAR(512) NOT NULL,
	state VARCHAR(16) NOT NULL DEFAULT 'pending',
	failure_reason VARCHAR(32) NOT NULL DEFAULT '',
	started_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	settled_at TIMESTAMPTZ,
	PRIMARY KEY (organization_id, export_file_id),
	CONSTRAINT fk_export_file_transfers_file FOREIGN KEY (organization_id, export_file_id)
		REFERENCES parent_billing.export_files (organization_id, id) ON DELETE CASCADE,
	CONSTRAINT chk_export_file_transfers_state CHECK (
		(state = 'pending' AND failure_reason = '' AND settled_at IS NULL) OR
		(state = 'transferred' AND failure_reason = '' AND settled_at IS NOT NULL) OR
		(state = 'failed' AND failure_reason <> '' AND settled_at IS NOT NULL))
);
COMMENT ON TABLE parent_billing.export_file_transfers IS
	'Handover of a file of the export protocol to the recipient of the Träger (#6824, ADR 0218): where to, pending until the transfer settles, then transferred or failed with its reason.';

-- A handover settles once; its file, target and start never change.
CREATE FUNCTION parent_billing.refuse_export_file_transfer_rewrite() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
	IF OLD.state <> 'pending' THEN
		RAISE EXCEPTION 'a settled handover never changes' USING ERRCODE = '55000';
	END IF;
	IF (NEW.organization_id, NEW.export_file_id, NEW.target_host, NEW.target_directory, NEW.started_at)
		IS DISTINCT FROM (OLD.organization_id, OLD.export_file_id, OLD.target_host, OLD.target_directory, OLD.started_at) THEN
		RAISE EXCEPTION 'a handover keeps its file and target' USING ERRCODE = '55000';
	END IF;
	RETURN NEW;
END;
$$;
REVOKE ALL ON FUNCTION parent_billing.refuse_export_file_transfer_rewrite() FROM PUBLIC;
CREATE TRIGGER refuse_parent_billing_export_file_transfer_rewrite
	BEFORE UPDATE ON parent_billing.export_file_transfers
	FOR EACH ROW EXECUTE FUNCTION parent_billing.refuse_export_file_transfer_rewrite();

-- A handover that failed did not arrive: the bookings its DATEV file took
-- become new again, so the next handover of the run sends them (ADR 0218
-- section 6). A booking that a later DATEV file held too, one whose
-- handover did not fail (an "all bookings" download, a repeated handover),
-- moves to that file instead: it went out there, and the next standard file
-- must leave it out. Only a failure releases bookings; an unknown outcome
-- stays pending and keeps them, a transferred file keeps them for good. The
-- function runs as the table owner because the tenant role may not change
-- bookings itself, and it touches only the rows of this file of this Träger.
CREATE FUNCTION parent_billing.release_failed_handover_bookings() RETURNS trigger
LANGUAGE plpgsql SECURITY DEFINER SET search_path = pg_catalog, parent_billing AS $$
BEGIN
	IF NEW.state = 'failed' AND OLD.state = 'pending' THEN
		WITH moved AS (
			SELECT booked.id, (
				SELECT min(later.id) FROM parent_billing.export_files AS later
				JOIN parent_billing.export_file_invoices AS held
					ON held.organization_id = later.organization_id AND held.export_file_id = later.id
				WHERE later.organization_id = booked.organization_id AND later.format = 'datev' AND later.id > booked.export_file_id
					AND NOT EXISTS (SELECT 1 FROM parent_billing.export_file_transfers AS handover
						WHERE handover.organization_id = later.organization_id AND handover.export_file_id = later.id
							AND handover.state = 'failed')
					AND (held.care_invoice_id = COALESCE(booked.care_invoice_id, cancellation.invoice_id)
						OR held.document_id = booked.document_id)
			) AS file_id
			FROM parent_billing.datev_booked_documents AS booked
			LEFT JOIN parent_billing.care_invoice_cancellations AS cancellation
				ON cancellation.organization_id = booked.organization_id AND cancellation.id = booked.care_cancellation_id
			WHERE booked.organization_id = NEW.organization_id AND booked.export_file_id = NEW.export_file_id
		)
		UPDATE parent_billing.datev_booked_documents AS booked SET export_file_id = moved.file_id
		FROM moved WHERE booked.id = moved.id AND moved.file_id IS NOT NULL;
		DELETE FROM parent_billing.datev_booked_documents
		WHERE organization_id = NEW.organization_id AND export_file_id = NEW.export_file_id;
	END IF;
	RETURN NULL;
END;
$$;
REVOKE ALL ON FUNCTION parent_billing.release_failed_handover_bookings() FROM PUBLIC;
CREATE TRIGGER release_parent_billing_failed_handover_bookings
	AFTER UPDATE OF state ON parent_billing.export_file_transfers
	FOR EACH ROW EXECUTE FUNCTION parent_billing.release_failed_handover_bookings();

ALTER TABLE config.traeger_export_recipients ENABLE ROW LEVEL SECURITY;
ALTER TABLE config.traeger_export_recipients FORCE ROW LEVEL SECURITY;
CREATE POLICY traeger_isolation_traeger_export_recipients ON config.traeger_export_recipients FOR ALL
	USING (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint)
	WITH CHECK (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint);
ALTER TABLE audit.traeger_export_transfers ENABLE ROW LEVEL SECURITY;
ALTER TABLE audit.traeger_export_transfers FORCE ROW LEVEL SECURITY;
CREATE POLICY traeger_isolation_traeger_export_transfers ON audit.traeger_export_transfers FOR ALL
	USING (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint)
	WITH CHECK (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint);
ALTER TABLE parent_billing.export_file_transfers ENABLE ROW LEVEL SECURITY;
ALTER TABLE parent_billing.export_file_transfers FORCE ROW LEVEL SECURITY;
CREATE POLICY traeger_isolation_parent_billing_export_file_transfers ON parent_billing.export_file_transfers FOR ALL
	USING (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint)
	WITH CHECK (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint);

REVOKE ALL ON config.traeger_export_recipients FROM PUBLIC;
REVOKE ALL ON audit.traeger_export_transfers FROM PUBLIC;
REVOKE ALL ON parent_billing.export_file_transfers FROM PUBLIC;
REVOKE ALL ON FUNCTION audit.claim_traeger_export_transfer(BIGINT, BIGINT) FROM PUBLIC;
REVOKE ALL ON FUNCTION audit.complete_traeger_export_transfer(BIGINT, BIGINT, TEXT, BOOLEAN, TEXT) FROM PUBLIC;
GRANT SELECT, INSERT, UPDATE, DELETE ON config.traeger_export_recipients TO phoenix_tenant;
GRANT SELECT, INSERT ON audit.traeger_export_transfers TO phoenix_tenant;
GRANT USAGE ON SEQUENCE audit.traeger_export_transfers_id_seq TO phoenix_tenant;
GRANT SELECT, INSERT, UPDATE ON parent_billing.export_file_transfers TO phoenix_tenant;
GRANT EXECUTE ON FUNCTION audit.claim_traeger_export_transfer(BIGINT, BIGINT) TO phoenix_tenant;
GRANT EXECUTE ON FUNCTION audit.complete_traeger_export_transfer(BIGINT, BIGINT, TEXT, BOOLEAN, TEXT) TO phoenix_tenant;
GRANT ALL ON config.traeger_export_recipients, audit.traeger_export_transfers, parent_billing.export_file_transfers TO phoenix_admin;
GRANT ALL ON SEQUENCE audit.traeger_export_transfers_id_seq TO phoenix_admin;
`

const traegerExportTransferDownSQL = `
DROP TABLE IF EXISTS parent_billing.export_file_transfers;
DROP FUNCTION IF EXISTS parent_billing.release_failed_handover_bookings();
DROP FUNCTION IF EXISTS parent_billing.refuse_export_file_transfer_rewrite();
DROP FUNCTION IF EXISTS audit.complete_traeger_export_transfer(BIGINT, BIGINT, TEXT, BOOLEAN, TEXT);
DROP FUNCTION IF EXISTS audit.claim_traeger_export_transfer(BIGINT, BIGINT);
DROP TABLE IF EXISTS audit.traeger_export_transfers;
DROP TABLE IF EXISTS config.traeger_export_recipients;
`

func traegerExportTransferUp(ctx context.Context, db *bun.DB) error {
	return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, traegerExportTransferUpSQL); err != nil {
			return fmt.Errorf("create the Träger scope of export transfer: %w", err)
		}
		return nil
	})
}

func traegerExportTransferDown(ctx context.Context, db *bun.DB) error {
	return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		var active int
		if err := tx.NewRaw(`SELECT count(*) FROM audit.traeger_export_transfers WHERE dispatch_state IN ('pending', 'dispatching')`).Scan(ctx, &active); err != nil {
			return err
		}
		if active > 0 {
			return fmt.Errorf("rolling back the Träger export transfer requires settling %d unfinished attempts", active)
		}
		if _, err := tx.ExecContext(ctx, traegerExportTransferDownSQL); err != nil {
			return fmt.Errorf("drop the Träger scope of export transfer: %w", err)
		}
		return nil
	})
}
