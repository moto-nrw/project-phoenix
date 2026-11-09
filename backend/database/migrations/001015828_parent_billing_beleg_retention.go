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
//   - beleg_clarification_reviews: the reviews of an entry of the
//     clarification list with note, escalation and next review
//     (Löschkonzept section 3).
//
// Only the administrative transaction of the hourly job and of the
// Operator portal reads and writes them; the request role has no privilege.
// Everything goes with the Träger.
const parentBillingBelegRetentionUpSQL = `
-- beleg_basis_cites_statute is a heuristic that refuses a basis "the
-- statutes do not apply" naming § 147 AO, § 257 HGB or § 14b UStG in a
-- common spelling: the text folded to NFKC, ASCII letters lowercased and
-- everything but a-z and 0-9 dropped contains one of the tokens, without
-- word boundaries. It is no gate: spellings it misses (Cyrillic letters,
-- abbreviations, numbers in words) exist, so a short period rests on the
-- structured confirmation of the Träger instead (ADR 0184). It is the only
-- implementation; the application maps its refusal and folds no text.
CREATE OR REPLACE FUNCTION parent_billing.beleg_basis_cites_statute(p_basis TEXT)
RETURNS BOOLEAN LANGUAGE sql IMMUTABLE SET search_path = pg_catalog AS $$
	SELECT COALESCE(regexp_replace(
		translate(normalize(p_basis, NFKC), 'ABCDEFGHIJKLMNOPQRSTUVWXYZ', 'abcdefghijklmnopqrstuvwxyz'),
		'[^abcdefghijklmnopqrstuvwxyz0123456789]', '', 'g')
		~ '(ao|hgb|ustg|abgabenordnung|handelsgesetzbuch|umsatzsteuergesetz|147|257|14b)', FALSE)
$$;

CREATE TABLE parent_billing.beleg_retention_instructions (
	id BIGSERIAL PRIMARY KEY,
	organization_id BIGINT NOT NULL REFERENCES platform.organizations(id) ON DELETE CASCADE,
	beleg_type VARCHAR(32) NOT NULL,
	trigger_event VARCHAR(32) NOT NULL,
	period INT NOT NULL,
	basis VARCHAR(32) NOT NULL,
	legal_basis TEXT NOT NULL,
	basis_justification TEXT,
	instructed_by TEXT NOT NULL,
	instructed_on DATE NOT NULL,
	instruction_reference TEXT NOT NULL,
	statutes_not_applicable_confirmed BOOLEAN NOT NULL DEFAULT FALSE,
	confirmed_by_operator_id BIGINT REFERENCES platform.operators(id) ON DELETE SET NULL,
	confirmed_at TIMESTAMPTZ,
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
		AND char_length(legal_basis) <= 500 AND char_length(instructed_by) <= 500 AND char_length(instruction_reference) <= 500),
	-- The basis a Weisung names (Löschkonzept section 2): an invoice, credit
	-- or cancellation note and a submitted debit are booking vouchers; a
	-- contract or notice is classified as booking voucher or business letter;
	-- a mandate follows the Inkassovereinbarung. A booking voucher keeps at
	-- least eight years, a business letter six. A shorter period needs the
	-- explicit basis that the statutes do not apply with its written
	-- justification, the instructing person, the reference to the written
	-- instruction and the structured confirmation that the Träger confirmed
	-- in writing that § 147 AO, § 257 HGB and § 14b UStG do not apply, with
	-- the time it was recorded. The operator who recorded it is required on
	-- insert (trg_beleg_retention_instructions_confirmation) and may go with
	-- the operator account later. The text of the basis is no gate.
	CONSTRAINT chk_beleg_retention_instructions_basis CHECK (
		(beleg_type IN ('holiday_document', 'care_invoice', 'sepa_debit') AND basis IN ('booking_voucher', 'not_applicable'))
		OR (beleg_type IN ('care_contract', 'care_notice') AND basis IN ('booking_voucher', 'business_letter', 'not_applicable'))
		OR (beleg_type = 'sepa_mandate' AND basis = 'collection_agreement')),
	CONSTRAINT chk_beleg_retention_instructions_statutory_period CHECK (
		(basis <> 'booking_voucher' OR period >= 8) AND (basis <> 'business_letter' OR period >= 6)),
	CONSTRAINT chk_beleg_retention_instructions_not_applicable CHECK (
		(basis = 'not_applicable') = (basis_justification IS NOT NULL)
		AND (basis_justification IS NULL OR (btrim(basis_justification) <> '' AND char_length(basis_justification) <= 1000))),
	CONSTRAINT chk_beleg_retention_instructions_confirmation CHECK (
		statutes_not_applicable_confirmed = (basis = 'not_applicable')
		AND statutes_not_applicable_confirmed = (confirmed_at IS NOT NULL)
		AND (statutes_not_applicable_confirmed OR confirmed_by_operator_id IS NULL)),
	CONSTRAINT chk_beleg_retention_instructions_basis_cites_statute CHECK (
		basis <> 'not_applicable' OR NOT parent_billing.beleg_basis_cites_statute(legal_basis))
);
-- trg_beleg_retention_instructions_confirmation names the operator of a
-- confirmation when it is written; a CHECK cannot, because the operator
-- account may be deleted later (ON DELETE SET NULL). After that, the
-- recorded operator never changes: the only change allowed is the NULL the
-- foreign key writes when the operator account itself is gone.
CREATE OR REPLACE FUNCTION parent_billing.beleg_retention_confirmation_names_operator()
RETURNS trigger LANGUAGE plpgsql SECURITY DEFINER SET search_path = pg_catalog AS $$
BEGIN
	IF TG_OP = 'UPDATE' AND OLD.statutes_not_applicable_confirmed
		AND NEW.confirmed_by_operator_id IS DISTINCT FROM OLD.confirmed_by_operator_id
		AND NOT (NEW.confirmed_by_operator_id IS NULL
			AND NOT EXISTS (SELECT 1 FROM platform.operators AS operator WHERE operator.id = OLD.confirmed_by_operator_id)) THEN
		RAISE EXCEPTION 'the operator who recorded the confirmation that the statutes do not apply does not change'
			USING ERRCODE = 'check_violation', CONSTRAINT = 'chk_beleg_retention_instructions_confirmation';
	END IF;
	IF NEW.statutes_not_applicable_confirmed AND NEW.confirmed_by_operator_id IS NULL
		AND (TG_OP = 'INSERT' OR NOT OLD.statutes_not_applicable_confirmed) THEN
		RAISE EXCEPTION 'the confirmation that the statutes do not apply names the operator who recorded it'
			USING ERRCODE = 'check_violation', CONSTRAINT = 'chk_beleg_retention_instructions_confirmation';
	END IF;
	RETURN NEW;
END;
$$;
CREATE TRIGGER trg_beleg_retention_instructions_confirmation
	BEFORE INSERT OR UPDATE OF statutes_not_applicable_confirmed, confirmed_by_operator_id
	ON parent_billing.beleg_retention_instructions
	FOR EACH ROW EXECUTE FUNCTION parent_billing.beleg_retention_confirmation_names_operator();
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
	collection_trigger_on DATE,
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
COMMENT ON COLUMN parent_billing.beleg_retentions.collection_trigger_on IS
	'Mandate only: submission day of its last submitted debit, kept once known, so the period does not move when the debit goes (ADR 0184).';

CREATE TABLE parent_billing.beleg_clarification_reviews (
	id BIGSERIAL PRIMARY KEY,
	organization_id BIGINT NOT NULL REFERENCES platform.organizations(id) ON DELETE CASCADE,
	beleg_type VARCHAR(32) NOT NULL,
	reason VARCHAR(32) NOT NULL,
	note TEXT NOT NULL,
	escalated BOOLEAN NOT NULL,
	escalation_reason TEXT,
	reviewed_on DATE NOT NULL,
	next_review_on DATE NOT NULL,
	reviewed_by_operator_id BIGINT REFERENCES platform.operators(id) ON DELETE SET NULL,
	reviewed_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	CONSTRAINT chk_beleg_clarification_reviews_type CHECK (beleg_type IN
		('holiday_document', 'care_invoice', 'care_notice', 'care_contract', 'sepa_mandate', 'sepa_debit')),
	CONSTRAINT chk_beleg_clarification_reviews_reason CHECK (reason IN ('no_instruction', 'no_collection', 'no_trigger_date')),
	CONSTRAINT chk_beleg_clarification_reviews_note CHECK (btrim(note) <> '' AND char_length(note) <= 1000),
	CONSTRAINT chk_beleg_clarification_reviews_escalation CHECK (
		escalated = (escalation_reason IS NOT NULL)
		AND (escalation_reason IS NULL OR (btrim(escalation_reason) <> '' AND char_length(escalation_reason) <= 1000))),
	CONSTRAINT chk_beleg_clarification_reviews_next CHECK (next_review_on > reviewed_on)
);
CREATE INDEX idx_beleg_clarification_reviews_entry
	ON parent_billing.beleg_clarification_reviews (organization_id, beleg_type, reason, reviewed_at DESC, id DESC);
COMMENT ON TABLE parent_billing.beleg_clarification_reviews IS
	'Review of an entry of the clarification list (#4720, Löschkonzept section 3): note, escalation to Florian with its reason, next review; history stays, goes with the Träger.';

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

ALTER TABLE parent_billing.beleg_clarification_reviews ENABLE ROW LEVEL SECURITY;
ALTER TABLE parent_billing.beleg_clarification_reviews FORCE ROW LEVEL SECURITY;
CREATE POLICY traeger_isolation_parent_billing_beleg_clarification_reviews ON parent_billing.beleg_clarification_reviews FOR ALL
	USING (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint)
	WITH CHECK (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint);

REVOKE ALL ON parent_billing.beleg_retention_instructions, parent_billing.beleg_retention_suspensions,
	parent_billing.beleg_retentions, parent_billing.beleg_deletions, parent_billing.beleg_clarification_reviews FROM PUBLIC, phoenix_tenant;
REVOKE ALL ON SEQUENCE parent_billing.beleg_retention_instructions_id_seq, parent_billing.beleg_retention_suspensions_id_seq,
	parent_billing.beleg_deletions_id_seq, parent_billing.beleg_clarification_reviews_id_seq FROM PUBLIC, phoenix_tenant;
GRANT ALL ON parent_billing.beleg_retention_instructions, parent_billing.beleg_retention_suspensions,
	parent_billing.beleg_retentions, parent_billing.beleg_deletions, parent_billing.beleg_clarification_reviews TO phoenix_admin;
GRANT USAGE ON SEQUENCE parent_billing.beleg_retention_instructions_id_seq, parent_billing.beleg_retention_suspensions_id_seq,
	parent_billing.beleg_deletions_id_seq, parent_billing.beleg_clarification_reviews_id_seq TO phoenix_admin;

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

` + parentBillingBelegRetentionFunctionsSQL + `
`

const parentBillingBelegRetentionDownSQL = `
DROP FUNCTION IF EXISTS parent_billing.due_retained_contracts(DATE);
DROP FUNCTION IF EXISTS parent_billing.delete_due_belege(DATE);
DROP FUNCTION IF EXISTS parent_billing.log_removed_belege(TEXT, DATE, BIGINT[], BIGINT[]);
DROP FUNCTION IF EXISTS parent_billing.beleg_retention_facts(BIGINT);
DROP FUNCTION IF EXISTS parent_billing.beleg_retention_ended(BIGINT, TEXT, BIGINT, DATE, DATE);
DROP FUNCTION IF EXISTS parent_billing.beleg_add_months(DATE, INT);
DROP FUNCTION IF EXISTS parent_billing.lock_beleg_retention(BIGINT[]);
DROP FUNCTION IF EXISTS parent_billing.authorize_beleg_retention_day(DATE);
DROP TABLE IF EXISTS parent_billing.beleg_clarification_reviews;
DROP TABLE IF EXISTS parent_billing.beleg_deletions;
DROP TABLE IF EXISTS parent_billing.beleg_retentions;
DROP TABLE IF EXISTS parent_billing.beleg_retention_suspensions;
DROP TABLE IF EXISTS parent_billing.beleg_retention_instructions;
DROP FUNCTION IF EXISTS parent_billing.beleg_retention_confirmation_names_operator();
DROP FUNCTION IF EXISTS parent_billing.beleg_basis_cites_statute(TEXT);
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

// parentBillingBelegRetentionFunctionsSQL reads the facts of the Belege,
// serializes the retention per Träger and deletes the due Belege.
//
// Every deletion decides on what holds when it deletes, not on the record
// of the day's evaluation: it takes the lock of the Träger that the
// operator's Weisungen and suspensions take, locks the rows it would delete
// so no cancellation, credit note, change notice, replacing invoice or
// debit can be added to them meanwhile, and then checks each row it would
// remove, cascaded Belege included, against its own current Weisung, its
// suspensions and the event days as they stand. It never waits on billing:
// row locks are taken with SKIP LOCKED in billing's order and a busy row
// waits for the next run.
const parentBillingBelegRetentionFunctionsSQL = `
-- lock_beleg_retention serializes the retention of the given Träger: the
-- operator's Weisungen, suspensions and reviews, the evaluation and the
-- deletion all take it, in ascending order, until the transaction ends.
-- Billing never takes it, so it waits for an operator's change whatever
-- lock_timeout its caller set.
CREATE OR REPLACE FUNCTION parent_billing.lock_beleg_retention(p_organization_ids BIGINT[])
RETURNS VOID LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path = pg_catalog SET lock_timeout = 0 AS $$
DECLARE
	traeger BIGINT;
BEGIN
	FOR traeger IN SELECT DISTINCT t FROM unnest(p_organization_ids) AS t WHERE t IS NOT NULL ORDER BY t LOOP
		PERFORM pg_advisory_xact_lock(hashtext('parent_billing.beleg_retention:' || traeger::text));
	END LOOP;
END;
$$;

-- beleg_add_months adds months to a day like the evaluation does: a day the
-- target month lacks rolls into the next month, never back.
CREATE OR REPLACE FUNCTION parent_billing.beleg_add_months(p_day DATE, p_months INT)
RETURNS DATE LANGUAGE sql IMMUTABLE SET search_path = pg_catalog AS $$
	SELECT (date_trunc('month', p_day) + make_interval(months => p_months))::date + (EXTRACT(DAY FROM p_day)::int - 1)
$$;

-- beleg_retention_ended reports whether a Beleg may go on p_today as things
-- stand: its type has a current Weisung, no active suspension covers it and
-- the period of that Weisung from p_event ended. A Beleg without an event
-- day never ended.
CREATE OR REPLACE FUNCTION parent_billing.beleg_retention_ended(p_organization_id BIGINT, p_type TEXT, p_id BIGINT, p_event DATE, p_today DATE)
RETURNS BOOLEAN LANGUAGE sql STABLE SET search_path = pg_catalog AS $$
	SELECT p_event IS NOT NULL
		AND NOT EXISTS (SELECT 1 FROM parent_billing.beleg_retention_suspensions s
			WHERE s.organization_id = p_organization_id AND s.beleg_type = p_type AND s.lifted_at IS NULL
				AND (s.beleg_id IS NULL OR s.beleg_id = p_id))
		AND EXISTS (SELECT 1 FROM parent_billing.beleg_retention_instructions i
			WHERE i.organization_id = p_organization_id AND i.beleg_type = p_type AND i.replaced_at IS NULL
				AND CASE WHEN p_type = 'sepa_mandate' THEN parent_billing.beleg_add_months(p_event, i.period)
					ELSE make_date(EXTRACT(YEAR FROM p_event)::int + i.period + 1, 1, 1) END <= p_today)
$$;

-- beleg_retention_facts lists the Belege with the days of their events. The
-- trigger of a mandate is the submission of its last submitted debit; once
-- known it stays on the record (collection_trigger_on), so it does not move
-- when the debit goes. Returned or rejected debits are not recorded, so the
-- last submitted one counts, which never shortens the period. Only a
-- mandate without any recorded debit falls back to its last collection day.
CREATE OR REPLACE FUNCTION parent_billing.beleg_retention_facts(p_organization_id BIGINT)
RETURNS TABLE (organization_id BIGINT, beleg_type TEXT, beleg_id BIGINT, issued DATE, signed_on DATE, revoked_on DATE, last_collection DATE, collection_trigger DATE)
LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog AS $$
	SELECT d.organization_id, 'holiday_document', d.id, d.issued_on, NULL::date, NULL::date, NULL::date, NULL::date
	FROM parent_billing.documents d
	JOIN parent_billing.runs r ON r.organization_id = d.organization_id AND r.id = d.run_id AND r.status = 'released'
	WHERE p_organization_id = 0 OR d.organization_id = p_organization_id
	UNION ALL
	SELECT i.organization_id, 'care_invoice', i.id, GREATEST(i.issued_on, c.issued_on), NULL, NULL, NULL, NULL
	FROM parent_billing.care_invoices i
	JOIN parent_billing.care_runs r ON r.organization_id = i.organization_id AND r.id = i.run_id AND r.status = 'released'
	LEFT JOIN parent_billing.care_invoice_cancellations c ON c.organization_id = i.organization_id AND c.invoice_id = i.id
	WHERE p_organization_id = 0 OR i.organization_id = p_organization_id
	UNION ALL
	SELECT n.organization_id, 'care_notice', n.id, n.issued_on, NULL, NULL, NULL, NULL
	FROM parent_billing.care_notices n
	JOIN parent_billing.care_notice_runs r ON r.organization_id = n.organization_id AND r.id = n.run_id AND r.status = 'released'
	WHERE p_organization_id = 0 OR n.organization_id = p_organization_id
	UNION ALL
	SELECT m.organization_id, 'sepa_mandate', m.id, NULL, m.signed_on, m.revoked_on,
		COALESCE(t.trigger_on, m.last_collected_on), t.trigger_on
	FROM parent_billing.sepa_mandates m
	CROSS JOIN LATERAL (SELECT GREATEST(
		(SELECT r.collection_trigger_on FROM parent_billing.beleg_retentions r
			WHERE r.organization_id = m.organization_id AND r.beleg_type = 'sepa_mandate' AND r.beleg_id = m.id),
		(SELECT max((s.submitted_at AT TIME ZONE 'Europe/Berlin')::date) FROM parent_billing.sepa_debits s
			WHERE s.organization_id = m.organization_id AND s.mandate_id = m.id AND s.status = 'submitted')) AS trigger_on) t
	WHERE p_organization_id = 0 OR m.organization_id = p_organization_id
	UNION ALL
	SELECT s.organization_id, 'sepa_debit', s.id, (s.submitted_at AT TIME ZONE 'Europe/Berlin')::date, NULL, NULL, NULL, NULL
	FROM parent_billing.sepa_debits s
	WHERE s.status = 'submitted' AND (p_organization_id = 0 OR s.organization_id = p_organization_id)
	ORDER BY 1, 2, 3
$$;

-- delete_due_belege deletes the Belege whose instructed period ended on
-- p_today (ADR 0184). It is a background job and never makes billing wait
-- on it or fail: it takes the lock of the Träger, then locks every row it
-- would delete with SKIP LOCKED in the order billing locks them (run, then
-- its Beleg; mandate, then its debit), so a row billing holds is skipped to
-- the next run instead of waited for. A lock the cascade of a step still
-- has to wait for gives up after lock_timeout and skips that step. It then
-- deletes a row only when, as things stand, its own Weisung period ended
-- without a suspension and its record names it due; a Beleg its cascade
-- would take (a cancellation, credit note or change notice, a submitted
-- debit, a replacing invoice) must pass the same check or keeps it. A debit
-- that was never submitted is no Beleg of its own and goes with its Beleg.
-- Submitted debits go first, after their mandate recorded the trigger they
-- set; mandates go last, then the released runs left without a Beleg. Each
-- step drops the records of what it deleted and logs the count per Träger
-- and type.
CREATE OR REPLACE FUNCTION parent_billing.delete_due_belege(p_today DATE)
RETURNS TABLE (beleg_type TEXT, deleted INT)
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path = pg_catalog SET lock_timeout = '100ms' AS $$
#variable_conflict use_column
DECLARE
	removed_count INT;
	removed_orgs BIGINT[];
	removed_ids BIGINT[];
	document_tree BIGINT[];
	notice_tree BIGINT[];
	held_runs BIGINT[];
	held_care_runs BIGINT[];
	held_notice_runs BIGINT[];
	held_documents BIGINT[];
	held_invoices BIGINT[];
	held_notices BIGINT[];
	held_mandates BIGINT[];
	held_due_mandates BIGINT[];
	held_debits BIGINT[];
BEGIN
	PERFORM parent_billing.authorize_beleg_retention_day(p_today);
	PERFORM parent_billing.lock_beleg_retention(ARRAY(
		SELECT DISTINCT r.organization_id FROM parent_billing.beleg_retentions r
		WHERE r.state = 'scheduled' AND r.deletable_on <= p_today));

	-- A holiday document or Jahresbescheid goes with its whole chain of
	-- references, so every member of the chain is locked and must be due.
	WITH RECURSIVE tree AS (
		SELECT r.organization_id, r.beleg_id AS id FROM parent_billing.beleg_retentions r
		WHERE r.beleg_type = 'holiday_document' AND r.state = 'scheduled' AND r.deletable_on <= p_today
		UNION
		SELECT c.organization_id, c.id FROM parent_billing.documents c
		JOIN tree ON c.organization_id = tree.organization_id AND c.reference_document_id = tree.id)
	SELECT COALESCE(array_agg(DISTINCT tree.id), '{}') INTO document_tree FROM tree;
	WITH RECURSIVE tree AS (
		SELECT r.organization_id, r.beleg_id AS id FROM parent_billing.beleg_retentions r
		WHERE r.beleg_type = 'care_notice' AND r.state = 'scheduled' AND r.deletable_on <= p_today
		UNION
		SELECT c.organization_id, c.id FROM parent_billing.care_notices c
		JOIN tree ON c.organization_id = tree.organization_id AND c.amends_notice_id = tree.id)
	SELECT COALESCE(array_agg(DISTINCT tree.id), '{}') INTO notice_tree FROM tree;

	-- Runs first, like a cancellation, credit note or change notice takes
	-- its run before its Beleg: a run billing holds keeps its Belege.
	SELECT COALESCE(array_agg(l.id), '{}') INTO held_runs FROM (
		SELECT run.id FROM parent_billing.runs run
		WHERE run.id IN (SELECT d.run_id FROM parent_billing.documents d WHERE d.id = ANY(document_tree))
		ORDER BY run.id FOR KEY SHARE SKIP LOCKED) l;
	SELECT COALESCE(array_agg(l.id), '{}') INTO held_care_runs FROM (
		SELECT run.id FROM parent_billing.care_runs run
		WHERE run.id IN (SELECT i.run_id FROM parent_billing.care_invoices i JOIN parent_billing.beleg_retentions r
			ON r.organization_id = i.organization_id AND r.beleg_type = 'care_invoice' AND r.beleg_id = i.id
			WHERE r.state = 'scheduled' AND r.deletable_on <= p_today)
		ORDER BY run.id FOR KEY SHARE SKIP LOCKED) l;
	SELECT COALESCE(array_agg(l.id), '{}') INTO held_notice_runs FROM (
		SELECT run.id FROM parent_billing.care_notice_runs run
		WHERE run.id IN (SELECT n.run_id FROM parent_billing.care_notices n WHERE n.id = ANY(notice_tree))
		ORDER BY run.id FOR KEY SHARE SKIP LOCKED) l;

	-- Then the Belege of the held runs, against new cancellations, credit
	-- notes, change notices, replacing invoices and debits.
	SELECT COALESCE(array_agg(l.id), '{}') INTO held_documents FROM (
		SELECT d.id FROM parent_billing.documents d WHERE d.id = ANY(document_tree) AND d.run_id = ANY(held_runs)
		ORDER BY d.id FOR UPDATE SKIP LOCKED) l;
	SELECT COALESCE(array_agg(l.id), '{}') INTO held_invoices FROM (
		SELECT i.id FROM parent_billing.care_invoices i JOIN parent_billing.beleg_retentions r
			ON r.organization_id = i.organization_id AND r.beleg_type = 'care_invoice' AND r.beleg_id = i.id
		WHERE r.state = 'scheduled' AND r.deletable_on <= p_today AND i.run_id = ANY(held_care_runs)
		ORDER BY i.id FOR UPDATE OF i SKIP LOCKED) l;
	SELECT COALESCE(array_agg(l.id), '{}') INTO held_notices FROM (
		SELECT n.id FROM parent_billing.care_notices n WHERE n.id = ANY(notice_tree) AND n.run_id = ANY(held_notice_runs)
		ORDER BY n.id FOR UPDATE SKIP LOCKED) l;

	-- Mandates before their debits, like a SEPA confirmation: a mandate
	-- billing holds keeps its debits. The due mandates are locked for their
	-- deletion, the mandates of due or pending debits only against change.
	SELECT COALESCE(array_agg(l.id), '{}') INTO held_mandates FROM (
		SELECT m.id FROM parent_billing.sepa_mandates m
		WHERE m.id IN (
			SELECT s.mandate_id FROM parent_billing.sepa_debits s JOIN parent_billing.beleg_retentions r
				ON r.organization_id = s.organization_id AND r.beleg_type = 'sepa_debit' AND r.beleg_id = s.id
			WHERE s.status = 'submitted' AND r.state = 'scheduled' AND r.deletable_on <= p_today
			UNION
			SELECT s.mandate_id FROM parent_billing.sepa_debits s
			WHERE s.status = 'pending' AND (s.document_id = ANY(held_documents) OR s.care_invoice_id = ANY(held_invoices)
				OR s.care_notice_id = ANY(held_notices))
			UNION
			SELECT r.beleg_id FROM parent_billing.beleg_retentions r
			WHERE r.beleg_type = 'sepa_mandate' AND r.state = 'scheduled' AND r.deletable_on <= p_today)
		ORDER BY m.id FOR KEY SHARE SKIP LOCKED) l;
	SELECT COALESCE(array_agg(l.id), '{}') INTO held_due_mandates FROM (
		SELECT m.id FROM parent_billing.sepa_mandates m JOIN parent_billing.beleg_retentions r
			ON r.organization_id = m.organization_id AND r.beleg_type = 'sepa_mandate' AND r.beleg_id = m.id
		WHERE r.state = 'scheduled' AND r.deletable_on <= p_today AND m.id = ANY(held_mandates)
		ORDER BY m.id FOR UPDATE OF m SKIP LOCKED) l;
	SELECT COALESCE(array_agg(l.id), '{}') INTO held_debits FROM (
		SELECT s.id FROM parent_billing.sepa_debits s
		WHERE s.mandate_id = ANY(held_mandates) AND (
			(s.status = 'submitted' AND EXISTS (SELECT 1 FROM parent_billing.beleg_retentions r
				WHERE r.organization_id = s.organization_id AND r.beleg_type = 'sepa_debit' AND r.beleg_id = s.id
					AND r.state = 'scheduled' AND r.deletable_on <= p_today))
			OR (s.status = 'pending' AND (s.document_id = ANY(held_documents) OR s.care_invoice_id = ANY(held_invoices)
				OR s.care_notice_id = ANY(held_notices))))
		ORDER BY s.id FOR UPDATE SKIP LOCKED) l;

	-- Submitted SEPA debits. Their mandate keeps the day of their submission first.
	BEGIN
		UPDATE parent_billing.beleg_retentions mr SET collection_trigger_on = GREATEST(mr.collection_trigger_on, s.submitted_on)
		FROM (SELECT s.organization_id, s.mandate_id, max((s.submitted_at AT TIME ZONE 'Europe/Berlin')::date) AS submitted_on
			FROM parent_billing.sepa_debits s JOIN parent_billing.beleg_retentions r
				ON r.organization_id = s.organization_id AND r.beleg_type = 'sepa_debit' AND r.beleg_id = s.id
			WHERE s.status = 'submitted' AND s.id = ANY(held_debits) AND r.state = 'scheduled' AND r.deletable_on <= p_today
			GROUP BY s.organization_id, s.mandate_id) s
		WHERE mr.organization_id = s.organization_id AND mr.beleg_type = 'sepa_mandate' AND mr.beleg_id = s.mandate_id;
		WITH removed AS (
			DELETE FROM parent_billing.sepa_debits s USING parent_billing.beleg_retentions r
			WHERE r.beleg_type = 'sepa_debit' AND r.state = 'scheduled' AND r.deletable_on <= p_today
				AND s.organization_id = r.organization_id AND s.id = r.beleg_id AND s.status = 'submitted' AND s.id = ANY(held_debits)
				AND parent_billing.beleg_retention_ended(s.organization_id, 'sepa_debit', s.id, (s.submitted_at AT TIME ZONE 'Europe/Berlin')::date, p_today)
				AND EXISTS (SELECT 1 FROM parent_billing.beleg_retentions mr
					WHERE mr.organization_id = s.organization_id AND mr.beleg_type = 'sepa_mandate' AND mr.beleg_id = s.mandate_id
						AND mr.collection_trigger_on >= (s.submitted_at AT TIME ZONE 'Europe/Berlin')::date)
			RETURNING s.organization_id, s.id)
		SELECT COALESCE(array_agg(removed.organization_id), '{}'), COALESCE(array_agg(removed.id), '{}') INTO removed_orgs, removed_ids FROM removed;
		PERFORM parent_billing.log_removed_belege('sepa_debit', p_today, removed_orgs, removed_ids);
	EXCEPTION WHEN lock_not_available THEN
		removed_ids := '{}';
	END;
	beleg_type := 'sepa_debit'; deleted := cardinality(removed_ids); RETURN NEXT;

	-- Holiday documents with their chain; a pending debit goes with them.
	BEGIN
		WITH RECURSIVE due AS (
			SELECT r.organization_id, r.beleg_id FROM parent_billing.beleg_retentions r
			WHERE r.beleg_type = 'holiday_document' AND r.state = 'scheduled' AND r.deletable_on <= p_today AND r.beleg_id = ANY(held_documents)),
		tree AS (
			SELECT due.organization_id, due.beleg_id AS root, due.beleg_id AS id FROM due
			UNION
			SELECT c.organization_id, tree.root, c.id FROM parent_billing.documents c
			JOIN tree ON c.organization_id = tree.organization_id AND c.reference_document_id = tree.id),
		blocked AS (
			SELECT DISTINCT tree.organization_id, tree.root FROM tree
			JOIN parent_billing.documents d ON d.organization_id = tree.organization_id AND d.id = tree.id
			WHERE NOT (tree.id = ANY(held_documents))
				OR NOT EXISTS (SELECT 1 FROM due WHERE due.organization_id = tree.organization_id AND due.beleg_id = tree.id)
				OR NOT EXISTS (SELECT 1 FROM parent_billing.runs run WHERE run.organization_id = d.organization_id AND run.id = d.run_id AND run.status = 'released')
				OR EXISTS (SELECT 1 FROM parent_billing.sepa_debits s WHERE s.organization_id = tree.organization_id AND s.document_id = tree.id
					AND (s.status = 'submitted' OR NOT (s.id = ANY(held_debits))))
				OR NOT parent_billing.beleg_retention_ended(d.organization_id, 'holiday_document', d.id, d.issued_on, p_today)),
		removed AS (
			DELETE FROM parent_billing.documents d USING due
			WHERE d.organization_id = due.organization_id AND d.id = due.beleg_id
				AND NOT EXISTS (SELECT 1 FROM blocked WHERE blocked.organization_id = d.organization_id AND blocked.root = d.id)
			RETURNING d.organization_id, d.id)
		SELECT COALESCE(array_agg(removed.organization_id), '{}'), COALESCE(array_agg(removed.id), '{}') INTO removed_orgs, removed_ids FROM removed;
		PERFORM parent_billing.log_removed_belege('holiday_document', p_today, removed_orgs, removed_ids);
	EXCEPTION WHEN lock_not_available THEN
		removed_ids := '{}';
	END;
	beleg_type := 'holiday_document'; deleted := cardinality(removed_ids); RETURN NEXT;

	-- Monthly invoices with their cancellation: the later issue of the two
	-- counts, as it stands once the invoice is locked. A pending debit goes
	-- with its invoice.
	BEGIN
		WITH removed AS (
			DELETE FROM parent_billing.care_invoices i USING parent_billing.beleg_retentions r
			WHERE r.beleg_type = 'care_invoice' AND r.state = 'scheduled' AND r.deletable_on <= p_today
				AND i.organization_id = r.organization_id AND i.id = r.beleg_id AND i.id = ANY(held_invoices)
				AND EXISTS (SELECT 1 FROM parent_billing.care_runs run WHERE run.organization_id = i.organization_id AND run.id = i.run_id AND run.status = 'released')
				AND parent_billing.beleg_retention_ended(i.organization_id, 'care_invoice', i.id,
					GREATEST(i.issued_on, (SELECT max(c.issued_on) FROM parent_billing.care_invoice_cancellations c
						WHERE c.organization_id = i.organization_id AND c.invoice_id = i.id)), p_today)
				AND NOT EXISTS (SELECT 1 FROM parent_billing.sepa_debits s WHERE s.organization_id = i.organization_id AND s.care_invoice_id = i.id
					AND (s.status = 'submitted' OR NOT (s.id = ANY(held_debits))))
				AND NOT EXISTS (SELECT 1 FROM parent_billing.care_invoices n WHERE n.organization_id = i.organization_id AND n.replaces_invoice_id = i.id)
			RETURNING i.organization_id, i.id)
		SELECT COALESCE(array_agg(removed.organization_id), '{}'), COALESCE(array_agg(removed.id), '{}') INTO removed_orgs, removed_ids FROM removed;
		PERFORM parent_billing.log_removed_belege('care_invoice', p_today, removed_orgs, removed_ids);
	EXCEPTION WHEN lock_not_available THEN
		removed_ids := '{}';
	END;
	beleg_type := 'care_invoice'; deleted := cardinality(removed_ids); RETURN NEXT;

	-- Jahresbescheide with their change notices, like the holiday documents.
	BEGIN
		WITH RECURSIVE due AS (
			SELECT r.organization_id, r.beleg_id FROM parent_billing.beleg_retentions r
			WHERE r.beleg_type = 'care_notice' AND r.state = 'scheduled' AND r.deletable_on <= p_today AND r.beleg_id = ANY(held_notices)),
		tree AS (
			SELECT due.organization_id, due.beleg_id AS root, due.beleg_id AS id FROM due
			UNION
			SELECT c.organization_id, tree.root, c.id FROM parent_billing.care_notices c
			JOIN tree ON c.organization_id = tree.organization_id AND c.amends_notice_id = tree.id),
		blocked AS (
			SELECT DISTINCT tree.organization_id, tree.root FROM tree
			JOIN parent_billing.care_notices n ON n.organization_id = tree.organization_id AND n.id = tree.id
			WHERE NOT (tree.id = ANY(held_notices))
				OR NOT EXISTS (SELECT 1 FROM due WHERE due.organization_id = tree.organization_id AND due.beleg_id = tree.id)
				OR NOT EXISTS (SELECT 1 FROM parent_billing.care_notice_runs run WHERE run.organization_id = n.organization_id AND run.id = n.run_id AND run.status = 'released')
				OR EXISTS (SELECT 1 FROM parent_billing.sepa_debits s WHERE s.organization_id = tree.organization_id AND s.care_notice_id = tree.id
					AND (s.status = 'submitted' OR NOT (s.id = ANY(held_debits))))
				OR NOT parent_billing.beleg_retention_ended(n.organization_id, 'care_notice', n.id, n.issued_on, p_today)),
		removed AS (
			DELETE FROM parent_billing.care_notices n USING due
			WHERE n.organization_id = due.organization_id AND n.id = due.beleg_id
				AND NOT EXISTS (SELECT 1 FROM blocked WHERE blocked.organization_id = n.organization_id AND blocked.root = n.id)
			RETURNING n.organization_id, n.id)
		SELECT COALESCE(array_agg(removed.organization_id), '{}'), COALESCE(array_agg(removed.id), '{}') INTO removed_orgs, removed_ids FROM removed;
		PERFORM parent_billing.log_removed_belege('care_notice', p_today, removed_orgs, removed_ids);
	EXCEPTION WHEN lock_not_available THEN
		removed_ids := '{}';
	END;
	beleg_type := 'care_notice'; deleted := cardinality(removed_ids); RETURN NEXT;

	-- SEPA mandates, once no debit hangs on them and they expired: revoked,
	-- or 36 months without a collection since the recorded trigger.
	BEGIN
		WITH removed AS (
			DELETE FROM parent_billing.sepa_mandates m USING parent_billing.beleg_retentions r
			WHERE r.beleg_type = 'sepa_mandate' AND r.state = 'scheduled' AND r.deletable_on <= p_today
				AND m.organization_id = r.organization_id AND m.id = r.beleg_id AND m.id = ANY(held_due_mandates)
				AND NOT EXISTS (SELECT 1 FROM parent_billing.sepa_debits s WHERE s.organization_id = m.organization_id AND s.mandate_id = m.id)
				AND ((m.revoked_on IS NOT NULL AND m.revoked_on <= p_today)
					OR parent_billing.beleg_add_months(COALESCE(r.collection_trigger_on, m.last_collected_on, m.signed_on), 36) <= p_today)
				AND parent_billing.beleg_retention_ended(m.organization_id, 'sepa_mandate', m.id,
					COALESCE(r.collection_trigger_on, m.last_collected_on), p_today)
			RETURNING m.organization_id, m.id)
		SELECT COALESCE(array_agg(removed.organization_id), '{}'), COALESCE(array_agg(removed.id), '{}') INTO removed_orgs, removed_ids FROM removed;
		PERFORM parent_billing.log_removed_belege('sepa_mandate', p_today, removed_orgs, removed_ids);
	EXCEPTION WHEN lock_not_available THEN
		removed_ids := '{}';
	END;
	beleg_type := 'sepa_mandate'; deleted := cardinality(removed_ids); RETURN NEXT;

	-- Released runs whose every Beleg went, with the free text and
	-- emergency numbers of their letters. A released run always had one. A
	-- run billing holds stays for a later run.
	deleted := 0;
	BEGIN
		DELETE FROM parent_billing.runs r WHERE r.id IN (
			SELECT run.id FROM parent_billing.runs run WHERE run.status = 'released' AND run.document_count > 0
				AND NOT EXISTS (SELECT 1 FROM parent_billing.documents d WHERE d.organization_id = run.organization_id AND d.run_id = run.id)
			ORDER BY run.id FOR UPDATE SKIP LOCKED);
		GET DIAGNOSTICS removed_count = ROW_COUNT;
		deleted := deleted + removed_count;
	EXCEPTION WHEN lock_not_available THEN
		NULL;
	END;
	BEGIN
		DELETE FROM parent_billing.care_runs r WHERE r.id IN (
			SELECT run.id FROM parent_billing.care_runs run WHERE run.status = 'released' AND run.document_count > 0
				AND NOT EXISTS (SELECT 1 FROM parent_billing.care_invoices i WHERE i.organization_id = run.organization_id AND i.run_id = run.id)
			ORDER BY run.id FOR UPDATE SKIP LOCKED);
		GET DIAGNOSTICS removed_count = ROW_COUNT;
		deleted := deleted + removed_count;
	EXCEPTION WHEN lock_not_available THEN
		NULL;
	END;
	BEGIN
		DELETE FROM parent_billing.care_notice_runs r WHERE r.id IN (
			SELECT run.id FROM parent_billing.care_notice_runs run WHERE run.status = 'released' AND run.notice_count > 0
				AND NOT EXISTS (SELECT 1 FROM parent_billing.care_notices n WHERE n.organization_id = run.organization_id AND n.run_id = run.id)
			ORDER BY run.id FOR UPDATE SKIP LOCKED);
		GET DIAGNOSTICS removed_count = ROW_COUNT;
		deleted := deleted + removed_count;
	EXCEPTION WHEN lock_not_available THEN
		NULL;
	END;
	beleg_type := 'runs'; RETURN NEXT;
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

-- due_retained_contracts lists the contracts whose period ended on p_today,
-- with the last day their event may lie on under the current Weisung. It
-- locks the Träger and the contracts, so no mandate or debit can be added
-- to them before Care Contract deletes them in the same transaction, and
-- then lists only those without a mandate, a debit or a suspension. A
-- contract another transaction holds is skipped to a later run.
CREATE OR REPLACE FUNCTION parent_billing.due_retained_contracts(p_today DATE)
RETURNS TABLE (organization_id BIGINT, contract_id BIGINT, trigger_event TEXT, last_event_on DATE)
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path = pg_catalog AS $$
#variable_conflict use_column
DECLARE
	held BIGINT[];
BEGIN
	PERFORM parent_billing.authorize_beleg_retention_day(p_today);
	PERFORM parent_billing.lock_beleg_retention(ARRAY(
		SELECT DISTINCT r.organization_id FROM parent_billing.beleg_retentions r
		WHERE r.beleg_type = 'care_contract' AND r.state = 'scheduled' AND r.deletable_on <= p_today));
	SELECT COALESCE(array_agg(l.id), '{}') INTO held FROM (
		SELECT c.id FROM care_contract.contracts c JOIN parent_billing.beleg_retentions r
			ON r.organization_id = c.organization_id AND r.beleg_type = 'care_contract' AND r.beleg_id = c.id
		WHERE r.state = 'scheduled' AND r.deletable_on <= p_today
		ORDER BY c.id FOR UPDATE OF c SKIP LOCKED) l;
	RETURN QUERY SELECT r.organization_id, r.beleg_id, i.trigger_event::text,
			make_date(EXTRACT(YEAR FROM p_today)::int - i.period - 1, 12, 31)
		FROM parent_billing.beleg_retentions r
		JOIN parent_billing.beleg_retention_instructions i
			ON i.organization_id = r.organization_id AND i.beleg_type = 'care_contract' AND i.replaced_at IS NULL
		WHERE r.beleg_type = 'care_contract' AND r.state = 'scheduled' AND r.deletable_on <= p_today AND r.beleg_id = ANY(held)
			AND NOT EXISTS (SELECT 1 FROM parent_billing.beleg_retention_suspensions s
				WHERE s.organization_id = r.organization_id AND s.beleg_type = 'care_contract' AND s.lifted_at IS NULL
					AND (s.beleg_id IS NULL OR s.beleg_id = r.beleg_id))
			AND NOT EXISTS (SELECT 1 FROM parent_billing.sepa_mandates m WHERE m.organization_id = r.organization_id AND m.care_contract_id = r.beleg_id)
			AND NOT EXISTS (SELECT 1 FROM parent_billing.sepa_debits s WHERE s.organization_id = r.organization_id AND s.care_contract_id = r.beleg_id)
		ORDER BY r.organization_id, r.beleg_id;
END;
$$;

REVOKE ALL ON FUNCTION parent_billing.authorize_beleg_retention_day(DATE), parent_billing.beleg_retention_facts(BIGINT),
	parent_billing.lock_beleg_retention(BIGINT[]), parent_billing.beleg_add_months(DATE, INT),
	parent_billing.beleg_retention_ended(BIGINT, TEXT, BIGINT, DATE, DATE),
	parent_billing.delete_due_belege(DATE), parent_billing.log_removed_belege(TEXT, DATE, BIGINT[], BIGINT[]),
	parent_billing.due_retained_contracts(DATE) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION parent_billing.beleg_retention_facts(BIGINT), parent_billing.lock_beleg_retention(BIGINT[]),
	parent_billing.delete_due_belege(DATE), parent_billing.due_retained_contracts(DATE) TO phoenix_admin;
`
