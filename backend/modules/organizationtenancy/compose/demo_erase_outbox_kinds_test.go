package compose

import (
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// traegerOutboxRows has, per kind of a Träger in an outbox table (key
// "<table> <kind>", as in demoErasureOutboxKindExceptions), the SQL that
// returns the ids of the rows of that kind about the school ?0, writing them
// where leaveVisitorTraces did not: each carries the visitor's address or
// name, or the visitor's parent account ?1. They run in key order.
var traegerOutboxRows = map[string]string{
	"platform.email_outbox care_contract": `SELECT outbox.id FROM platform.email_outbox AS outbox
		JOIN care_contract.contracts AS contract ON contract.id = outbox.related_entity_id
		WHERE outbox.kind = 'care_contract' AND outbox.related_entity_type = 'care_contract' AND contract.school_id = ?0`,
	// The confirmation of a termination (#4682).
	"platform.email_outbox care_contract_notice": `INSERT INTO platform.email_outbox
			(organization_id, kind, recipient, payload, related_entity_type, related_entity_id)
		SELECT organization_id, 'care_contract_notice', jsonb_build_object('address', 'eltern@example.org', 'name', '` + visitorFull + `'),
			jsonb_build_object('guardian_name', '` + visitorFull + `'), 'care_contract', id
		FROM care_contract.contracts WHERE school_id = ?0 RETURNING id`,
	// The receipt of the holiday registration (#4662).
	"platform.email_outbox holiday_registration_received": `INSERT INTO platform.email_outbox
			(organization_id, kind, recipient, payload, related_entity_type, related_entity_id)
		SELECT organization_id, 'holiday_registration_received', jsonb_build_object('address', 'besuch@example.org'),
			jsonb_build_object('guardian_name', '` + visitorFull + `'), 'holiday_registration', holiday_registration_id
		FROM enrollment.holiday_registration_children WHERE home_school_id = ?0 RETURNING id`,
	"platform.email_outbox parent_billing_letter": `SELECT outbox.id FROM platform.email_outbox AS outbox
		JOIN parent_billing.document_mailings AS mailing ON mailing.organization_id = outbox.organization_id AND mailing.delivery_id = outbox.id
		JOIN parent_billing.documents AS document ON document.organization_id = mailing.organization_id AND document.id = mailing.document_id
		WHERE outbox.kind = 'parent_billing_letter' AND document.number = 'F-' || ?0`,
	"platform.push_outbox care_contract_document": `SELECT push.id FROM platform.push_outbox AS push
		JOIN care_contract.contracts AS contract ON contract.id = push.related_entity_id
		WHERE push.kind = 'care_contract_document' AND contract.school_id = ?0`,
	"platform.push_outbox parent_billing_document": `SELECT push.id FROM platform.push_outbox AS push
		JOIN parent_billing.documents AS document ON document.organization_id = push.organization_id AND document.id = push.related_entity_id
		WHERE push.kind = 'parent_billing_document' AND document.number = 'F-' || ?0`,
	// The Träger released a document for the parents of the school (#5941).
	"platform.push_outbox traeger_document_release": `WITH document AS (
			INSERT INTO platform.traeger_documents (organization_id, title, file_name, content_type, size_bytes, content)
			SELECT organization_id, 'Elternbrief ' || id, 'brief.pdf', 'application/pdf', 3, '\x255044'::bytea
			FROM platform.schools WHERE id = ?0 RETURNING organization_id, id),
		release AS (
			INSERT INTO platform.traeger_document_releases (organization_id, document_id, audience, school_id)
			SELECT organization_id, id, 'parents', ?0 FROM document RETURNING organization_id, id)
		INSERT INTO platform.push_outbox (organization_id, kind, related_entity_type, related_entity_id, recipient, payload)
		SELECT organization_id, 'traeger_document_release', 'traeger_document_release', id, '{"deferred":true}'::jsonb,
			jsonb_build_object('organization_id', organization_id, 'release_id', id)
		FROM release RETURNING id`,
	// It fanned out to the visitor's parent account.
	"platform.push_outbox traeger_document_release_account": `INSERT INTO platform.push_outbox
			(organization_id, kind, related_entity_type, related_entity_id, recipient, payload)
		SELECT organization_id, 'traeger_document_release_account', 'traeger_document_release', id, '{"deferred":true}'::jsonb,
			jsonb_build_object('organization_id', organization_id, 'release_id', id, 'account_id', ?1)
		FROM platform.traeger_document_releases WHERE school_id = ?0 RETURNING id`,
}

// schoolPushRow is a push of the school ?0 to the device of account ?1.
const schoolPushRow = `INSERT INTO platform.push_outbox (tenant_id, kind, related_entity_type, related_entity_id, recipient, payload)
	VALUES (?0, 'parent_message', 'parent_message', ?0, jsonb_build_object('account_id', ?1,
		'endpoint', 'https://push.example.org/' || ?0, 'p256dh', 'p256dh-key', 'auth', 'auth-key'),
		jsonb_build_object('title', 'Nachricht von ` + visitorFull + `')) RETURNING id`

var declaredKind = regexp.MustCompile(`'([a-z_]+)'::text`)

// declaredTraegerOutboxKinds reads the kinds of a Träger the schema allows
// in each outbox table, keyed like traegerOutboxRows.
func declaredTraegerOutboxKinds(t *testing.T, db *bun.DB) []string {
	t.Helper()
	var checks []struct {
		Table      string
		Definition string
	}
	require.NoError(t, db.NewRaw(`SELECT n.nspname || '.' || c.relname AS "table", pg_get_constraintdef(k.oid) AS definition
		FROM pg_constraint AS k JOIN pg_class AS c ON c.oid = k.conrelid JOIN pg_namespace AS n ON n.oid = c.relnamespace
		WHERE k.conname IN ('chk_email_outbox_traeger_kind', 'chk_push_outbox_traeger_kind')`).Scan(t.Context(), &checks))
	require.Len(t, checks, 2, "both outbox tables declare the kinds of a Träger")
	var kinds []string
	for _, check := range checks {
		for _, kind := range declaredKind.FindAllStringSubmatch(check.Definition, -1) {
			kinds = append(kinds, check.Table+" "+kind[1])
		}
	}
	sort.Strings(kinds)
	return kinds
}

// leaveOutboxRows writes the rows of every kind about the school and returns
// their ids per kind, and the push of the school under "school push".
func leaveOutboxRows(t *testing.T, db *bun.DB, schoolID, parentAccountID int64) map[string][]int64 {
	t.Helper()
	keys := make([]string, 0, len(traegerOutboxRows))
	for key := range traegerOutboxRows {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	rows := map[string][]int64{}
	for _, key := range append(keys, "school push") {
		query := traegerOutboxRows[key]
		if key == "school push" {
			query = schoolPushRow
		}
		var ids []int64
		require.NoError(t, db.NewRaw(query, schoolID, parentAccountID).Scan(t.Context(), &ids), key)
		require.NotEmpty(t, ids, "%s has a row about school %d", key, schoolID)
		rows[key] = ids
	}
	return rows
}

// remainingOutboxRows are the rows of rows still there.
func remainingOutboxRows(t *testing.T, db *bun.DB, rows map[string][]int64) map[string][]int64 {
	t.Helper()
	remaining := map[string][]int64{}
	for key, ids := range rows {
		table := "platform.push_outbox"
		if strings.HasPrefix(key, "platform.email_outbox ") {
			table = "platform.email_outbox"
		}
		if left := remainingIDs(t, db, table, ids...); len(left) > 0 {
			remaining[key] = left
		}
	}
	return remaining
}

// Every kind of the outbox the erasure has to find goes with the visitor
// (#6342): each kind of a Träger the schema declares has a row about the
// retired demo school that carries the visitor, and so has a push of the
// school. All of them go; the same rows about another visitor's demo school
// of the same Träger, still in use, and about a regular school stay.
func TestDemoErasureErasesEveryOutboxKindOfTheVisitor(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	retiredID, _ := testpkg.CreateTestTenant(t, db)
	otherVisitorID := schoolOfOrganization(t, db, retiredID, false)
	regularID, _ := testpkg.CreateTestTenant(t, db)

	fixtures := make([]string, 0, len(traegerOutboxRows))
	for key := range traegerOutboxRows {
		fixtures = append(fixtures, key)
	}
	sort.Strings(fixtures)
	require.Equal(t, declaredTraegerOutboxKinds(t, db), fixtures,
		"every kind of a Träger the schema declares has a row here; a new kind needs one, and its erasure")

	accounts := map[int64]visitorAccounts{}
	rows := map[int64]map[string][]int64{}
	for _, schoolID := range []int64{retiredID, otherVisitorID, regularID} {
		accounts[schoolID] = leaveVisitorTraces(t, db, schoolID)
		onlyInSchools(t, db, accounts[schoolID].caregiver, schoolID)
		onlyInSchools(t, db, accounts[schoolID].parent, schoolID)
		rows[schoolID] = leaveOutboxRows(t, db, schoolID, accounts[schoolID].parent)
	}
	// The other visitor's order is finished and still in use.
	ctx := t.Context()
	queue, err := NewDemoSchoolQueue(db)
	require.NoError(t, err)
	schools, err := NewDemoSchools(db)
	require.NoError(t, err)
	otherSlug := orderDemoSchool(t, db, "OGS Ost")
	_, err = queue.ClaimDemoSchoolOrder(ctx)
	require.NoError(t, err)
	require.NoError(t, schools.RememberDemoSchool(ctx, otherSlug, organizationtenancy.DemoSchoolState{
		SchoolID: otherVisitorID, SeedJSON: []byte(`{}`),
		VisitorAccountID: accounts[otherVisitorID].caregiver, VisitorParentAccountID: accounts[otherVisitorID].parent,
	}))
	require.NoError(t, queue.FinishDemoSchoolOrder(ctx, otherSlug, accounts[otherVisitorID].caregiver, accounts[otherVisitorID].parent))
	giveDemoAccess(t, db, otherSlug)

	retireVisitorOrder(t, db, retiredID, accounts[retiredID])

	assert.Empty(t, remainingOutboxRows(t, db, rows[retiredID]), "no row of any kind about the retired demo school stays")
	assert.Equal(t, rows[otherVisitorID], remainingOutboxRows(t, db, rows[otherVisitorID]), "another visitor's demo school keeps its rows")
	assert.Equal(t, rows[regularID], remainingOutboxRows(t, db, rows[regularID]), "a regular school keeps its rows")
}

// A release for every school of the Träger fans out to the parents of all
// its schools (#5941). The entry of the erased visitor account goes; the
// entry of another visitor's account, which keeps its school, stays.
func TestDemoErasureErasesTheReleaseEntryOfTheErasedAccount(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	retiredID, _ := testpkg.CreateTestTenant(t, db)
	otherID := schoolOfOrganization(t, db, retiredID, false)
	retired, other := leaveVisitorTraces(t, db, retiredID), leaveVisitorTraces(t, db, otherID)
	for _, accountID := range []int64{retired.caregiver, retired.parent} {
		onlyInSchools(t, db, accountID, retiredID)
	}
	onlyInSchools(t, db, other.parent, otherID)
	var entries []int64
	require.NoError(t, db.NewRaw(`WITH document AS (
			INSERT INTO platform.traeger_documents (organization_id, title, file_name, content_type, size_bytes, content)
			SELECT organization_id, 'Hausordnung', 'hausordnung.pdf', 'application/pdf', 3, '\x255044'::bytea
			FROM platform.schools WHERE id = ?0 RETURNING organization_id, id),
		release AS (
			INSERT INTO platform.traeger_document_releases (organization_id, document_id, audience)
			SELECT organization_id, id, 'parents' FROM document RETURNING organization_id, id)
		INSERT INTO platform.push_outbox (organization_id, kind, related_entity_type, related_entity_id, recipient, payload)
		SELECT release.organization_id, 'traeger_document_release_account', 'traeger_document_release', release.id, '{"deferred":true}'::jsonb,
			jsonb_build_object('organization_id', release.organization_id, 'release_id', release.id, 'account_id', account.id)
		FROM release CROSS JOIN (VALUES (?1::bigint, 1), (?2::bigint, 2)) AS account(id, position)
		ORDER BY account.position RETURNING id`, retiredID, retired.parent, other.parent).Scan(t.Context(), &entries))
	require.Len(t, entries, 2)

	retireVisitorOrder(t, db, retiredID, retired)

	assert.Equal(t, entries[1:], remainingIDs(t, db, "platform.push_outbox", entries...),
		"the erased visitor account loses its entry, another account keeps its own")
}

// requireDeferredWhileHeld: a push of the order that a sender holds
// (held, with the school as its argument) defers the order (#5540). The run
// gives up after its lock timeout of 100 ms and leaves the order unchanged,
// including the mails it deleted before the pushes; the next run erases it
// completely.
func requireDeferredWhileHeld(t *testing.T, held string) {
	t.Helper()
	order := newHiddenDemoOrder(t)
	for _, key := range []string{"platform.push_outbox traeger_document_release", "school push"} {
		query := traegerOutboxRows[key]
		if key == "school push" {
			query = schoolPushRow
		}
		var ids []int64
		require.NoError(t, order.db.NewRaw(query, order.schoolID, order.accounts.parent).Scan(t.Context(), &ids), key)
		require.NotEmpty(t, ids, key)
	}
	before := order.snapshot(t)
	require.NotEqual(t, "[]", before["platform.push_outbox"], "the snapshot holds the pushes of the order")
	require.NotEqual(t, "[]", before["platform.email_outbox"], "the snapshot holds mails the run deletes before the pushes")
	order.writerFirst(t, statement(held, order.schoolID))
	order.requireErasedNext(t)
	assert.Equal(t, "[]", order.snapshot(t)["platform.push_outbox"], "no push of the order stays")
}

func TestDemoErasureDefersWhileAPushOfTheSchoolIsHeld(t *testing.T) {
	t.Parallel()
	requireDeferredWhileHeld(t, `SELECT id FROM platform.push_outbox WHERE tenant_id = ? FOR UPDATE`)
}

func TestDemoErasureDefersWhileAPushOfTheTraegerIsHeld(t *testing.T) {
	t.Parallel()
	requireDeferredWhileHeld(t, `SELECT push.id FROM platform.push_outbox AS push
		JOIN care_contract.contracts AS contract ON contract.id = push.related_entity_id
		WHERE push.kind = 'care_contract_document' AND contract.school_id = ? FOR UPDATE OF push`)
}

func TestDemoErasureDefersWhileTheReleasePushOfTheSchoolIsHeld(t *testing.T) {
	t.Parallel()
	requireDeferredWhileHeld(t, `SELECT push.id FROM platform.push_outbox AS push
		JOIN platform.traeger_document_releases AS release ON release.id = push.related_entity_id
		WHERE push.kind = 'traeger_document_release' AND release.school_id = ? FOR UPDATE OF push`)
}
