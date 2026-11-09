package migrations

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
)

const demoVisitorErasureOutboxKindsVersion = "1.15.843"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     demoVisitorErasureOutboxKindsVersion,
		Description: "Demo-Besuch löschen: jede Art der Versand-Warteschlange erfassen, Träger-Arten im Schema festhalten",
		DependsOn: []string{
			cleanupRunFailuresVersion,         // the newest migration before this one
			parentBillingCareNoticesVersion,   // the erasure of 1.15.823 this one extends
			traegerSharedDocumentsVersion,     // the releases whose pushes go
			holidayRegistrationsVersion,       // the mails of a Träger and the receipts that go
			parentBillingDocumentInboxVersion, // the pushes of a Träger
		},
		Precondition: demoVisitorErasureOutboxKindsPrecondition,
	})
	Migrations.MustRegister(demoVisitorErasureOutboxKindsUp, demoVisitorErasureOutboxKindsDown)
}

// A row of the Träger in Delivery's outbox (organization_id set, ADR 0110,
// ADR 0126) outlives the school it is about, so the demo erasure has to name
// it by its kind. The constraints are the list of those kinds: a producer
// cannot enqueue a kind of the Träger that is not in it, and the guard of
// #6342 (TestDemoErasureCoversEveryOutboxKind) reads the list from the
// schema and fails for a kind no statement of the erasure names. A new kind
// of the Träger is therefore a migration that adds it here and to the
// erasure, or to demoErasureOutboxKindExceptions with its reason.

// The kinds a row of a Träger may have, per outbox table. The preflight
// takes them as its parameter; TestOutboxKindsOfATraegerAreDeclared keeps
// them equal to the CHECKs below.
var (
	emailOutboxTraegerKinds = []string{"care_contract", "care_contract_notice", "holiday_registration_received", "parent_billing_letter"}
	pushOutboxTraegerKinds  = []string{"care_contract_document", "parent_billing_document", "traeger_document_release", "traeger_document_release_account"}
)

// Each CHECK is added NOT VALID: adding it holds the table exclusively only
// for the catalog change, not for a scan. It replaces a CHECK an earlier
// attempt added before its VALIDATE failed, so a corrected release can run
// the migration again.
const emailOutboxTraegerKindCheckSQL = `
ALTER TABLE platform.email_outbox DROP CONSTRAINT IF EXISTS chk_email_outbox_traeger_kind;
ALTER TABLE platform.email_outbox ADD CONSTRAINT chk_email_outbox_traeger_kind CHECK (
	organization_id IS NULL OR kind IN (
		'care_contract', 'care_contract_notice', 'holiday_registration_received', 'parent_billing_letter')) NOT VALID;
COMMENT ON CONSTRAINT chk_email_outbox_traeger_kind ON platform.email_outbox IS
	'The mail kinds of a Träger. A new kind needs its demo erasure in platform.erase_closed_demo_visitors (#6342).';
`

const pushOutboxTraegerKindCheckSQL = `
ALTER TABLE platform.push_outbox DROP CONSTRAINT IF EXISTS chk_push_outbox_traeger_kind;
ALTER TABLE platform.push_outbox ADD CONSTRAINT chk_push_outbox_traeger_kind CHECK (
	organization_id IS NULL OR kind IN (
		'care_contract_document', 'parent_billing_document', 'traeger_document_release', 'traeger_document_release_account')) NOT VALID;
COMMENT ON CONSTRAINT chk_push_outbox_traeger_kind ON platform.push_outbox IS
	'The push kinds of a Träger. A new kind needs its demo erasure in platform.erase_closed_demo_visitors (#6342).';
`

// VALIDATE CONSTRAINT scans its table under SHARE UPDATE EXCLUSIVE: the
// outbox keeps taking and sending entries meanwhile.
const (
	validateEmailOutboxTraegerKindSQL = `ALTER TABLE platform.email_outbox VALIDATE CONSTRAINT chk_email_outbox_traeger_kind`
	validatePushOutboxTraegerKindSQL  = `ALTER TABLE platform.push_outbox VALIDATE CONSTRAINT chk_push_outbox_traeger_kind`
)

const demoVisitorErasureOutboxKindsDownSQL = `
ALTER TABLE platform.email_outbox DROP CONSTRAINT IF EXISTS chk_email_outbox_traeger_kind;
ALTER TABLE platform.push_outbox DROP CONSTRAINT IF EXISTS chk_push_outbox_traeger_kind;
`

// The rows of a Träger with a kind the CHECK would refuse, per kind. Kinds
// and counts only, never a row's content.
const (
	undeclaredEmailOutboxTraegerKindsSQL = `SELECT kind, count(*) AS count FROM platform.email_outbox
		WHERE organization_id IS NOT NULL AND kind <> ALL (?::text[]) GROUP BY kind ORDER BY kind`
	undeclaredPushOutboxTraegerKindsSQL = `SELECT kind, count(*) AS count FROM platform.push_outbox
		WHERE organization_id IS NOT NULL AND kind <> ALL (?::text[]) GROUP BY kind ORDER BY kind`
)

var errUndeclaredOutboxTraegerKind = errors.New("rows of a Träger with a kind the outbox does not declare")

// demoVisitorErasureOutboxKindsPrecondition stops the release before the
// application stops when a row of a Träger has a kind the new CHECK would
// refuse; VALIDATE CONSTRAINT would fail on it inside the migration. It
// reports each such kind with its count. A table whose rows of a Träger come
// with a migration still pending in the same release has none yet.
func demoVisitorErasureOutboxKindsPrecondition(ctx context.Context, db *bun.DB) error {
	type undeclaredKind struct {
		Kind  string
		Count int
	}
	var undeclared []string
	report := func(table string, rows []undeclaredKind) {
		for _, row := range rows {
			undeclared = append(undeclared, fmt.Sprintf("%s %s: %d", table, row.Kind, row.Count))
		}
	}
	if !migrationPending(ctx, holidayRegistrationsVersion) {
		var rows []undeclaredKind
		if err := db.NewRaw(undeclaredEmailOutboxTraegerKindsSQL, pgdialect.Array(emailOutboxTraegerKinds)).Scan(ctx, &rows); err != nil {
			return fmt.Errorf("count undeclared mail kinds of a Träger: %w", err)
		}
		report("platform.email_outbox", rows)
	}
	if !migrationPending(ctx, parentBillingDocumentInboxVersion) {
		var rows []undeclaredKind
		if err := db.NewRaw(undeclaredPushOutboxTraegerKindsSQL, pgdialect.Array(pushOutboxTraegerKinds)).Scan(ctx, &rows); err != nil {
			return fmt.Errorf("count undeclared push kinds of a Träger: %w", err)
		}
		report("platform.push_outbox", rows)
	}
	if len(undeclared) > 0 {
		return fmt.Errorf("%w (declare or remove them first): %s", errUndeclaredOutboxTraegerKind, strings.Join(undeclared, "; "))
	}
	return nil
}

// demoVisitorErasureOutboxKindsUp adds both CHECKs NOT VALID, then validates
// each in a statement of its own. Each ExecContext is its own transaction,
// so the exclusive lock of an ADD ends before any scan; rows written in
// between are already checked. The erasure goes last.
func demoVisitorErasureOutboxKindsUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, emailOutboxTraegerKindCheckSQL); err != nil {
		return fmt.Errorf("declare the mail kinds of a Träger: %w", err)
	}
	if _, err := db.ExecContext(ctx, pushOutboxTraegerKindCheckSQL); err != nil {
		return fmt.Errorf("declare the push kinds of a Träger: %w", err)
	}
	if _, err := db.ExecContext(ctx, validateEmailOutboxTraegerKindSQL); err != nil {
		return fmt.Errorf("validate the mail kinds of a Träger: %w", err)
	}
	if _, err := db.ExecContext(ctx, validatePushOutboxTraegerKindSQL); err != nil {
		return fmt.Errorf("validate the push kinds of a Träger: %w", err)
	}
	return installOutboxKindErasure(ctx, db)
}

func demoVisitorErasureOutboxKindsDown(ctx context.Context, db *bun.DB) error {
	if err := installCareNoticeErasure(ctx, db); err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, demoVisitorErasureOutboxKindsDownSQL); err != nil {
		return fmt.Errorf("drop the outbox kinds of a Träger: %w", err)
	}
	return nil
}
