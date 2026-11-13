package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const traegerExportRecipientsVersion = "1.15.963"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     traegerExportRecipientsVersion,
		Description: "Export Transfer mit mehreren Empfängern je Träger und einem Empfänger je Exportformat (#6911, ADR 0222)",
		DependsOn: []string{
			workTimeModelArchiveVersion,  // the newest migration on development (1.15.962)
			traegerExportTransferVersion, // the one recipient, the journal and the handovers this one extends
		},
	})
	Migrations.MustRegister(traegerExportRecipientsUp, traegerExportRecipientsDown)
}

// traegerExportRecipientsUpSQL lets a Träger keep several recipients of its
// finance exports and send each export format to exactly one of them (#6911,
// ADR 0222, extending ADR 0218).
//
//   - config.traeger_export_recipients gets an id and a name of its own; the
//     Träger is no longer its key. The existing recipient keeps its values
//     and its sealed password and is named after its address.
//   - config.traeger_export_recipient_formats holds the assignment: one row
//     per Träger and format, pointing at one recipient of that Träger. A
//     format without a row is downloaded only. Removing a recipient removes
//     its rows (ON DELETE CASCADE), so its formats go back to the download.
//     The existing recipient receives every format, as it did so far.
//   - audit.traeger_export_transfers and parent_billing.export_file_transfers
//     name the recipient of each handover and keep its name as it was then.
//     The journal loses only the reference when a recipient goes
//     (ON DELETE SET NULL (recipient_id)); the handover of the export
//     protocol keeps a plain id, Export Transfer's key, without a reference
//     across the owners. Existing rows belong to the existing recipient.
const traegerExportRecipientsUpSQL = `
ALTER TABLE config.traeger_export_recipients
	ADD COLUMN id BIGSERIAL,
	ADD COLUMN name VARCHAR(80);
UPDATE config.traeger_export_recipients
	SET name = COALESCE(NULLIF(left(btrim(regexp_replace(host, '[[:cntrl:]]', '', 'g')), 80), ''), 'Empfänger der Exporte');
ALTER TABLE config.traeger_export_recipients
	ALTER COLUMN name SET NOT NULL,
	DROP CONSTRAINT traeger_export_recipients_pkey,
	ADD PRIMARY KEY (id),
	ADD CONSTRAINT uq_traeger_export_recipients_organization_id UNIQUE (organization_id, id),
	ADD CONSTRAINT chk_traeger_export_recipients_name CHECK (
		name = btrim(name) AND char_length(name) BETWEEN 1 AND 80 AND name !~ '[[:cntrl:]]');
-- Case-folded with the built-in ICU root collation, so „KÄMMEREI“ and
-- „Kämmerei“ clash whatever the locale of the server.
CREATE UNIQUE INDEX uq_traeger_export_recipients_name ON config.traeger_export_recipients
	(organization_id, lower(name COLLATE "und-x-icu"));
COMMENT ON TABLE config.traeger_export_recipients IS
	'The recipients of the finance exports of a Träger (#6824, #6911, ADR 0218, ADR 0222): name, SFTP address, user, sealed password, directory and host key fingerprint.';

CREATE TABLE config.traeger_export_recipient_formats (
	organization_id BIGINT NOT NULL REFERENCES platform.organizations(id) ON DELETE CASCADE,
	export_format VARCHAR(32) NOT NULL,
	recipient_id BIGINT NOT NULL,
	updated_by_account_id BIGINT REFERENCES auth.accounts(id) ON DELETE SET NULL,
	updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	PRIMARY KEY (organization_id, export_format),
	CONSTRAINT fk_traeger_export_recipient_formats_recipient FOREIGN KEY (organization_id, recipient_id)
		REFERENCES config.traeger_export_recipients (organization_id, id) ON DELETE CASCADE,
	CONSTRAINT chk_traeger_export_recipient_formats_format CHECK (export_format IN ('sollstellung', 'datev', 'pain008'))
);
CREATE INDEX idx_traeger_export_recipient_formats_recipient ON config.traeger_export_recipient_formats (organization_id, recipient_id);
COMMENT ON TABLE config.traeger_export_recipient_formats IS
	'Which recipient of a Träger receives which finance export format (#6911, ADR 0222); a format without a row is downloaded only.';
INSERT INTO config.traeger_export_recipient_formats (organization_id, export_format, recipient_id, updated_by_account_id, updated_at)
SELECT recipient.organization_id, format.export_format, recipient.id, recipient.updated_by_account_id, recipient.updated_at
FROM config.traeger_export_recipients AS recipient
CROSS JOIN (VALUES ('sollstellung'), ('datev'), ('pain008')) AS format (export_format);

ALTER TABLE audit.traeger_export_transfers
	ADD COLUMN recipient_id BIGINT,
	ADD COLUMN recipient_name VARCHAR(80) NOT NULL DEFAULT '',
	ADD CONSTRAINT fk_traeger_export_transfers_recipient FOREIGN KEY (organization_id, recipient_id)
		REFERENCES config.traeger_export_recipients (organization_id, id) ON DELETE SET NULL (recipient_id);
UPDATE audit.traeger_export_transfers AS journal SET recipient_id = recipient.id, recipient_name = recipient.name
FROM config.traeger_export_recipients AS recipient WHERE recipient.organization_id = journal.organization_id;
CREATE INDEX idx_traeger_export_transfers_recipient ON audit.traeger_export_transfers (organization_id, recipient_id, created_at DESC, id DESC)
	WHERE recipient_id IS NOT NULL;

ALTER TABLE parent_billing.export_file_transfers
	ADD COLUMN recipient_id BIGINT,
	ADD COLUMN recipient_name VARCHAR(80) NOT NULL DEFAULT '';
-- Settled handovers never change; the migration names their recipient once.
ALTER TABLE parent_billing.export_file_transfers DISABLE TRIGGER refuse_parent_billing_export_file_transfer_rewrite;
UPDATE parent_billing.export_file_transfers AS handover SET recipient_id = recipient.id, recipient_name = recipient.name
FROM config.traeger_export_recipients AS recipient WHERE recipient.organization_id = handover.organization_id;
ALTER TABLE parent_billing.export_file_transfers ENABLE TRIGGER refuse_parent_billing_export_file_transfer_rewrite;

-- A handover also keeps its recipient.
CREATE OR REPLACE FUNCTION parent_billing.refuse_export_file_transfer_rewrite() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
	IF OLD.state <> 'pending' THEN
		RAISE EXCEPTION 'a settled handover never changes' USING ERRCODE = '55000';
	END IF;
	IF (NEW.organization_id, NEW.export_file_id, NEW.target_host, NEW.target_directory, NEW.started_at, NEW.recipient_id, NEW.recipient_name)
		IS DISTINCT FROM (OLD.organization_id, OLD.export_file_id, OLD.target_host, OLD.target_directory, OLD.started_at, OLD.recipient_id, OLD.recipient_name) THEN
		RAISE EXCEPTION 'a handover keeps its file, target and recipient' USING ERRCODE = '55000';
	END IF;
	RETURN NEW;
END;
$$;

ALTER TABLE config.traeger_export_recipient_formats ENABLE ROW LEVEL SECURITY;
ALTER TABLE config.traeger_export_recipient_formats FORCE ROW LEVEL SECURITY;
CREATE POLICY traeger_isolation_traeger_export_recipient_formats ON config.traeger_export_recipient_formats FOR ALL
	USING (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint)
	WITH CHECK (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint);

REVOKE ALL ON config.traeger_export_recipient_formats FROM PUBLIC;
GRANT SELECT, INSERT, UPDATE, DELETE ON config.traeger_export_recipient_formats TO phoenix_tenant;
GRANT USAGE ON SEQUENCE config.traeger_export_recipients_id_seq TO phoenix_tenant;
GRANT ALL ON config.traeger_export_recipient_formats TO phoenix_admin;
GRANT ALL ON SEQUENCE config.traeger_export_recipients_id_seq TO phoenix_admin;
`

// The way back keeps one recipient per Träger: it refuses while a Träger
// has more than one, so no recipient and no password is dropped silently.
const traegerExportRecipientsDownSQL = `
CREATE OR REPLACE FUNCTION parent_billing.refuse_export_file_transfer_rewrite() RETURNS trigger
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
ALTER TABLE parent_billing.export_file_transfers DROP COLUMN IF EXISTS recipient_name, DROP COLUMN IF EXISTS recipient_id;
DROP INDEX IF EXISTS audit.idx_traeger_export_transfers_recipient;
ALTER TABLE audit.traeger_export_transfers DROP CONSTRAINT IF EXISTS fk_traeger_export_transfers_recipient,
	DROP COLUMN IF EXISTS recipient_name, DROP COLUMN IF EXISTS recipient_id;
DROP TABLE IF EXISTS config.traeger_export_recipient_formats;
DROP INDEX IF EXISTS config.uq_traeger_export_recipients_name;
ALTER TABLE config.traeger_export_recipients
	DROP CONSTRAINT IF EXISTS chk_traeger_export_recipients_name,
	DROP CONSTRAINT IF EXISTS uq_traeger_export_recipients_organization_id,
	DROP CONSTRAINT traeger_export_recipients_pkey,
	ADD PRIMARY KEY (organization_id),
	DROP COLUMN IF EXISTS name,
	DROP COLUMN IF EXISTS id;
COMMENT ON TABLE config.traeger_export_recipients IS
	'The recipient of the finance exports of a Träger (#6824, ADR 0218): SFTP address, user, sealed password, directory and host key fingerprint.';
`

func traegerExportRecipientsUp(ctx context.Context, db *bun.DB) error {
	return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, traegerExportRecipientsUpSQL); err != nil {
			return fmt.Errorf("give the Träger several export recipients: %w", err)
		}
		return nil
	})
}

func traegerExportRecipientsDown(ctx context.Context, db *bun.DB) error {
	return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		var crowded int
		if err := tx.NewRaw(`SELECT count(*) FROM (SELECT organization_id FROM config.traeger_export_recipients
			GROUP BY organization_id HAVING count(*) > 1) AS several`).Scan(ctx, &crowded); err != nil {
			return err
		}
		if crowded > 0 {
			return fmt.Errorf("rolling back the export recipients requires removing the extra recipients of %d Träger first", crowded)
		}
		if _, err := tx.ExecContext(ctx, traegerExportRecipientsDownSQL); err != nil {
			return fmt.Errorf("return to one export recipient per Träger: %w", err)
		}
		return nil
	})
}
