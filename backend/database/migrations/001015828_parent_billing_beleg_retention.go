package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const parentBillingBelegRetentionVersion = "1.15.828"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     parentBillingBelegRetentionVersion,
		Description: "Belege des Trägers: Weisung, Aussetzung, Frist je Beleg, Klärungsliste und Löschprotokoll (#4720)",
		DependsOn: []string{
			parentBillingSEPADebitsVersion, // the newest migration on development and the debits the retention keeps
		},
	})
	Migrations.MustRegister(parentBillingBelegRetentionUp, parentBillingBelegRetentionDown)
}

// parentBillingBelegRetentionUpSQL adds the retention of the Belege of a
// Träger (#4720 slice 2, ADR 0184, Löschkonzept Träger-Finanzdaten):
//
//   - beleg_retention_instructions: the documented instruction of a Träger
//     per Beleg type, recorded by the moto team. A new instruction replaces
//     the active one; the replaced one stays as history.
//   - beleg_retention_suspensions: a suspension of one Beleg or a whole
//     type with reason, scope, responsible person and next review.
//   - beleg_retentions: one record per Beleg with type, event, its day, the
//     first deletable day, the instruction or the clarification with its
//     listing, review and responsible person. A record names its Beleg by
//     ID without a foreign key, because the Belege live in six tables; the
//     daily evaluation drops records whose Beleg went.
//   - beleg_deletions: the deletion log, counts per Träger, type and day,
//     no content (like enrollment.holiday_registration_deletions, ADR 0118).
//
// Only the administrative transaction of the hourly job and of the
// Operator portal reads and writes them; the request role has no privilege.
// Everything goes with the Träger.
const parentBillingBelegRetentionUpSQL = `
CREATE TABLE parent_billing.beleg_retention_instructions (
	id BIGSERIAL PRIMARY KEY,
	organization_id BIGINT NOT NULL REFERENCES platform.organizations(id) ON DELETE CASCADE,
	beleg_type VARCHAR(32) NOT NULL,
	trigger_event VARCHAR(32) NOT NULL,
	period INT NOT NULL,
	legal_basis TEXT NOT NULL,
	instructed_by TEXT NOT NULL,
	instructed_on DATE NOT NULL,
	instruction_reference TEXT NOT NULL,
	recorded_by_operator_id BIGINT REFERENCES platform.operators(id) ON DELETE SET NULL,
	recorded_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	replaced_at TIMESTAMPTZ,
	UNIQUE (organization_id, id),
	CONSTRAINT chk_beleg_retention_instructions_type CHECK (beleg_type IN
		('holiday_document', 'care_invoice', 'care_notice', 'care_contract', 'sepa_mandate', 'sepa_debit')),
	CONSTRAINT chk_beleg_retention_instructions_trigger CHECK (
		(beleg_type IN ('holiday_document', 'care_invoice', 'care_notice', 'sepa_debit') AND trigger_event = 'issued')
		OR (beleg_type = 'care_contract' AND trigger_event IN ('contract_concluded', 'contract_ended'))
		OR (beleg_type = 'sepa_mandate' AND trigger_event = 'last_collection')),
	CONSTRAINT chk_beleg_retention_instructions_period CHECK (
		(beleg_type = 'sepa_mandate' AND period BETWEEN 14 AND 360)
		OR (beleg_type <> 'sepa_mandate' AND period BETWEEN 1 AND 30)),
	CONSTRAINT chk_beleg_retention_instructions_texts CHECK (
		btrim(legal_basis) <> '' AND btrim(instructed_by) <> '' AND btrim(instruction_reference) <> ''
		AND char_length(legal_basis) <= 500 AND char_length(instructed_by) <= 500 AND char_length(instruction_reference) <= 500)
);
CREATE UNIQUE INDEX uq_beleg_retention_instructions_active
	ON parent_billing.beleg_retention_instructions (organization_id, beleg_type) WHERE replaced_at IS NULL;

CREATE TABLE parent_billing.beleg_retention_suspensions (
	id BIGSERIAL PRIMARY KEY,
	organization_id BIGINT NOT NULL REFERENCES platform.organizations(id) ON DELETE CASCADE,
	beleg_type VARCHAR(32) NOT NULL,
	beleg_id BIGINT,
	reason TEXT NOT NULL,
	scope TEXT NOT NULL,
	responsible TEXT NOT NULL,
	review_on DATE NOT NULL,
	created_by_operator_id BIGINT REFERENCES platform.operators(id) ON DELETE SET NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	lifted_at TIMESTAMPTZ,
	lifted_by_operator_id BIGINT REFERENCES platform.operators(id) ON DELETE SET NULL,
	UNIQUE (organization_id, id),
	CONSTRAINT chk_beleg_retention_suspensions_type CHECK (beleg_type IN
		('holiday_document', 'care_invoice', 'care_notice', 'care_contract', 'sepa_mandate', 'sepa_debit')),
	CONSTRAINT chk_beleg_retention_suspensions_texts CHECK (
		btrim(reason) <> '' AND btrim(scope) <> '' AND btrim(responsible) <> ''
		AND char_length(reason) <= 500 AND char_length(scope) <= 500 AND char_length(responsible) <= 500)
);
CREATE INDEX idx_beleg_retention_suspensions_active
	ON parent_billing.beleg_retention_suspensions (organization_id, beleg_type) WHERE lifted_at IS NULL;

CREATE TABLE parent_billing.beleg_retentions (
	organization_id BIGINT NOT NULL REFERENCES platform.organizations(id) ON DELETE CASCADE,
	beleg_type VARCHAR(32) NOT NULL,
	beleg_id BIGINT NOT NULL,
	state VARCHAR(16) NOT NULL,
	reason VARCHAR(32),
	trigger_event VARCHAR(32),
	trigger_date DATE,
	deletable_on DATE,
	proposed_on DATE,
	instruction_id BIGINT,
	suspension_id BIGINT,
	listed_on DATE,
	review_on DATE,
	responsible TEXT,
	evaluated_on DATE NOT NULL,
	PRIMARY KEY (organization_id, beleg_type, beleg_id),
	FOREIGN KEY (organization_id, instruction_id)
		REFERENCES parent_billing.beleg_retention_instructions (organization_id, id),
	FOREIGN KEY (organization_id, suspension_id)
		REFERENCES parent_billing.beleg_retention_suspensions (organization_id, id),
	CONSTRAINT chk_beleg_retentions_type CHECK (beleg_type IN
		('holiday_document', 'care_invoice', 'care_notice', 'care_contract', 'sepa_mandate', 'sepa_debit')),
	CONSTRAINT chk_beleg_retentions_state CHECK (
		(state = 'scheduled' AND instruction_id IS NOT NULL AND trigger_date IS NOT NULL AND deletable_on IS NOT NULL AND suspension_id IS NULL)
		OR (state = 'running' AND instruction_id IS NOT NULL AND deletable_on IS NULL AND suspension_id IS NULL)
		OR (state = 'suspended' AND suspension_id IS NOT NULL)
		OR (state = 'clarification' AND reason IN ('no_instruction', 'no_collection', 'no_trigger_date')
			AND deletable_on IS NULL AND suspension_id IS NULL
			AND listed_on IS NOT NULL AND review_on IS NOT NULL AND btrim(responsible) <> ''))
);
CREATE INDEX idx_beleg_retentions_due
	ON parent_billing.beleg_retentions (beleg_type, deletable_on) WHERE state = 'scheduled';
CREATE INDEX idx_beleg_retentions_instruction ON parent_billing.beleg_retentions (organization_id, instruction_id);
CREATE INDEX idx_beleg_retentions_suspension ON parent_billing.beleg_retentions (organization_id, suspension_id);
COMMENT ON TABLE parent_billing.beleg_retentions IS
	'Retention record per Beleg of a Träger (#4720, ADR 0184): event, deletable day, instruction or clarification; no content.';

CREATE TABLE parent_billing.beleg_deletions (
	id BIGSERIAL PRIMARY KEY,
	organization_id BIGINT NOT NULL REFERENCES platform.organizations(id) ON DELETE CASCADE,
	beleg_type VARCHAR(32) NOT NULL,
	deleted_on DATE NOT NULL,
	deleted_count INT NOT NULL,
	deleted_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	CONSTRAINT chk_beleg_deletions_type CHECK (beleg_type IN
		('holiday_document', 'care_invoice', 'care_notice', 'care_contract', 'sepa_mandate', 'sepa_debit')),
	CONSTRAINT chk_beleg_deletions_count CHECK (deleted_count > 0)
);
CREATE INDEX idx_beleg_deletions_organization ON parent_billing.beleg_deletions (organization_id, deleted_on);
CREATE INDEX idx_beleg_deletions_deleted_at ON parent_billing.beleg_deletions (deleted_at);
COMMENT ON TABLE parent_billing.beleg_deletions IS
	'Deletion log of the Belege of a Träger (#4720, ADR 0184): counts per type and day, no content; goes after three years or with the Träger.';

ALTER TABLE parent_billing.beleg_retention_instructions ENABLE ROW LEVEL SECURITY;
ALTER TABLE parent_billing.beleg_retention_instructions FORCE ROW LEVEL SECURITY;
CREATE POLICY traeger_isolation_parent_billing_beleg_retention_instructions ON parent_billing.beleg_retention_instructions FOR ALL
	USING (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint)
	WITH CHECK (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint);
ALTER TABLE parent_billing.beleg_retention_suspensions ENABLE ROW LEVEL SECURITY;
ALTER TABLE parent_billing.beleg_retention_suspensions FORCE ROW LEVEL SECURITY;
CREATE POLICY traeger_isolation_parent_billing_beleg_retention_suspensions ON parent_billing.beleg_retention_suspensions FOR ALL
	USING (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint)
	WITH CHECK (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint);
ALTER TABLE parent_billing.beleg_retentions ENABLE ROW LEVEL SECURITY;
ALTER TABLE parent_billing.beleg_retentions FORCE ROW LEVEL SECURITY;
CREATE POLICY traeger_isolation_parent_billing_beleg_retentions ON parent_billing.beleg_retentions FOR ALL
	USING (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint)
	WITH CHECK (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint);
ALTER TABLE parent_billing.beleg_deletions ENABLE ROW LEVEL SECURITY;
ALTER TABLE parent_billing.beleg_deletions FORCE ROW LEVEL SECURITY;
CREATE POLICY traeger_isolation_parent_billing_beleg_deletions ON parent_billing.beleg_deletions FOR ALL
	USING (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint)
	WITH CHECK (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint);

REVOKE ALL ON parent_billing.beleg_retention_instructions, parent_billing.beleg_retention_suspensions,
	parent_billing.beleg_retentions, parent_billing.beleg_deletions FROM PUBLIC, phoenix_tenant;
REVOKE ALL ON SEQUENCE parent_billing.beleg_retention_instructions_id_seq, parent_billing.beleg_retention_suspensions_id_seq,
	parent_billing.beleg_deletions_id_seq FROM PUBLIC, phoenix_tenant;
GRANT ALL ON parent_billing.beleg_retention_instructions, parent_billing.beleg_retention_suspensions,
	parent_billing.beleg_retentions, parent_billing.beleg_deletions TO phoenix_admin;
GRANT USAGE ON SEQUENCE parent_billing.beleg_retention_instructions_id_seq, parent_billing.beleg_retention_suspensions_id_seq,
	parent_billing.beleg_deletions_id_seq TO phoenix_admin;

-- The Belege of the SEPA tables and their bank data stay closed to every
-- role but the request role of their Träger (1.15.824, 1.15.826). The
-- retention reads only their days and deletes them through these functions,
-- which only the administrative role may call. A day later than tomorrow in
-- Berlin is refused, so no caller deletes ahead of the period; only a
-- superuser login (tests, maintenance) may pass another day.
CREATE OR REPLACE FUNCTION parent_billing.authorize_beleg_retention_day(p_today DATE)
RETURNS VOID LANGUAGE plpgsql STABLE SECURITY DEFINER SET search_path = pg_catalog AS $$
DECLARE
	invoker_is_superuser BOOLEAN;
BEGIN
	IF p_today IS NULL THEN
		RAISE EXCEPTION 'beleg retention requires a day' USING ERRCODE = '22023';
	END IF;
	SELECT rolsuper INTO invoker_is_superuser FROM pg_roles WHERE rolname = session_user;
	IF NOT COALESCE(invoker_is_superuser, FALSE) AND p_today > (clock_timestamp() AT TIME ZONE 'Europe/Berlin')::date + 1 THEN
		RAISE EXCEPTION 'beleg retention day lies in the future' USING ERRCODE = '42501';
	END IF;
END;
$$;

CREATE OR REPLACE FUNCTION parent_billing.beleg_retention_facts(p_organization_id BIGINT)
RETURNS TABLE (organization_id BIGINT, beleg_type TEXT, beleg_id BIGINT, issued DATE, signed_on DATE, revoked_on DATE, last_collection DATE)
LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog AS $$
	SELECT d.organization_id, 'holiday_document', d.id, d.issued_on, NULL::date, NULL::date, NULL::date
	FROM parent_billing.documents d
	JOIN parent_billing.runs r ON r.organization_id = d.organization_id AND r.id = d.run_id AND r.status = 'released'
	WHERE p_organization_id = 0 OR d.organization_id = p_organization_id
	UNION ALL
	SELECT i.organization_id, 'care_invoice', i.id, GREATEST(i.issued_on, c.issued_on), NULL, NULL, NULL
	FROM parent_billing.care_invoices i
	JOIN parent_billing.care_runs r ON r.organization_id = i.organization_id AND r.id = i.run_id AND r.status = 'released'
	LEFT JOIN parent_billing.care_invoice_cancellations c ON c.organization_id = i.organization_id AND c.invoice_id = i.id
	WHERE p_organization_id = 0 OR i.organization_id = p_organization_id
	UNION ALL
	SELECT n.organization_id, 'care_notice', n.id, n.issued_on, NULL, NULL, NULL
	FROM parent_billing.care_notices n
	JOIN parent_billing.care_notice_runs r ON r.organization_id = n.organization_id AND r.id = n.run_id AND r.status = 'released'
	WHERE p_organization_id = 0 OR n.organization_id = p_organization_id
	UNION ALL
	SELECT m.organization_id, 'sepa_mandate', m.id, NULL, m.signed_on, m.revoked_on,
		COALESCE((SELECT max((s.submitted_at AT TIME ZONE 'Europe/Berlin')::date) FROM parent_billing.sepa_debits s
			WHERE s.organization_id = m.organization_id AND s.mandate_id = m.id AND s.status = 'submitted'), m.last_collected_on)
	FROM parent_billing.sepa_mandates m
	WHERE p_organization_id = 0 OR m.organization_id = p_organization_id
	UNION ALL
	SELECT s.organization_id, 'sepa_debit', s.id, (s.submitted_at AT TIME ZONE 'Europe/Berlin')::date, NULL, NULL, NULL
	FROM parent_billing.sepa_debits s
	WHERE s.status = 'submitted' AND (p_organization_id = 0 OR s.organization_id = p_organization_id)
	ORDER BY 1, 2, 3
$$;

-- delete_due_belege deletes the Belege whose instructed period ended on
-- p_today (ADR 0184). Deleting a Beleg takes by cascade what hangs on it,
-- so one is deleted only when nothing it would take is still kept: a debit
-- blocks its invoice, document, notice or mandate; a cancellation or credit
-- note not yet due blocks the document it refers to; a change notice not
-- yet due blocks the notice it amends; a replacing invoice blocks the one it
-- replaces. Debits go first, mandates last, then the released runs left
-- without a Beleg. Each step drops the records of what it deleted and logs
-- the count per Träger and type, without content.
CREATE OR REPLACE FUNCTION parent_billing.delete_due_belege(p_today DATE)
RETURNS TABLE (beleg_type TEXT, deleted INT)
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path = pg_catalog AS $$
#variable_conflict use_column
DECLARE
	removed_count INT;
	removed_orgs BIGINT[];
	removed_ids BIGINT[];
BEGIN
	PERFORM parent_billing.authorize_beleg_retention_day(p_today);

	-- SEPA debits
	WITH removed AS (
		DELETE FROM parent_billing.sepa_debits s USING parent_billing.beleg_retentions r
		WHERE r.beleg_type = 'sepa_debit' AND r.state = 'scheduled' AND r.deletable_on <= p_today
			AND s.organization_id = r.organization_id AND s.id = r.beleg_id AND s.status = 'submitted'
		RETURNING s.organization_id, s.id)
	SELECT COALESCE(array_agg(removed.organization_id), '{}'), COALESCE(array_agg(removed.id), '{}') INTO removed_orgs, removed_ids FROM removed;
	PERFORM parent_billing.log_removed_belege('sepa_debit', p_today, removed_orgs, removed_ids);
	beleg_type := 'sepa_debit'; deleted := cardinality(removed_ids); RETURN NEXT;

	-- Holiday documents: a document goes with its whole chain of references.
	WITH RECURSIVE due AS (
		SELECT r.organization_id, r.beleg_id FROM parent_billing.beleg_retentions r
		WHERE r.beleg_type = 'holiday_document' AND r.state = 'scheduled' AND r.deletable_on <= p_today),
	tree AS (
		SELECT due.organization_id, due.beleg_id AS root, due.beleg_id AS id FROM due
		UNION ALL
		SELECT c.organization_id, tree.root, c.id FROM parent_billing.documents c
		JOIN tree ON c.organization_id = tree.organization_id AND c.reference_document_id = tree.id),
	blocked AS (
		SELECT DISTINCT tree.organization_id, tree.root FROM tree
		WHERE EXISTS (SELECT 1 FROM parent_billing.sepa_debits s WHERE s.organization_id = tree.organization_id AND s.document_id = tree.id)
			OR NOT EXISTS (SELECT 1 FROM due WHERE due.organization_id = tree.organization_id AND due.beleg_id = tree.id)),
	removed AS (
		DELETE FROM parent_billing.documents d USING due
		WHERE d.organization_id = due.organization_id AND d.id = due.beleg_id
			AND NOT EXISTS (SELECT 1 FROM blocked WHERE blocked.organization_id = d.organization_id AND blocked.root = d.id)
		RETURNING d.organization_id, d.id)
	SELECT COALESCE(array_agg(removed.organization_id), '{}'), COALESCE(array_agg(removed.id), '{}') INTO removed_orgs, removed_ids FROM removed;
	PERFORM parent_billing.log_removed_belege('holiday_document', p_today, removed_orgs, removed_ids);
	beleg_type := 'holiday_document'; deleted := cardinality(removed_ids); RETURN NEXT;

	-- Monthly invoices with their cancellation.
	WITH removed AS (
		DELETE FROM parent_billing.care_invoices i USING parent_billing.beleg_retentions r
		WHERE r.beleg_type = 'care_invoice' AND r.state = 'scheduled' AND r.deletable_on <= p_today
			AND i.organization_id = r.organization_id AND i.id = r.beleg_id
			AND NOT EXISTS (SELECT 1 FROM parent_billing.sepa_debits s WHERE s.organization_id = i.organization_id AND s.care_invoice_id = i.id)
			AND NOT EXISTS (SELECT 1 FROM parent_billing.care_invoices n WHERE n.organization_id = i.organization_id AND n.replaces_invoice_id = i.id)
		RETURNING i.organization_id, i.id)
	SELECT COALESCE(array_agg(removed.organization_id), '{}'), COALESCE(array_agg(removed.id), '{}') INTO removed_orgs, removed_ids FROM removed;
	PERFORM parent_billing.log_removed_belege('care_invoice', p_today, removed_orgs, removed_ids);
	beleg_type := 'care_invoice'; deleted := cardinality(removed_ids); RETURN NEXT;

	-- Jahresbescheide with their change notices.
	WITH RECURSIVE due AS (
		SELECT r.organization_id, r.beleg_id FROM parent_billing.beleg_retentions r
		WHERE r.beleg_type = 'care_notice' AND r.state = 'scheduled' AND r.deletable_on <= p_today),
	tree AS (
		SELECT due.organization_id, due.beleg_id AS root, due.beleg_id AS id FROM due
		UNION ALL
		SELECT c.organization_id, tree.root, c.id FROM parent_billing.care_notices c
		JOIN tree ON c.organization_id = tree.organization_id AND c.amends_notice_id = tree.id),
	blocked AS (
		SELECT DISTINCT tree.organization_id, tree.root FROM tree
		WHERE EXISTS (SELECT 1 FROM parent_billing.sepa_debits s WHERE s.organization_id = tree.organization_id AND s.care_notice_id = tree.id)
			OR NOT EXISTS (SELECT 1 FROM due WHERE due.organization_id = tree.organization_id AND due.beleg_id = tree.id)),
	removed AS (
		DELETE FROM parent_billing.care_notices n USING due
		WHERE n.organization_id = due.organization_id AND n.id = due.beleg_id
			AND NOT EXISTS (SELECT 1 FROM blocked WHERE blocked.organization_id = n.organization_id AND blocked.root = n.id)
		RETURNING n.organization_id, n.id)
	SELECT COALESCE(array_agg(removed.organization_id), '{}'), COALESCE(array_agg(removed.id), '{}') INTO removed_orgs, removed_ids FROM removed;
	PERFORM parent_billing.log_removed_belege('care_notice', p_today, removed_orgs, removed_ids);
	beleg_type := 'care_notice'; deleted := cardinality(removed_ids); RETURN NEXT;

	-- SEPA mandates, once no debit hangs on them.
	WITH removed AS (
		DELETE FROM parent_billing.sepa_mandates m USING parent_billing.beleg_retentions r
		WHERE r.beleg_type = 'sepa_mandate' AND r.state = 'scheduled' AND r.deletable_on <= p_today
			AND m.organization_id = r.organization_id AND m.id = r.beleg_id
			AND NOT EXISTS (SELECT 1 FROM parent_billing.sepa_debits s WHERE s.organization_id = m.organization_id AND s.mandate_id = m.id)
		RETURNING m.organization_id, m.id)
	SELECT COALESCE(array_agg(removed.organization_id), '{}'), COALESCE(array_agg(removed.id), '{}') INTO removed_orgs, removed_ids FROM removed;
	PERFORM parent_billing.log_removed_belege('sepa_mandate', p_today, removed_orgs, removed_ids);
	beleg_type := 'sepa_mandate'; deleted := cardinality(removed_ids); RETURN NEXT;

	-- Released runs whose every Beleg went, with the free text and
	-- emergency numbers of their letters. A released run always had one.
	DELETE FROM parent_billing.runs r WHERE r.status = 'released' AND r.document_count > 0
		AND NOT EXISTS (SELECT 1 FROM parent_billing.documents d WHERE d.organization_id = r.organization_id AND d.run_id = r.id);
	GET DIAGNOSTICS removed_count = ROW_COUNT;
	deleted := removed_count;
	DELETE FROM parent_billing.care_runs r WHERE r.status = 'released' AND r.document_count > 0
		AND NOT EXISTS (SELECT 1 FROM parent_billing.care_invoices i WHERE i.organization_id = r.organization_id AND i.run_id = r.id);
	GET DIAGNOSTICS removed_count = ROW_COUNT;
	deleted := deleted + removed_count;
	DELETE FROM parent_billing.care_notice_runs r WHERE r.status = 'released' AND r.notice_count > 0
		AND NOT EXISTS (SELECT 1 FROM parent_billing.care_notices n WHERE n.organization_id = r.organization_id AND n.run_id = r.id);
	GET DIAGNOSTICS removed_count = ROW_COUNT;
	beleg_type := 'runs'; deleted := deleted + removed_count; RETURN NEXT;
END;
$$;

-- log_removed_belege drops the records of the removed Belege of a type and
-- logs their count per Träger.
CREATE OR REPLACE FUNCTION parent_billing.log_removed_belege(p_type TEXT, p_today DATE, p_orgs BIGINT[], p_ids BIGINT[])
RETURNS VOID LANGUAGE sql VOLATILE SECURITY DEFINER SET search_path = pg_catalog AS $$
	DELETE FROM parent_billing.beleg_retentions r USING unnest(p_orgs, p_ids) AS b(organization_id, id)
	WHERE r.organization_id = b.organization_id AND r.beleg_type = p_type AND r.beleg_id = b.id;
	INSERT INTO parent_billing.beleg_deletions (organization_id, beleg_type, deleted_on, deleted_count)
	SELECT b.organization_id, p_type, p_today, count(*)::int FROM unnest(p_orgs) AS b(organization_id) GROUP BY b.organization_id;
$$;

-- due_retained_contracts lists the contracts whose period ended on p_today
-- and to which no mandate or debit hangs any more; Care Contract deletes
-- them, and their deletion takes nothing still kept.
CREATE OR REPLACE FUNCTION parent_billing.due_retained_contracts(p_today DATE)
RETURNS TABLE (organization_id BIGINT, contract_id BIGINT)
LANGUAGE plpgsql STABLE SECURITY DEFINER SET search_path = pg_catalog AS $$
#variable_conflict use_column
BEGIN
	PERFORM parent_billing.authorize_beleg_retention_day(p_today);
	RETURN QUERY SELECT r.organization_id, r.beleg_id FROM parent_billing.beleg_retentions r
		WHERE r.beleg_type = 'care_contract' AND r.state = 'scheduled' AND r.deletable_on <= p_today
			AND NOT EXISTS (SELECT 1 FROM parent_billing.sepa_mandates m WHERE m.organization_id = r.organization_id AND m.care_contract_id = r.beleg_id)
			AND NOT EXISTS (SELECT 1 FROM parent_billing.sepa_debits s WHERE s.organization_id = r.organization_id AND s.care_contract_id = r.beleg_id)
		ORDER BY r.organization_id, r.beleg_id;
END;
$$;

REVOKE ALL ON FUNCTION parent_billing.authorize_beleg_retention_day(DATE), parent_billing.beleg_retention_facts(BIGINT),
	parent_billing.delete_due_belege(DATE), parent_billing.log_removed_belege(TEXT, DATE, BIGINT[], BIGINT[]),
	parent_billing.due_retained_contracts(DATE) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION parent_billing.beleg_retention_facts(BIGINT), parent_billing.delete_due_belege(DATE),
	parent_billing.due_retained_contracts(DATE) TO phoenix_admin;
`

const parentBillingBelegRetentionDownSQL = `
DROP FUNCTION IF EXISTS parent_billing.due_retained_contracts(DATE);
DROP FUNCTION IF EXISTS parent_billing.delete_due_belege(DATE);
DROP FUNCTION IF EXISTS parent_billing.log_removed_belege(TEXT, DATE, BIGINT[], BIGINT[]);
DROP FUNCTION IF EXISTS parent_billing.beleg_retention_facts(BIGINT);
DROP FUNCTION IF EXISTS parent_billing.authorize_beleg_retention_day(DATE);
DROP TABLE IF EXISTS parent_billing.beleg_deletions;
DROP TABLE IF EXISTS parent_billing.beleg_retentions;
DROP TABLE IF EXISTS parent_billing.beleg_retention_suspensions;
DROP TABLE IF EXISTS parent_billing.beleg_retention_instructions;
`

func parentBillingBelegRetentionUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, parentBillingBelegRetentionUpSQL); err != nil {
		return fmt.Errorf("add beleg retention: %w", err)
	}
	return nil
}

func parentBillingBelegRetentionDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, parentBillingBelegRetentionDownSQL); err != nil {
		return fmt.Errorf("drop beleg retention: %w", err)
	}
	return nil
}
