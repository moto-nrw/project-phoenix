package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const parentBillingCareNoticeLettersVersion = "1.15.877"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     parentBillingCareNoticeLettersVersion,
		Description: "Elternabrechnung: gespeichertes Original, Versand und Elternpostfach der Jahresbescheide (#6347)",
		DependsOn: []string{
			demoRetirementRevokesSessionsVersion,      // the newest migration before this one
			parentBillingCareNoticesVersion,           // the notices whose originals these are
			parentBillingCareInvoiceExclusionsVersion, // the erasure this one extends
		},
	})
	Migrations.MustRegister(parentBillingCareNoticeLettersUp, parentBillingCareNoticeLettersDown)
}

// parentBillingCareNoticeLettersUpSQL adds the stored originals of the
// Jahresbescheide and their sends (#6347, decision A):
//
//   - care_notice_letters: the PDF of one issued notice, rendered once on
//     the letterhead of that moment. Its ID comes from the sequence of the
//     holiday documents like the monthly letters (1.15.809), so the inbox of
//     the parents names every letter by one ID without a clash. One per
//     notice; it never changes and goes with its notice.
//   - care_notice_letter_mailings: every send with the outbox entry and the
//     address; a notice is in the parents inbox once it was sent. It never
//     changes; deleting the account that asked for it clears the account.
//
// The trail of a notice run records creating the originals (letters) and
// every mail (mail). Both tables carry the Träger and are isolated by it
// (RLS); the request role reads and appends only.
const parentBillingCareNoticeLettersUpSQL = `
CREATE TABLE parent_billing.care_notice_letters (
	id BIGINT PRIMARY KEY DEFAULT nextval('parent_billing.documents_id_seq'),
	organization_id BIGINT NOT NULL,
	run_id BIGINT NOT NULL,
	notice_id BIGINT NOT NULL,
	filename TEXT NOT NULL,
	pdf BYTEA NOT NULL,
	sha256 CHAR(64) GENERATED ALWAYS AS (encode(sha256(pdf), 'hex')) STORED,
	created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	CONSTRAINT uq_care_notice_letters_organization UNIQUE (organization_id, id),
	CONSTRAINT uq_care_notice_letters_notice UNIQUE (organization_id, notice_id),
	CONSTRAINT fk_care_notice_letters_notice FOREIGN KEY (organization_id, notice_id)
		REFERENCES parent_billing.care_notices (organization_id, id) ON DELETE CASCADE,
	CONSTRAINT fk_care_notice_letters_run FOREIGN KEY (organization_id, run_id)
		REFERENCES parent_billing.care_notice_runs (organization_id, id) ON DELETE CASCADE,
	CONSTRAINT chk_care_notice_letters_pdf CHECK (octet_length(pdf) > 0 AND BTRIM(filename) <> '')
);
CREATE INDEX idx_care_notice_letters_run ON parent_billing.care_notice_letters (organization_id, run_id);
COMMENT ON TABLE parent_billing.care_notice_letters IS
	'Stored original (PDF) of an issued Jahresbescheid in the layout of the Träger (#6347); rendered once, never changes, goes with its notice.';

CREATE TABLE parent_billing.care_notice_letter_mailings (
	id BIGSERIAL PRIMARY KEY,
	organization_id BIGINT NOT NULL,
	run_id BIGINT NOT NULL,
	letter_id BIGINT NOT NULL,
	delivery_id BIGINT NOT NULL,
	recipient_email TEXT NOT NULL,
	requested_by_account_id BIGINT REFERENCES auth.accounts(id) ON DELETE SET NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	CONSTRAINT uq_care_notice_letter_mailings_delivery UNIQUE (organization_id, delivery_id),
	CONSTRAINT fk_care_notice_letter_mailings_letter FOREIGN KEY (organization_id, letter_id)
		REFERENCES parent_billing.care_notice_letters (organization_id, id) ON DELETE CASCADE,
	CONSTRAINT fk_care_notice_letter_mailings_run FOREIGN KEY (organization_id, run_id)
		REFERENCES parent_billing.care_notice_runs (organization_id, id) ON DELETE CASCADE,
	CONSTRAINT chk_care_notice_letter_mailings_recipient CHECK (BTRIM(recipient_email) <> '' AND delivery_id > 0)
);
CREATE INDEX idx_care_notice_letter_mailings_letter
	ON parent_billing.care_notice_letter_mailings (organization_id, letter_id, id);
CREATE INDEX idx_care_notice_letter_mailings_run
	ON parent_billing.care_notice_letter_mailings (organization_id, run_id);
CREATE INDEX idx_care_notices_recipient
	ON parent_billing.care_notices (organization_id, LOWER(recipient_email));
COMMENT ON TABLE parent_billing.care_notice_letter_mailings IS
	'Every send of the original of a Jahresbescheid with its outbox entry and address (#6347); the notice is in the parents inbox once sent; no formal delivery.';

CREATE TRIGGER refuse_parent_billing_care_notice_letter_change
	BEFORE UPDATE ON parent_billing.care_notice_letters
	FOR EACH ROW EXECUTE FUNCTION parent_billing.refuse_issued_change();
CREATE TRIGGER refuse_parent_billing_care_notice_letter_mailing_change
	BEFORE UPDATE ON parent_billing.care_notice_letter_mailings
	FOR EACH ROW EXECUTE FUNCTION parent_billing.refuse_care_letter_mailing_change();

ALTER TABLE parent_billing.care_notice_run_events DROP CONSTRAINT chk_care_notice_run_events_action;
ALTER TABLE parent_billing.care_notice_run_events ADD CONSTRAINT chk_care_notice_run_events_action
	CHECK (action IN ('create', 'read', 'release', 'letter', 'letters', 'mail'));

ALTER TABLE parent_billing.care_notice_letters ENABLE ROW LEVEL SECURITY;
ALTER TABLE parent_billing.care_notice_letters FORCE ROW LEVEL SECURITY;
CREATE POLICY traeger_isolation_parent_billing_care_notice_letters ON parent_billing.care_notice_letters FOR ALL
	USING (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint)
	WITH CHECK (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint);
ALTER TABLE parent_billing.care_notice_letter_mailings ENABLE ROW LEVEL SECURITY;
ALTER TABLE parent_billing.care_notice_letter_mailings FORCE ROW LEVEL SECURITY;
CREATE POLICY traeger_isolation_parent_billing_care_notice_letter_mailings ON parent_billing.care_notice_letter_mailings FOR ALL
	USING (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint)
	WITH CHECK (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint);

REVOKE ALL ON parent_billing.care_notice_letters, parent_billing.care_notice_letter_mailings FROM PUBLIC;
GRANT SELECT, INSERT ON parent_billing.care_notice_letters, parent_billing.care_notice_letter_mailings TO phoenix_tenant;
GRANT USAGE ON SEQUENCE parent_billing.care_notice_letter_mailings_id_seq, parent_billing.documents_id_seq TO phoenix_tenant;
GRANT ALL ON parent_billing.care_notice_letters, parent_billing.care_notice_letter_mailings TO phoenix_admin;
GRANT ALL ON SEQUENCE parent_billing.care_notice_letter_mailings_id_seq TO phoenix_admin;
`

const parentBillingCareNoticeLettersDownSQL = `
DROP TABLE IF EXISTS parent_billing.care_notice_letter_mailings, parent_billing.care_notice_letters;
DROP INDEX IF EXISTS parent_billing.idx_care_notices_recipient;
DELETE FROM parent_billing.care_notice_run_events WHERE action IN ('letters', 'mail');
ALTER TABLE parent_billing.care_notice_run_events DROP CONSTRAINT chk_care_notice_run_events_action;
ALTER TABLE parent_billing.care_notice_run_events ADD CONSTRAINT chk_care_notice_run_events_action
	CHECK (action IN ('create', 'read', 'release', 'letter'));
`

// careNoticeLetterErasureAnchor is where 1.15.864 deletes the notices of a
// retired demo school; the originals and their sends still exist there.
const careNoticeLetterErasureAnchor = `					-- A Jahresbescheid of the school copies child, recipient and
					-- mail address (#6219); it goes with the school, its months
					-- by cascade, whether or not it names anybody.
					DELETE FROM parent_billing.care_notices
					WHERE school_id = ANY (hidden_schools);
`

// careNoticeLetterErasureStatement is what 1.15.877 adds before it. The
// original and its sends go with the notice by cascade, but the mail on
// Delivery's outbox carries the PDF with child, recipient and address, its
// delivery result names the address, and the push names the letter and the
// address; those copies name the notice only through the send. A send holds
// the notice run: take the runs first, so a send under way is seen before
// its copies are cleaned up.
const careNoticeLetterErasureStatement = `					-- The original of a Jahresbescheid travels on Delivery's
					-- outbox with the PDF and the address, and a push announces
					-- it in the parents inbox (#6347). A send holds the notice
					-- run: take the runs first.
					PERFORM 1 FROM parent_billing.care_notice_runs AS run
					WHERE (run.organization_id, run.id) IN (
						SELECT notice.organization_id, notice.run_id
						FROM parent_billing.care_notices AS notice
						WHERE notice.school_id = ANY (hidden_schools))
					ORDER BY run.organization_id, run.id
					FOR UPDATE;
					DELETE FROM platform.email_delivery AS delivery
					USING parent_billing.care_notice_letter_mailings AS mailing, parent_billing.care_notice_letters AS letter,
						parent_billing.care_notices AS notice
					WHERE notice.school_id = ANY (hidden_schools)
						AND letter.organization_id = notice.organization_id
						AND letter.notice_id = notice.id
						AND mailing.organization_id = letter.organization_id
						AND mailing.letter_id = letter.id
						AND delivery.outbox_id = mailing.delivery_id;
					DELETE FROM platform.email_outbox AS outbox
					USING parent_billing.care_notice_letter_mailings AS mailing, parent_billing.care_notice_letters AS letter,
						parent_billing.care_notices AS notice
					WHERE notice.school_id = ANY (hidden_schools)
						AND letter.organization_id = notice.organization_id
						AND letter.notice_id = notice.id
						AND mailing.organization_id = letter.organization_id
						AND mailing.letter_id = letter.id
						AND outbox.organization_id = mailing.organization_id
						AND outbox.kind = 'parent_billing_letter'
						AND outbox.id = mailing.delivery_id;
					DELETE FROM platform.push_outbox AS push
					USING parent_billing.care_notice_letters AS letter, parent_billing.care_notices AS notice
					WHERE notice.school_id = ANY (hidden_schools)
						AND letter.organization_id = notice.organization_id
						AND letter.notice_id = notice.id
						AND push.organization_id = letter.organization_id
						AND push.kind = 'parent_billing_document'
						AND push.related_entity_type = 'parent_billing_document'
						AND push.related_entity_id = letter.id;
`

func parentBillingCareNoticeLettersUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, parentBillingCareNoticeLettersUpSQL); err != nil {
		return fmt.Errorf("create the notice letters of parent billing: %w", err)
	}
	return installCareNoticeLetterErasure(ctx, db)
}

func parentBillingCareNoticeLettersDown(ctx context.Context, db *bun.DB) error {
	if err := installCareInvoiceExclusionErasure(ctx, db); err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, parentBillingCareNoticeLettersDownSQL); err != nil {
		return fmt.Errorf("drop the notice letters of parent billing: %w", err)
	}
	return nil
}
