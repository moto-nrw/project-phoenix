// Router-level tests for the push „Neues Dokument vom Träger“ of a sent care
// contract and of a release for parents (#5941): the Träger's write puts one
// deferred push on Delivery's outbox in the same Träger transaction, and
// neither a further send, a retry nor a repeated release adds one. The stores'
// own Träger filters of the worker's checks are pinned beneath RLS.
package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	careContractCompose "github.com/moto-nrw/project-phoenix/modules/carecontract/compose"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	organizationtenancyCompose "github.com/moto-nrw/project-phoenix/modules/organizationtenancy/compose"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

type queuedTraegerPush struct {
	TenantID       *int64          `bun:"tenant_id"`
	Recipient      json.RawMessage `bun:"recipient"`
	Payload        json.RawMessage `bun:"payload"`
	IdempotencyKey string          `bun:"idempotency_key"`
	RelatedType    string          `bun:"related_entity_type"`
	RelatedID      int64           `bun:"related_entity_id"`
}

func queuedTraegerPushes(t *testing.T, db *bun.DB, organization int64, kind string) []queuedTraegerPush {
	t.Helper()
	var queued []queuedTraegerPush
	require.NoError(t, db.NewRaw(`SELECT tenant_id, recipient, payload, idempotency_key, related_entity_type, related_entity_id
		FROM platform.push_outbox WHERE organization_id = ? AND kind = ? ORDER BY id`, organization, kind).
		Scan(context.Background(), &queued))
	return queued
}

// TestContractSendPushesOncePerAddress pins the contract half of #5941: the
// first send of a contract puts one push for its address on the outbox,
// named by kind and related type of its own; a send again, a send after
// Delivery forgot the push and a retried request add nothing, a corrected
// address after a failed mail adds one for the new recipient.
func TestContractSendPushesOncePerAddress(t *testing.T) {
	t.Parallel()
	f := newCareContractFixture(t, "Träger Vertrag Push")
	token := f.office.token
	require.Equal(t, http.StatusOK, traegerJSON(t, f.traeger, token, http.MethodPut, templatePath(f.school, f.offering), careContractTemplate).Code)
	rec := traegerJSON(t, f.traeger, token, http.MethodPost, "/traeger/contracts", map[string]any{"contracts": []any{f.key()}})
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	id := decodeSchoolPortalData[careContractCreateBody](t, rec.Body.Bytes()).Created[0].ID
	pushes := func() []queuedTraegerPush {
		return queuedTraegerPushes(t, f.db, f.office.organization, "care_contract_document")
	}
	assert.Empty(t, pushes(), "a draft pushes nothing")
	send := func(body map[string]any) {
		t.Helper()
		body["contract_ids"] = []int64{id}
		rec := traegerJSON(t, f.traeger, token, http.MethodPost, "/traeger/contracts/send", body)
		require.Equal(t, http.StatusAccepted, rec.Code, rec.Body.String())
		require.Contains(t, rec.Body.String(), fmt.Sprintf(`"sent":[%d]`, id))
	}
	mailState := func(status string) {
		t.Helper()
		_, err := f.db.ExecContext(context.Background(), `UPDATE platform.email_outbox SET status = ?, sent_at = NOW(),
			dead_letter_at = CASE WHEN ? = 'dead_letter' THEN NOW() END, last_error = CASE WHEN ? = 'dead_letter' THEN 'recipient rejected' END
			WHERE organization_id = ? AND kind = 'care_contract' AND related_entity_id = ?`, status, status, status, f.office.organization, id)
		require.NoError(t, err)
	}

	send(map[string]any{})
	queued := pushes()
	require.Len(t, queued, 1)
	assert.Nil(t, queued[0].TenantID, "a push of the Träger names no school")
	assert.JSONEq(t, `{"deferred":true}`, string(queued[0].Recipient), "the devices are found when the worker sends")
	assert.Equal(t, "care_contract_document", queued[0].RelatedType, "not the letters' type: equal IDs never collide")
	assert.Equal(t, id, queued[0].RelatedID)
	assert.Regexp(t, fmt.Sprintf(`^care-contract-document:%d:[0-9a-f]{16}$`, id), queued[0].IdempotencyKey, "the key names no address")
	var payload struct {
		OrganizationID int64  `json:"organization_id"`
		ContractID     int64  `json:"contract_id"`
		RecipientEmail string `json:"recipient_email"`
	}
	require.NoError(t, json.Unmarshal(queued[0].Payload, &payload))
	assert.Equal(t, f.office.organization, payload.OrganizationID)
	assert.Equal(t, id, payload.ContractID)
	assert.Equal(t, "erika.muster@example.org", payload.RecipientEmail)

	mailState("sent")
	send(map[string]any{})
	assert.Len(t, pushes(), 1, "a send again pushes nothing new")

	// After Delivery forgot the ended push, the stored mailings still decide.
	_, err := f.db.ExecContext(context.Background(), `DELETE FROM platform.push_outbox WHERE organization_id = ?`, f.office.organization)
	require.NoError(t, err)
	mailState("sent")
	send(map[string]any{})
	assert.Empty(t, pushes(), "the mailing history outlives the outbox")

	mailState("dead_letter")
	send(map[string]any{"recipient_email": "korrigiert@example.org"})
	queued = pushes()
	require.Len(t, queued, 1, "a corrected address is a new recipient with one push")
	require.NoError(t, json.Unmarshal(queued[0].Payload, &payload))
	assert.Equal(t, "korrigiert@example.org", payload.RecipientEmail)
	mailState("dead_letter")
	send(map[string]any{})
	assert.Len(t, pushes(), 1, "a retry to the corrected address pushes nothing new")
}

// TestReleaseForParentsPushesOncePerRelease pins the release half of #5941:
// a release for parents puts one push for its release on the outbox; a
// release for schools, an upload and a refused repeat of the same release
// put nothing there; a new release after the revocation is a new push.
func TestReleaseForParentsPushesOncePerRelease(t *testing.T) {
	t.Parallel()
	f := newTraegerDocumentsFixture(t)
	token := f.office.token
	pushes := func() []queuedTraegerPush {
		return queuedTraegerPushes(t, f.db, f.office.organization, "traeger_document_release")
	}
	document := f.uploaded(t, token, "Elternbrief")
	assert.Empty(t, pushes(), "an upload without a release pushes nothing")

	f.released(t, token, document.ID, map[string]any{"audience": "schools"})
	assert.Empty(t, pushes(), "a release for schools tells no parent")

	release := f.released(t, token, document.ID, map[string]any{"audience": "parents", "school_id": f.office.schools[0]})
	queued := pushes()
	require.Len(t, queued, 1)
	assert.Equal(t, fmt.Sprintf("traeger-document-release:%d", release.ID), queued[0].IdempotencyKey)
	assert.Equal(t, "traeger_document_release", queued[0].RelatedType)
	assert.Equal(t, release.ID, queued[0].RelatedID)
	assert.Nil(t, queued[0].TenantID)
	var payload organizationtenancy.TraegerDocumentReleaseNotice
	require.NoError(t, json.Unmarshal(queued[0].Payload, &payload))
	assert.Equal(t, organizationtenancy.TraegerDocumentReleaseNotice{OrganizationID: f.office.organization, ReleaseID: release.ID, DocumentID: document.ID}, payload)

	rec := f.release(t, token, document.ID, map[string]any{"audience": "parents", "school_id": f.office.schools[0]})
	require.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
	assert.Len(t, pushes(), 1, "a repeated release is refused and pushes nothing")

	rec = traegerJSON(t, f.router, token, http.MethodDelete, fmt.Sprintf("/traeger/documents/%d/releases/%d", document.ID, release.ID), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	again := f.released(t, token, document.ID, map[string]any{"audience": "parents", "school_id": f.office.schools[0]})
	queued = pushes()
	require.Len(t, queued, 2, "a release after the revocation is a new event")
	assert.Equal(t, again.ID, queued[1].RelatedID)
}

// TestDocumentPushChecksFindNothingOfAnotherTraegerBeneathRLS pins the
// Träger filters of the worker's checks in the stores, beneath RLS (#5941,
// as in #4679 and #5993): Träger B asks in its own Träger transaction,
// running as the owner of the tables, for A's contract and A's release and
// finds neither. RLS alone would hide a missing filter, so this goes red when
// a check reads without the organization.
func TestDocumentPushChecksFindNothingOfAnotherTraegerBeneathRLS(t *testing.T) {
	t.Parallel()
	f := newTraegerDocumentsFixture(t)
	document := f.uploaded(t, f.office.token, "Elternbrief")
	release := f.released(t, f.office.token, document.ID, map[string]any{"audience": "parents"})
	other := f.newOffice(t, "Träger Fremd Push unter RLS", identityaccess.TraegerRoleDocuments)
	testpkg.OwnTraegerCareContracts(t, f.db, f.office.organization)
	testpkg.OwnTraegerCareContracts(t, f.db, other.organization)
	chain := testpkg.CreateTestParentGuardianChain(t, f.db)
	contract := pushContract(t, f.db, f.office.organization, f.office.schools[0], chain.StudentID, chain.Email)

	documents := organizationtenancyCompose.NewSharedTraegerDocuments()
	contracts := careContractCompose.NewInboxNotices()
	parent := organizationtenancy.TraegerDocumentParent{AccountID: chain.AccountID, Schools: map[int64][]int64{f.office.schools[0]: {}}}
	unit := tenant.WithUnitOfWork(context.Background(), testpkg.TenantRuntime(t, f.db))
	check := func(organization int64) (releaseFound, isNew, contractFound bool) {
		t.Helper()
		require.NoError(t, testpkg.WithinTraeger(unit, organization, func(txCtx context.Context) error {
			// As the owner of the tables, beneath RLS: only the stores' own
			// Träger filters keep A's rows out.
			require.NoError(t, testpkg.WithoutRequestRole(txCtx))
			var err error
			if _, releaseFound, err = documents.ActiveParentRelease(txCtx, organization, release.ID); err != nil {
				return err
			}
			if isNew, err = documents.ReleaseIsNewForParent(txCtx, organization, release.ID, parent); err != nil {
				return err
			}
			_, contractFound, err = contracts.ContractNoticeTarget(txCtx, organization, contract, chain.Email)
			return err
		}))
		return releaseFound, isNew, contractFound
	}

	releaseFound, isNew, contractFound := check(f.office.organization)
	require.True(t, releaseFound && isNew && contractFound, "the own Träger finds its release and its contract")
	releaseFound, isNew, contractFound = check(other.organization)
	assert.False(t, releaseFound, "another Träger's release is not found")
	assert.False(t, isNew, "another Träger's release covers no parent")
	assert.False(t, contractFound, "another Träger's contract is not found")
}

// pushContract stores a contract of the Träger for the child, sent to email.
func pushContract(t *testing.T, db *bun.DB, organization, school, studentID int64, email string) int64 {
	t.Helper()
	ctx := context.Background()
	var contract, version, document int64
	require.NoError(t, db.NewRaw(`INSERT INTO care_contract.contracts (organization_id, school_id, request_child_id, student_id, status, sent_at)
		VALUES (?, ?, ?, ?, 'versendet', NOW()) RETURNING id`, organization, school, testpkg.UniqueTestTenantID(t), studentID).Scan(ctx, &contract))
	require.NoError(t, db.NewRaw(`INSERT INTO care_contract.contract_versions (organization_id, contract_id, version_number, effective_from,
			child_first_name, child_last_name, school_name, term_kind, notice_months, notice_to, special_termination,
			special_termination_note, statute_required, statute_title, statute_url, contract_partner)
		VALUES (?, ?, 1, '2027-08-01', 'Felix', 'Schneider', 'OGS', 'open_ended', 1, 'month_end', false, '', false, '', '', '')
		RETURNING id`, organization, contract).Scan(ctx, &version))
	require.NoError(t, db.NewRaw(`INSERT INTO care_contract.contract_documents (organization_id, contract_id, contract_version_id,
			template_version, filename, pdf) VALUES (?, ?, ?, 0, 'betreuungsvertrag.pdf', '%PDF-1.4'::bytea) RETURNING id`,
		organization, contract, version).Scan(ctx, &document))
	_, err := db.ExecContext(ctx, `INSERT INTO care_contract.contract_mailings (organization_id, contract_id, contract_document_id,
			outbox_id, recipient_email) VALUES (?, ?, ?, ?, ?)`, organization, contract, document, testpkg.UniqueTestTenantID(t), email)
	require.NoError(t, err)
	return contract
}
