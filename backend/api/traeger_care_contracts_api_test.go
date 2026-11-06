// Router-level tests for the Betreuungsvertrag (#4678, ADR 0153): the
// template of an offering, the drafts from the accepted applications of the
// Träger's schools, the preview and the frozen PDF, the send by mail with the
// PDF, the right "Verträge verwalten", the Träger boundary and the record of
// every read and write.
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

type careContractCandidateBody struct {
	SchoolID       int64    `json:"school_id"`
	RequestChildID int64    `json:"request_child_id"`
	CareOfferingID int64    `json:"care_offering_id"`
	ChildFirstName string   `json:"child_first_name"`
	GuardianEmail  string   `json:"guardian_email"`
	OfferingName   string   `json:"offering_name"`
	BookedDays     []string `json:"booked_days"`
	EffectiveFrom  string   `json:"effective_from"`
	HasTemplate    bool     `json:"has_template"`
}

type careContractSummaryBody struct {
	ID            int64  `json:"id"`
	Status        string `json:"status"`
	SchoolID      int64  `json:"school_id"`
	OfferingName  string `json:"offering_name"`
	PartyEmail    string `json:"party_email"`
	EffectiveFrom string `json:"effective_from"`
}

type careContractCreateBody struct {
	Created []careContractSummaryBody `json:"created"`
	Skipped []struct {
		RequestChildID int64  `json:"request_child_id"`
		Reason         string `json:"reason"`
	} `json:"skipped"`
}

type careContractDetailBody struct {
	careContractSummaryBody
	Version struct {
		Number         int      `json:"number"`
		Status         string   `json:"status"`
		ChildFirstName string   `json:"child_first_name"`
		ChildBirthDate string   `json:"child_birth_date"`
		BookedDays     []string `json:"booked_days"`
		PriceCents     *int64   `json:"price_cents"`
		Terms          struct {
			TermKind     string `json:"term_kind"`
			NoticeMonths int    `json:"notice_months"`
		} `json:"terms"`
	} `json:"version"`
	Mailings []struct {
		Recipient string `json:"recipient"`
		State     string `json:"state"`
	} `json:"mailings"`
	DocumentFrozen bool `json:"document_frozen"`
}

var careContractTemplate = map[string]any{
	"term_kind": "school_year", "notice_months": 1, "notice_to": "month_end",
	"special_termination": true, "special_termination_note": "Umzug oder Schulwechsel",
	"statute_required": true, "statute_title": "Satzung über die Elternbeiträge", "statute_url": "https://example.org/satzung",
}

// careContractApplication is an accepted child of the school with an
// approved booking of the offering from Monday to Wednesday.
func careContractApplication(t *testing.T, db *bun.DB, schoolID, phaseID, offeringID int64, first, email string) int64 {
	t.Helper()
	ctx := context.Background()
	var requestID, childID int64
	require.NoError(t, db.NewRaw(`INSERT INTO enrollment.requests (tenant_id, phase_id, guardian_first_name, guardian_last_name,
			guardian_email, consent_flags, legal_blocks_snapshot, custom_data, source_metadata, status_token, submitted_at)
		VALUES (?, ?, 'Erika', 'Muster', ?, '{}'::jsonb, '[]'::jsonb, '{}'::jsonb, '{}'::jsonb, ?, NOW()) RETURNING id`,
		schoolID, phaseID, email, fmt.Sprintf("care-contract-%d-%s", schoolID, first)).Scan(ctx, &requestID))
	require.NoError(t, db.NewRaw(`INSERT INTO enrollment.request_children (tenant_id, request_id, first_name, last_name,
			date_of_birth, custom_data, status, activation_mode)
		VALUES (?, ?, ?, 'Muster', DATE '2019-03-04', '{}'::jsonb, 'approved', 'scheduled') RETURNING id`,
		schoolID, requestID, first).Scan(ctx, &childID))
	_, err := db.ExecContext(ctx, `INSERT INTO enrollment.care_offering_bookings (tenant_id, request_child_id, care_offering_id,
			manual_selected_days, valid_from) VALUES (?, ?, ?, '["mon","tue","wed"]'::jsonb, DATE '2027-08-01')`,
		schoolID, childID, offeringID)
	require.NoError(t, err)
	return childID
}

type careContractFixture struct {
	reviewFixture
	school   int64
	offering int64
	child    int64
}

func newCareContractFixture(t *testing.T, name string) careContractFixture {
	t.Helper()
	f := newReviewFixture(t, name)
	testpkg.OwnTraegerCareContracts(t, f.db, f.office.organization)
	school := f.office.schools[0]
	ctx := context.Background()
	var phaseID int64
	require.NoError(t, f.db.NewRaw(`INSERT INTO enrollment.phases (tenant_id, name, kind, service_start_date, service_end_date,
			care_overflow_mode, care_offering_selection_mode, is_active)
		VALUES (?, 'Schuljahr 2027/28', 'school_year', DATE '2027-08-01', DATE '2028-07-31', 'waitlist', 'optional', true) RETURNING id`,
		school).Scan(ctx, &phaseID))
	offering := &testpkg.CareOffering{PhaseID: phaseID, Name: "Offene Ganztagsschule", DaysOfWeekMode: "fixed",
		AvailableDays: []string{"mon", "tue", "wed", "thu", "fri"}, AutoAddGradeLevels: []int{}, IsActive: true, CountsAsCare: true}
	offering.TenantID = school
	testpkg.InsertTestCareOffering(t, f.db, testpkg.TenantContext(school), offering)
	_, err := f.db.ExecContext(ctx, `UPDATE enrollment.care_offerings SET price_cents = 9500 WHERE id = ?`, offering.ID)
	require.NoError(t, err)
	child := careContractApplication(t, f.db, school, phaseID, offering.ID, "Lina", "erika.muster@example.org")
	return careContractFixture{reviewFixture: f, school: school, offering: offering.ID, child: child}
}

func (f careContractFixture) key() map[string]any {
	return map[string]any{"school_id": f.school, "request_child_id": f.child, "care_offering_id": f.offering}
}

func templatePath(schoolID, offeringID int64) string {
	return fmt.Sprintf("/traeger/contract-templates/%d/%d", schoolID, offeringID)
}

func contractPath(id int64, suffix string) string {
	return "/traeger/contracts/" + strconv.FormatInt(id, 10) + suffix
}

func TestCareContractDraftFromTheAcceptedApplicationIsSentWithItsFrozenPDF(t *testing.T) {
	t.Parallel()
	f := newCareContractFixture(t, "Träger Vertrag Entwurf")
	token := f.office.token

	rec := traegerJSON(t, f.traeger, token, http.MethodGet, "/traeger/contracts/candidates", nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	candidates := decodeSchoolPortalData[[]careContractCandidateBody](t, rec.Body.Bytes())
	require.Len(t, candidates, 1)
	assert.Equal(t, f.child, candidates[0].RequestChildID)
	assert.Equal(t, []string{"mon", "tue", "wed"}, candidates[0].BookedDays)
	assert.Equal(t, "2027-08-01", candidates[0].EffectiveFrom)
	assert.False(t, candidates[0].HasTemplate)

	// Without a template there is no draft.
	rec = traegerJSON(t, f.traeger, token, http.MethodPost, "/traeger/contracts", map[string]any{"contracts": []any{f.key()}})
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	created := decodeSchoolPortalData[careContractCreateBody](t, rec.Body.Bytes())
	assert.Empty(t, created.Created)
	require.Len(t, created.Skipped, 1)
	assert.Equal(t, "template_missing", created.Skipped[0].Reason)

	invalid := map[string]any{"term_kind": "fixed_months", "notice_months": 13, "notice_to": "month_end", "statute_required": true}
	rec = traegerJSON(t, f.traeger, token, http.MethodPut, templatePath(f.school, f.offering), invalid)
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "care_contract.template_invalid")
	assert.Contains(t, rec.Body.String(), "term_months")
	assert.Contains(t, rec.Body.String(), "statute_title")
	rec = traegerJSON(t, f.traeger, token, http.MethodPut, templatePath(f.school, f.offering), careContractTemplate)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	rec = traegerJSON(t, f.traeger, token, http.MethodPost, "/traeger/contracts", map[string]any{"contracts": []any{f.key()}})
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	created = decodeSchoolPortalData[careContractCreateBody](t, rec.Body.Bytes())
	require.Len(t, created.Created, 1)
	contract := created.Created[0]
	assert.Equal(t, "entwurf", contract.Status)
	assert.Equal(t, "erika.muster@example.org", contract.PartyEmail)

	// A second creation for the same child and offering makes no second contract.
	rec = traegerJSON(t, f.traeger, token, http.MethodPost, "/traeger/contracts", map[string]any{"contracts": []any{f.key()}})
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	created = decodeSchoolPortalData[careContractCreateBody](t, rec.Body.Bytes())
	assert.Empty(t, created.Created)
	require.Len(t, created.Skipped, 1)
	assert.Equal(t, "contract_exists", created.Skipped[0].Reason)
	rec = traegerJSON(t, f.traeger, token, http.MethodGet, "/traeger/contracts/candidates", nil)
	assert.Empty(t, decodeSchoolPortalData[[]careContractCandidateBody](t, rec.Body.Bytes()))

	rec = traegerJSON(t, f.traeger, token, http.MethodGet, contractPath(contract.ID, ""), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	detail := decodeSchoolPortalData[careContractDetailBody](t, rec.Body.Bytes())
	assert.Equal(t, 1, detail.Version.Number)
	assert.Equal(t, "beantragt", detail.Version.Status)
	assert.Equal(t, "2019-03-04", detail.Version.ChildBirthDate)
	require.NotNil(t, detail.Version.PriceCents)
	assert.Equal(t, int64(9500), *detail.Version.PriceCents)
	assert.Equal(t, "school_year", detail.Version.Terms.TermKind)
	assert.False(t, detail.DocumentFrozen)

	// A later change of the template changes no contract.
	changed := map[string]any{"term_kind": "open_ended", "notice_months": 3, "notice_to": "half_year"}
	require.Equal(t, http.StatusOK, traegerJSON(t, f.traeger, token, http.MethodPut, templatePath(f.school, f.offering), changed).Code)
	rec = traegerJSON(t, f.traeger, token, http.MethodGet, contractPath(contract.ID, ""), nil)
	assert.Equal(t, "school_year", decodeSchoolPortalData[careContractDetailBody](t, rec.Body.Bytes()).Version.Terms.TermKind)

	rec = traegerJSON(t, f.traeger, token, http.MethodGet, contractPath(contract.ID, "/document"), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, "application/pdf", rec.Header().Get("Content-Type"))
	assert.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
	assert.True(t, bytes.HasPrefix(rec.Body.Bytes(), []byte("%PDF")))

	rec = traegerJSON(t, f.traeger, token, http.MethodPost, "/traeger/contracts/send", map[string]any{"contract_ids": []int64{contract.ID}})
	require.Equal(t, http.StatusAccepted, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), fmt.Sprintf(`"sent":[%d]`, contract.ID))

	var mails []struct {
		Payload        json.RawMessage `bun:"payload"`
		IdempotencyKey string          `bun:"idempotency_key"`
	}
	require.NoError(t, f.db.NewRaw(`SELECT payload, idempotency_key FROM platform.email_outbox
		WHERE organization_id = ? AND kind = 'care_contract' AND related_entity_type = 'care_contract' AND related_entity_id = ?`,
		f.office.organization, contract.ID).Scan(context.Background(), &mails))
	require.Len(t, mails, 1)
	assert.Equal(t, fmt.Sprintf("care-contract:%d:1", contract.ID), mails[0].IdempotencyKey)
	var payload struct {
		Recipient string `json:"recipient"`
		PDF       []byte `json:"pdf"`
	}
	require.NoError(t, json.Unmarshal(mails[0].Payload, &payload))
	assert.Equal(t, "erika.muster@example.org", payload.Recipient)
	assert.True(t, bytes.HasPrefix(payload.PDF, []byte("%PDF")))

	rec = traegerJSON(t, f.traeger, token, http.MethodGet, contractPath(contract.ID, ""), nil)
	detail = decodeSchoolPortalData[careContractDetailBody](t, rec.Body.Bytes())
	assert.Equal(t, "versendet", detail.Status)
	assert.True(t, detail.DocumentFrozen)
	require.Len(t, detail.Mailings, 1)
	assert.Equal(t, "queued", detail.Mailings[0].State)

	// The frozen PDF is the mailed one, and the waiting mail is not sent twice.
	rec = traegerJSON(t, f.traeger, token, http.MethodGet, contractPath(contract.ID, "/document"), nil)
	assert.Equal(t, payload.PDF, rec.Body.Bytes())
	rec = traegerJSON(t, f.traeger, token, http.MethodPost, "/traeger/contracts/send", map[string]any{"contract_ids": []int64{contract.ID}})
	require.Equal(t, http.StatusAccepted, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "mail_pending")

	// A sent contract cannot be discarded.
	rec = traegerJSON(t, f.traeger, token, http.MethodDelete, contractPath(contract.ID, ""), nil)
	require.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "care_contract.contract_not_draft")

	rec = traegerJSON(t, f.traeger, token, http.MethodGet, "/traeger/contracts?status=versendet", nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	list := decodeSchoolPortalData[struct {
		Contracts []careContractSummaryBody `json:"contracts"`
		Counts    map[string]int            `json:"counts"`
	}](t, rec.Body.Bytes())
	require.Len(t, list.Contracts, 1)
	assert.Equal(t, 1, list.Counts["versendet"])
	rec = traegerJSON(t, f.traeger, token, http.MethodGet, "/traeger/contracts?status=entwurf", nil)
	assert.Empty(t, decodeSchoolPortalData[struct {
		Contracts []careContractSummaryBody `json:"contracts"`
	}](t, rec.Body.Bytes()).Contracts)

	// Every read and write is in the trail, the read of the applications in
	// the access log of the school.
	var actions []string
	require.NoError(t, f.db.NewRaw(`SELECT action FROM care_contract.contract_events WHERE organization_id = ? ORDER BY id`,
		f.office.organization).Scan(context.Background(), &actions))
	for _, action := range []string{"template", "create", "read", "send"} {
		assert.Contains(t, actions, action)
	}
	var reads int
	require.NoError(t, f.db.NewRaw(`SELECT COUNT(*) FROM audit.data_access_log WHERE tenant_id = ?
		AND resource_type = 'care_contract_applications' AND actor_role = 'traeger'`, f.school).Scan(context.Background(), &reads))
	assert.GreaterOrEqual(t, reads, 2)
}

func TestCareContractDraftIsDiscardedAndOnlyADraft(t *testing.T) {
	t.Parallel()
	f := newCareContractFixture(t, "Träger Vertrag Verwerfen")
	token := f.office.token
	require.Equal(t, http.StatusOK, traegerJSON(t, f.traeger, token, http.MethodPut, templatePath(f.school, f.offering), careContractTemplate).Code)
	rec := traegerJSON(t, f.traeger, token, http.MethodPost, "/traeger/contracts", map[string]any{"contracts": []any{f.key()}})
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	contract := decodeSchoolPortalData[careContractCreateBody](t, rec.Body.Bytes()).Created[0]

	rec = traegerJSON(t, f.traeger, token, http.MethodDelete, contractPath(contract.ID, ""), nil)
	require.Equal(t, http.StatusNoContent, rec.Code, rec.Body.String())
	rec = traegerJSON(t, f.traeger, token, http.MethodGet, contractPath(contract.ID, ""), nil)
	require.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "care_contract.contract_not_found")

	// The child is a candidate again, and the trail keeps the draft.
	rec = traegerJSON(t, f.traeger, token, http.MethodGet, "/traeger/contracts/candidates", nil)
	assert.Len(t, decodeSchoolPortalData[[]careContractCandidateBody](t, rec.Body.Bytes()), 1)
	var discarded int
	require.NoError(t, f.db.NewRaw(`SELECT COUNT(*) FROM care_contract.contract_events WHERE organization_id = ? AND contract_id = ? AND action = 'discard'`,
		f.office.organization, contract.ID).Scan(context.Background(), &discarded))
	assert.Equal(t, 1, discarded)
}

func TestCareContractsNeedTheRightAndStayWithTheirTraeger(t *testing.T) {
	t.Parallel()
	f := newCareContractFixture(t, "Träger Vertrag Grenze")
	require.Equal(t, http.StatusOK, traegerJSON(t, f.traeger, f.office.token, http.MethodPut, templatePath(f.school, f.offering), careContractTemplate).Code)
	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodPost, "/traeger/contracts", map[string]any{"contracts": []any{f.key()}})
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	contract := decodeSchoolPortalData[careContractCreateBody](t, rec.Body.Bytes()).Created[0]

	// The Abrechnung holds no right "Verträge verwalten".
	billing := traegerColleague(t, f.reviewFixture, identityaccess.TraegerRoleBilling)
	for _, route := range []struct{ method, path string }{
		{http.MethodGet, "/traeger/contracts"}, {http.MethodGet, "/traeger/contracts/candidates"},
		{http.MethodPost, "/traeger/contracts"}, {http.MethodPost, "/traeger/contracts/send"},
		{http.MethodGet, contractPath(contract.ID, "")}, {http.MethodGet, contractPath(contract.ID, "/document")},
		{http.MethodDelete, contractPath(contract.ID, "")}, {http.MethodGet, "/traeger/contract-templates"},
		{http.MethodPut, templatePath(f.school, f.offering)},
	} {
		rec := traegerJSON(t, f.traeger, billing, route.method, route.path, map[string]any{})
		assert.Equal(t, http.StatusForbidden, rec.Code, "%s %s: %s", route.method, route.path, rec.Body.String())
	}

	// Another Träger sees none of it and cannot reach the school.
	other := newTraegerOffice(t, f.db, f.traeger, "Träger Vertrag Fremd", identityaccess.TraegerRoleAdministration)
	testpkg.OwnTraegerCareContracts(t, f.db, other.organization)
	rec = traegerJSON(t, f.traeger, other.token, http.MethodGet, contractPath(contract.ID, ""), nil)
	assert.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
	rec = traegerJSON(t, f.traeger, other.token, http.MethodGet, contractPath(contract.ID, "/document"), nil)
	assert.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
	rec = traegerJSON(t, f.traeger, other.token, http.MethodGet, "/traeger/contracts", nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Empty(t, decodeSchoolPortalData[struct {
		Contracts []careContractSummaryBody `json:"contracts"`
	}](t, rec.Body.Bytes()).Contracts)
	rec = traegerJSON(t, f.traeger, other.token, http.MethodPost, "/traeger/contracts/send", map[string]any{"contract_ids": []int64{contract.ID}})
	assert.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
	rec = traegerJSON(t, f.traeger, other.token, http.MethodPost, "/traeger/contracts", map[string]any{"contracts": []any{f.key()}})
	assert.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "care_contract.school_outside_traeger")
	rec = traegerJSON(t, f.traeger, other.token, http.MethodPut, templatePath(f.school, f.offering), careContractTemplate)
	assert.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
	rec = traegerJSON(t, f.traeger, other.token, http.MethodGet, "/traeger/contracts/candidates?school_id="+strconv.FormatInt(f.school, 10), nil)
	assert.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
}
