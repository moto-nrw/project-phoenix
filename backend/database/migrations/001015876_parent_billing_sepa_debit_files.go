package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const parentBillingSEPADebitFilesVersion = "1.15.876"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     parentBillingSEPADebitFilesVersion,
		Description: "Elternabrechnung: heruntergeladene PAIN.008-Datei je Nachricht mit Prüfsumme und Gläubigerkonto aufbewahren (#6314)",
		DependsOn: []string{
			traegerRecordsHandoverVersion,  // the newest migration of this branch
			parentBillingSEPADebitsVersion, // the debits whose file it keeps
		},
	})
	Migrations.MustRegister(parentBillingSEPADebitFilesUp, parentBillingSEPADebitFilesDown)
}

// parentBillingSEPADebitFilesUpSQL keeps the PAIN.008 file of each message
// (#6314, ADR 0187). The download stores the exact bytes it hands out,
// sealed with the bank key and bound to the Träger and the message ID, with
// their SHA-256 and the creditor account in the file; a later download of
// the same message replaces it until the office confirms the submission.
// The confirmation marks the row submitted, or, for a message without a
// stored download, records the creditor account of the submission. From
// then on the row never changes (the bank data rekey rewrites file_sealed
// under its maintenance replication role). The Rückgabe of the Träger ships
// the stored original; a change of the Träger's default account no longer
// rewrites a submitted file. Messages submitted before have no row.
//
// Each row names its members from the download on: the debits, the notices
// of the pending installments (which have no debit row yet) and the
// mandates whose account the file holds. Storing a file locks its members
// and refuses a member that no longer exists, so a download never outlives
// a member deleted meanwhile; a deletion that comes later waits for it.
//
// An unsubmitted download is evidence of nothing: it goes as soon as one of
// its members goes, by any delete path, when the confirmation of another
// message of the run supersedes it, and 30 days after the download. A
// submitted file is the original of what the bank received: it stays while
// one of its debits or installment notices stays and goes with the last of
// them; a mandate alone never takes it.
//
// The request role stores and submits files but deletes none. Every
// deletion runs in an owner function with a pinned search path: the member
// trigger, the supersede of a confirmation (scoped to the Träger of the
// transaction) and the expiry of the hourly administrative cleanup.
const parentBillingSEPADebitFilesUpSQL = `
CREATE TABLE parent_billing.sepa_debit_files (
	id BIGSERIAL PRIMARY KEY,
	organization_id BIGINT NOT NULL REFERENCES platform.organizations(id) ON DELETE CASCADE,
	message_id VARCHAR(35) NOT NULL,
	run_kind VARCHAR(16) NOT NULL,
	run_id BIGINT NOT NULL,
	file_sealed BYTEA,
	file_sha256 CHAR(64),
	creditor_name VARCHAR(70) NOT NULL DEFAULT '',
	creditor_iban VARCHAR(34) NOT NULL DEFAULT '',
	creditor_bic VARCHAR(11) NOT NULL DEFAULT '',
	creditor_id VARCHAR(35) NOT NULL DEFAULT '',
	debit_ids BIGINT[] NOT NULL DEFAULT '{}',
	notice_ids BIGINT[] NOT NULL DEFAULT '{}',
	mandate_ids BIGINT[] NOT NULL DEFAULT '{}',
	downloaded_at TIMESTAMPTZ,
	downloaded_by_account_id BIGINT REFERENCES auth.accounts(id) ON DELETE SET NULL,
	submitted_at TIMESTAMPTZ,
	submitted_by_account_id BIGINT REFERENCES auth.accounts(id) ON DELETE SET NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	CONSTRAINT uq_sepa_debit_files_message UNIQUE (organization_id, message_id),
	CONSTRAINT chk_sepa_debit_files_run_kind CHECK (run_kind IN ('care', 'holiday', 'notice')),
	CONSTRAINT chk_sepa_debit_files_file CHECK ((file_sealed IS NULL) = (file_sha256 IS NULL)
		AND (file_sealed IS NULL) = (downloaded_at IS NULL)
		AND (file_sha256 IS NULL OR file_sha256 ~ '^[0-9a-f]{64}$')),
	CONSTRAINT chk_sepa_debit_files_kept CHECK (file_sealed IS NOT NULL OR submitted_at IS NOT NULL)
);
COMMENT ON TABLE parent_billing.sepa_debit_files IS
	'#6314, ADR 0187: the PAIN.008 file of one message as the office downloaded it (sealed), its SHA-256 and the creditor account in it; unchanged once submitted.';
COMMENT ON COLUMN parent_billing.sepa_debit_files.file_sealed IS
	'The exact bytes of the last download, sealed with the bank key and bound to the Träger and the message ID; holds IBANs and account holders.';
COMMENT ON COLUMN parent_billing.sepa_debit_files.debit_ids IS
	'The parent_billing.sepa_debits in the file; an unsubmitted download goes with the first of them, a submitted file with the last of its debits and notices.';
COMMENT ON COLUMN parent_billing.sepa_debit_files.notice_ids IS
	'The parent_billing.care_notices of the installments in the file; an unsubmitted download goes with the first of them, a submitted file with the last of its debits and notices.';
COMMENT ON COLUMN parent_billing.sepa_debit_files.mandate_ids IS
	'The parent_billing.sepa_mandates whose account the file holds; an unsubmitted download goes with the first of them, a submitted file never with a mandate alone.';
CREATE INDEX idx_sepa_debit_files_debits ON parent_billing.sepa_debit_files USING GIN (debit_ids);
CREATE INDEX idx_sepa_debit_files_notices ON parent_billing.sepa_debit_files USING GIN (notice_ids);
CREATE INDEX idx_sepa_debit_files_mandates ON parent_billing.sepa_debit_files USING GIN (mandate_ids);
CREATE INDEX idx_sepa_debit_files_run ON parent_billing.sepa_debit_files (organization_id, run_kind, run_id);

CREATE FUNCTION parent_billing.refuse_sepa_debit_file_rewrite() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
	IF NEW.organization_id IS DISTINCT FROM OLD.organization_id OR NEW.message_id IS DISTINCT FROM OLD.message_id
		OR NEW.run_kind IS DISTINCT FROM OLD.run_kind OR NEW.run_id IS DISTINCT FROM OLD.run_id THEN
		RAISE EXCEPTION 'a SEPA debit file keeps its message' USING ERRCODE = '55000';
	END IF;
	IF OLD.submitted_at IS NOT NULL AND (to_jsonb(NEW) - 'downloaded_by_account_id' - 'submitted_by_account_id')
		IS DISTINCT FROM (to_jsonb(OLD) - 'downloaded_by_account_id' - 'submitted_by_account_id') THEN
		RAISE EXCEPTION 'a submitted SEPA debit file never changes' USING ERRCODE = '55000';
	END IF;
	RETURN NEW;
END;
$$;
REVOKE ALL ON FUNCTION parent_billing.refuse_sepa_debit_file_rewrite() FROM PUBLIC;
CREATE TRIGGER refuse_parent_billing_sepa_debit_file_rewrite
	BEFORE UPDATE ON parent_billing.sepa_debit_files
	FOR EACH ROW EXECUTE FUNCTION parent_billing.refuse_sepa_debit_file_rewrite();

-- Storing a file locks its members against deletion until the transaction
-- ends and refuses one that is gone: a download whose member was deleted
-- after it read the run stores nothing (#6314). BEFORE INSERT fires before
-- the conflict check of the upsert, so the members are locked before the
-- row is, in the order a member deletion takes them.
CREATE FUNCTION parent_billing.lock_sepa_debit_file_members() RETURNS trigger
LANGUAGE plpgsql SECURITY DEFINER SET search_path = pg_catalog AS $$
BEGIN
	IF (SELECT count(*) FROM (SELECT 1 FROM parent_billing.sepa_debits
			WHERE organization_id = NEW.organization_id AND id = ANY (NEW.debit_ids) FOR KEY SHARE) AS locked)
			<> (SELECT count(DISTINCT member) FROM unnest(NEW.debit_ids) AS member)
		OR (SELECT count(*) FROM (SELECT 1 FROM parent_billing.care_notices
			WHERE organization_id = NEW.organization_id AND id = ANY (NEW.notice_ids) FOR KEY SHARE) AS locked)
			<> (SELECT count(DISTINCT member) FROM unnest(NEW.notice_ids) AS member)
		OR (SELECT count(*) FROM (SELECT 1 FROM parent_billing.sepa_mandates
			WHERE organization_id = NEW.organization_id AND id = ANY (NEW.mandate_ids) FOR KEY SHARE) AS locked)
			<> (SELECT count(DISTINCT member) FROM unnest(NEW.mandate_ids) AS member) THEN
		RAISE EXCEPTION 'a member of the SEPA debit file is gone' USING ERRCODE = '23503',
			CONSTRAINT = 'sepa_debit_files_members';
	END IF;
	RETURN NEW;
END;
$$;
REVOKE ALL ON FUNCTION parent_billing.lock_sepa_debit_file_members() FROM PUBLIC;
CREATE TRIGGER lock_parent_billing_sepa_debit_file_members
	BEFORE INSERT OR UPDATE OF debit_ids, notice_ids, mandate_ids ON parent_billing.sepa_debit_files
	FOR EACH ROW EXECUTE FUNCTION parent_billing.lock_sepa_debit_file_members();

-- A member that goes (with its contract, the demo visitor's, a deleted Beleg
-- or any other path) takes every unsubmitted download it is in, so no erased
-- family stays in a kept file. A submitted file goes only with the last of
-- its debits (by ID or by the message they were submitted with) and
-- installment notices. The file row is locked before the remaining members
-- are counted: of two transactions deleting the last two debits, the second
-- sees the first's deletion and takes the file.
CREATE FUNCTION parent_billing.drop_sepa_debit_files_with_their_members() RETURNS trigger
LANGUAGE plpgsql SECURITY DEFINER SET search_path = pg_catalog AS $$
DECLARE
	v_file RECORD;
	v_message TEXT := CASE WHEN TG_TABLE_NAME = 'sepa_debits' THEN to_jsonb(OLD) ->> 'message_id' END;
BEGIN
	FOR v_file IN SELECT f.id, f.message_id, f.debit_ids, f.notice_ids, f.submitted_at FROM parent_billing.sepa_debit_files AS f
		WHERE f.organization_id = OLD.organization_id
			AND ((TG_TABLE_NAME = 'sepa_debits' AND (f.debit_ids @> ARRAY[OLD.id] OR f.message_id = v_message))
				OR (TG_TABLE_NAME = 'care_notices' AND f.notice_ids @> ARRAY[OLD.id])
				OR (TG_TABLE_NAME = 'sepa_mandates' AND f.mandate_ids @> ARRAY[OLD.id]))
		ORDER BY f.id FOR UPDATE
	LOOP
		IF v_file.submitted_at IS NULL OR NOT (
			EXISTS (SELECT 1 FROM parent_billing.sepa_debits AS d WHERE d.organization_id = OLD.organization_id
				AND (d.id = ANY (v_file.debit_ids) OR d.message_id = v_file.message_id))
			OR EXISTS (SELECT 1 FROM parent_billing.care_notices AS n WHERE n.organization_id = OLD.organization_id
				AND n.id = ANY (v_file.notice_ids))) THEN
			DELETE FROM parent_billing.sepa_debit_files WHERE id = v_file.id;
		END IF;
	END LOOP;
	RETURN NULL;
END;
$$;
REVOKE ALL ON FUNCTION parent_billing.drop_sepa_debit_files_with_their_members() FROM PUBLIC;
CREATE TRIGGER drop_parent_billing_sepa_debit_files_with_their_debits
	AFTER DELETE ON parent_billing.sepa_debits
	FOR EACH ROW EXECUTE FUNCTION parent_billing.drop_sepa_debit_files_with_their_members();
CREATE TRIGGER drop_parent_billing_sepa_debit_files_with_their_notices
	AFTER DELETE ON parent_billing.care_notices
	FOR EACH ROW EXECUTE FUNCTION parent_billing.drop_sepa_debit_files_with_their_members();
CREATE TRIGGER drop_parent_billing_sepa_debit_files_with_their_mandates
	AFTER DELETE ON parent_billing.sepa_mandates
	FOR EACH ROW EXECUTE FUNCTION parent_billing.drop_sepa_debit_files_with_their_members();

-- The confirmation of a message drops the unsubmitted downloads of its run
-- that share a debit or an installment with it, only of the Träger of the
-- transaction.
CREATE FUNCTION parent_billing.drop_superseded_sepa_debit_files(p_organization_id BIGINT, p_run_kind TEXT, p_run_id BIGINT,
	p_message_id TEXT, p_debit_ids BIGINT[], p_notice_ids BIGINT[]) RETURNS INTEGER
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path = pg_catalog AS $$
DECLARE
	v_count INTEGER;
BEGIN
	IF p_organization_id IS NULL
		OR p_organization_id IS DISTINCT FROM NULLIF(current_setting('app.current_org_id', true), '')::BIGINT THEN
		RAISE EXCEPTION 'the SEPA debit files of another Träger are out of reach' USING ERRCODE = '42501';
	END IF;
	DELETE FROM parent_billing.sepa_debit_files
		WHERE organization_id = p_organization_id AND run_kind = p_run_kind AND run_id = p_run_id
			AND message_id <> p_message_id AND submitted_at IS NULL
			AND (debit_ids && p_debit_ids OR notice_ids && p_notice_ids);
	GET DIAGNOSTICS v_count = ROW_COUNT;
	RETURN v_count;
END;
$$;
REVOKE ALL ON FUNCTION parent_billing.drop_superseded_sepa_debit_files(BIGINT, TEXT, BIGINT, TEXT, BIGINT[], BIGINT[]) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION parent_billing.drop_superseded_sepa_debit_files(BIGINT, TEXT, BIGINT, TEXT, BIGINT[], BIGINT[]) TO phoenix_tenant;

-- The hourly administrative cleanup expires the unsubmitted downloads of
-- every Träger 30 days after the download; a submitted file never expires
-- here. A confirmation that holds the row lock wins: the deletion rechecks
-- submitted_at after the wait.
CREATE FUNCTION parent_billing.delete_expired_sepa_debit_downloads() RETURNS INTEGER
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path = pg_catalog AS $$
DECLARE
	v_count INTEGER;
BEGIN
	DELETE FROM parent_billing.sepa_debit_files
		WHERE submitted_at IS NULL AND downloaded_at < clock_timestamp() - INTERVAL '30 days';
	GET DIAGNOSTICS v_count = ROW_COUNT;
	RETURN v_count;
END;
$$;
REVOKE ALL ON FUNCTION parent_billing.delete_expired_sepa_debit_downloads() FROM PUBLIC;
GRANT EXECUTE ON FUNCTION parent_billing.delete_expired_sepa_debit_downloads() TO phoenix_admin;

ALTER TABLE parent_billing.sepa_debit_files ENABLE ROW LEVEL SECURITY;
ALTER TABLE parent_billing.sepa_debit_files FORCE ROW LEVEL SECURITY;
CREATE POLICY traeger_isolation_parent_billing_sepa_debit_files ON parent_billing.sepa_debit_files FOR ALL
	USING (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint)
	WITH CHECK (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint);
REVOKE ALL ON parent_billing.sepa_debit_files FROM PUBLIC;
-- No DELETE: a submitted original is evidence; the owner functions above
-- delete what may go.
GRANT SELECT, INSERT, UPDATE ON parent_billing.sepa_debit_files TO phoenix_tenant;
GRANT USAGE ON SEQUENCE parent_billing.sepa_debit_files_id_seq TO phoenix_tenant;
`

func parentBillingSEPADebitFilesUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, parentBillingSEPADebitFilesUpSQL); err != nil {
		return fmt.Errorf("create parent billing SEPA debit files: %w", err)
	}
	return nil
}

func parentBillingSEPADebitFilesDown(ctx context.Context, db *bun.DB) error {
	return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		// A stored submitted file is the original the Rückgabe ships; never discard it.
		var count int
		if err := tx.NewRaw(`SELECT count(*) FROM parent_billing.sepa_debit_files`).Scan(ctx, &count); err != nil {
			return err
		}
		if count != 0 {
			return fmt.Errorf("SEPA debit files rollback would discard %d stored files; use a forward migration", count)
		}
		_, err := tx.ExecContext(ctx, `DROP TRIGGER drop_parent_billing_sepa_debit_files_with_their_debits ON parent_billing.sepa_debits;
			DROP TRIGGER drop_parent_billing_sepa_debit_files_with_their_notices ON parent_billing.care_notices;
			DROP TRIGGER drop_parent_billing_sepa_debit_files_with_their_mandates ON parent_billing.sepa_mandates;
			DROP FUNCTION parent_billing.drop_sepa_debit_files_with_their_members();
			DROP FUNCTION parent_billing.drop_superseded_sepa_debit_files(BIGINT, TEXT, BIGINT, TEXT, BIGINT[], BIGINT[]);
			DROP FUNCTION parent_billing.delete_expired_sepa_debit_downloads();
			DROP TABLE parent_billing.sepa_debit_files;
			DROP FUNCTION parent_billing.lock_sepa_debit_file_members();
			DROP FUNCTION parent_billing.refuse_sepa_debit_file_rewrite()`)
		return err
	})
}
