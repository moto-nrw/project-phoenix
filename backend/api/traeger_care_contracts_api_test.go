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
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	querybudgets "github.com/moto-nrw/project-phoenix/test/query-budgets"
)

type careContractServiceBody struct {
	Weekday        string `json:"weekday"`
	CareOfferingID int64  `json:"care_offering_id"`
	ModuleName     string `json:"module_name"`
	EndsAt         string `json:"ends_at"`
}

type careContractCandidateBody struct {
	SchoolID       int64                     `json:"school_id"`
	RequestChildID int64                     `json:"request_child_id"`
	ChildFirstName string                    `json:"child_first_name"`
	GuardianEmail  string                    `json:"guardian_email"`
	Services       []careContractServiceBody `json:"services"`
	EffectiveFrom  string                    `json:"effective_from"`
	TemplateIssue  string                    `json:"template_issue"`
}

type careContractSummaryBody struct {
	ID            int64    `json:"id"`
	Status        string   `json:"status"`
	SchoolID      int64    `json:"school_id"`
	ModuleNames   []string `json:"module_names"`
	PartyEmail    string   `json:"party_email"`
	EffectiveFrom string   `json:"effective_from"`
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
		Number         int                       `json:"number"`
		Status         string                    `json:"status"`
		ChildFirstName string                    `json:"child_first_name"`
		ChildBirthDate string                    `json:"child_birth_date"`
		EndsOn         *string                   `json:"ends_on"`
		Services       []careContractServiceBody `json:"services"`
		PriceCents     *int64                    `json:"price_cents"`
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
	"term_kind": "school_year", "notice_months": 1, "notice_to": "month_end", "contract_partner": "Träger Vertrag",
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
			manual_selected_days, valid_from) VALUES (?, ?, ?, '[]'::jsonb, DATE '2027-08-01')`,
		schoolID, childID, offeringID)
	require.NoError(t, err)
	return childID
}

type careContractFixture struct {
	reviewFixture
	school   int64
	phase    int64
	offering int64
	child    int64
}

func newCareContractFixture(t *testing.T, name string) careContractFixture {
	t.Helper()
	f := newReviewFixture(t, name)
	f.db = testpkg.SetupTestDB(t)
	testpkg.OwnTraegerCareContracts(t, f.db, f.office.organization)
	school := f.office.schools[0]
	ctx := context.Background()
	var phaseID int64
	require.NoError(t, f.db.NewRaw(`INSERT INTO enrollment.phases (tenant_id, name, kind, service_start_date, service_end_date,
			care_overflow_mode, care_offering_selection_mode, is_active)
		VALUES (?, 'Schuljahr 2027/28', 'school_year', DATE '2027-08-01', DATE '2028-07-31', 'waitlist', 'optional', true) RETURNING id`,
		school).Scan(ctx, &phaseID))
	offering := &testpkg.CareOffering{PhaseID: phaseID, Name: "Offene Ganztagsschule", DaysOfWeekMode: "fixed",
		AvailableDays: []string{"mon", "tue", "wed"}, AutoAddGradeLevels: []int{}, IsActive: true, CountsAsCare: true}
	offering.TenantID = school
	testpkg.InsertTestCareOffering(t, f.db, testpkg.TenantContext(school), offering)
	// The offering ends at 14:00 on Monday and at 16:30 on the other days.
	_, err := f.db.ExecContext(ctx, `UPDATE enrollment.care_offerings SET price_cents = 9500,
		pickup_times = '{"mon":"14:00","tue":"16:30","wed":"16:30","thu":"16:30","fri":"16:30"}'::jsonb WHERE id = ?`, offering.ID)
	require.NoError(t, err)
	child := careContractApplication(t, f.db, school, phaseID, offering.ID, "Lina", "erika.muster@example.org")
	// An AG the child books too counts as no care: a Betreuungsvertrag does
	// not cover it.
	ag := &testpkg.CareOffering{PhaseID: phaseID, Name: "Fußball-AG", DaysOfWeekMode: "fixed",
		AvailableDays: []string{"thu"}, AutoAddGradeLevels: []int{}, IsActive: true}
	ag.TenantID = school
	testpkg.InsertTestCareOffering(t, f.db, testpkg.TenantContext(school), ag)
	_, err = f.db.ExecContext(ctx, `INSERT INTO enrollment.care_offering_bookings (tenant_id, request_child_id, care_offering_id,
			manual_selected_days, valid_from) VALUES (?, ?, ?, '["thu"]'::jsonb, DATE '2027-08-01')`, school, child, ag.ID)
	require.NoError(t, err)
	return careContractFixture{reviewFixture: f, school: school, phase: phaseID, offering: offering.ID, child: child}
}

func (f careContractFixture) key() map[string]any {
	return map[string]any{"school_id": f.school, "request_child_id": f.child}
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
	assert.Equal(t, []careContractServiceBody{
		{Weekday: "mon", CareOfferingID: f.offering, ModuleName: "Offene Ganztagsschule", EndsAt: "14:00"},
		{Weekday: "tue", CareOfferingID: f.offering, ModuleName: "Offene Ganztagsschule", EndsAt: "16:30"},
		{Weekday: "wed", CareOfferingID: f.offering, ModuleName: "Offene Ganztagsschule", EndsAt: "16:30"},
	}, candidates[0].Services, "per weekday the module with its end time; the AG on Thursday is no care")
	assert.Equal(t, "2027-08-01", candidates[0].EffectiveFrom)
	assert.Equal(t, "template_missing", candidates[0].TemplateIssue)

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
	assert.Equal(t, []string{"Offene Ganztagsschule"}, contract.ModuleNames)

	// A second creation for the same child makes no second contract.
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
	require.NotNil(t, detail.Version.EndsOn)
	assert.Equal(t, "2028-07-31", *detail.Version.EndsOn, "the offering phase bounds the contract")
	assert.Equal(t, candidates[0].Services, detail.Version.Services, "the version keeps the services")
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

func TestCareContractListsQueryBudget(t *testing.T) {
	t.Parallel()
	f := newCareContractFixture(t, "Träger Vertrag Budget")
	token := f.office.token
	require.Equal(t, http.StatusOK, traegerJSON(t, f.traeger, token, http.MethodPut, templatePath(f.school, f.offering), careContractTemplate).Code)
	read := func(path string) []string {
		counter := testpkg.CaptureQueriesForContext(t, f.db)
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		f.traeger.ServeHTTP(rec, req.WithContext(counter.Context(req.Context())))
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		return counter.Selects("")
	}
	createAll := func() {
		rec := traegerJSON(t, f.traeger, token, http.MethodGet, "/traeger/contracts/candidates", nil)
		keys := []map[string]any{}
		for _, candidate := range decodeSchoolPortalData[[]careContractCandidateBody](t, rec.Body.Bytes()) {
			keys = append(keys, map[string]any{"school_id": candidate.SchoolID, "request_child_id": candidate.RequestChildID})
		}
		require.Equal(t, http.StatusCreated, traegerJSON(t, f.traeger, token, http.MethodPost, "/traeger/contracts", map[string]any{"contracts": keys}).Code)
	}
	candidates := read("/traeger/contracts/candidates")
	createAll()
	list := read("/traeger/contracts")
	for _, first := range []string{"Ben", "Mia", "Ole"} {
		careContractApplication(t, f.db, f.school, f.phase, f.offering, first, "familie-"+first+"@example.org")
	}
	assert.Len(t, read("/traeger/contracts/candidates"), len(candidates), "the candidates read the same statements for any number of children")
	createAll()
	assert.Len(t, read("/traeger/contracts"), len(list), "the list reads the same statements for any number of contracts")
	querybudgets.AssertQueryBudget(t, "api.traeger.care_contract_candidates", candidates)
	querybudgets.AssertQueryBudget(t, "api.traeger.care_contracts", list)
}

// Pagination and filters apply before the page is cut, not just to its rows.
func TestCareContractListPaginatesAndFiltersAllContracts(t *testing.T) {
	t.Parallel()
	f := newCareContractFixture(t, "Träger Vertrag Seiten")
	token := f.office.token
	other := careContractApplication(t, f.db, f.school, f.phase, f.offering, "Ben", "ben@example.org")
	require.Equal(t, http.StatusOK, traegerJSON(t, f.traeger, token, http.MethodPut, templatePath(f.school, f.offering), careContractTemplate).Code)
	rec := traegerJSON(t, f.traeger, token, http.MethodPost, "/traeger/contracts", map[string]any{"contracts": []any{f.key(), map[string]any{"school_id": f.school, "request_child_id": other}}})
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	created := decodeSchoolPortalData[careContractCreateBody](t, rec.Body.Bytes()).Created
	require.Len(t, created, 2)
	type page struct {
		Contracts []careContractSummaryBody `json:"contracts"`
		Counts    map[string]int            `json:"counts"`
		HasMore   bool                      `json:"has_more"`
	}
	read := func(query string) page {
		t.Helper()
		response := traegerJSON(t, f.traeger, token, http.MethodGet, "/traeger/contracts?"+query, nil)
		require.Equal(t, http.StatusOK, response.Code, response.Body.String())
		return decodeSchoolPortalData[page](t, response.Body.Bytes())
	}
	first := read("limit=1")
	require.Len(t, first.Contracts, 1)
	assert.True(t, first.HasMore)
	assert.Equal(t, 2, first.Counts["entwurf"])
	next := read("limit=1&offset=1")
	require.Len(t, next.Contracts, 1)
	assert.False(t, next.HasMore)
	assert.NotEqual(t, first.Contracts[0].ID, next.Contracts[0].ID)
	assert.Empty(t, read("offset=1000").Contracts)
	assert.Len(t, read("search=Lina&limit=1").Contracts, 1)
	assert.Len(t, read("search=Ganztagsschule").Contracts, 2, "module names remain searchable")
	assert.Empty(t, read("status=versendet").Contracts)
	assert.Len(t, read("school_id="+strconv.FormatInt(f.school, 10)).Contracts, 2)
	assert.Empty(t, read("school_id="+strconv.FormatInt(f.office.schools[1], 10)).Contracts)
	for _, query := range []string{"offset=-1", "offset=no", "limit=101"} {
		assert.Equal(t, http.StatusBadRequest, traegerJSON(t, f.traeger, token, http.MethodGet, "/traeger/contracts?"+query, nil).Code)
	}
}

func TestCareContractFailedDeliveryCanUseACorrectedAddressWithItsFrozenPDF(t *testing.T) {
	t.Parallel()
	f := newCareContractFixture(t, "Träger Vertrag Empfänger")
	token := f.office.token
	require.Equal(t, http.StatusOK, traegerJSON(t, f.traeger, token, http.MethodPut, templatePath(f.school, f.offering), careContractTemplate).Code)
	rec := traegerJSON(t, f.traeger, token, http.MethodPost, "/traeger/contracts", map[string]any{"contracts": []any{f.key()}})
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	created := decodeSchoolPortalData[careContractCreateBody](t, rec.Body.Bytes()).Created
	require.Len(t, created, 1)
	id := created[0].ID
	send := func(address *string) *httptest.ResponseRecorder {
		t.Helper()
		body := map[string]any{"contract_ids": []int64{id}}
		if address != nil {
			body["recipient_email"] = *address
		}
		return traegerJSON(t, f.traeger, token, http.MethodPost, "/traeger/contracts/send", body)
	}
	correctedBatch := traegerJSON(t, f.traeger, token, http.MethodPost, "/traeger/contracts/send", map[string]any{"contract_ids": []int64{id, id}, "recipient_email": "neu@example.org"})
	require.Equal(t, http.StatusBadRequest, correctedBatch.Code, "a correction must identify exactly one contract")
	corrected := "korrigiert@example.org"
	// A new address is not a way to bypass draft review or a pending send.
	rec = send(&corrected)
	require.Equal(t, http.StatusAccepted, rec.Code)
	assert.Contains(t, rec.Body.String(), "not_sendable")
	require.Equal(t, http.StatusAccepted, send(nil).Code)
	rec = send(&corrected)
	assert.Contains(t, rec.Body.String(), "mail_pending")
	fail := func() {
		t.Helper()
		_, err := f.db.ExecContext(context.Background(), `UPDATE platform.email_outbox SET status = 'dead_letter', dead_letter_at = NOW(), last_error = 'recipient rejected'
   WHERE organization_id = ? AND kind = 'care_contract' AND related_entity_id = ?`, f.office.organization, id)
		require.NoError(t, err)
	}
	fail()
	for _, invalid := range []string{"kein-postfach", "", "Name <neu@example.org>", "neu@example.org\r\nBcc: fremd@example.org"} {
		require.Equal(t, http.StatusBadRequest, send(&invalid).Code)
	}
	require.Equal(t, http.StatusAccepted, send(&corrected).Code)
	fail()
	require.Equal(t, http.StatusAccepted, send(nil).Code)
	var mails []struct {
		Recipient string `json:"recipient"`
		PDF       []byte `json:"pdf"`
	}
	var payloads []json.RawMessage
	require.NoError(t, f.db.NewRaw(`SELECT payload FROM platform.email_outbox WHERE organization_id = ? AND kind = 'care_contract' AND related_entity_id = ? ORDER BY id`, f.office.organization, id).Scan(context.Background(), &payloads))
	for _, payload := range payloads {
		var mail struct {
			Recipient string `json:"recipient"`
			PDF       []byte `json:"pdf"`
		}
		require.NoError(t, json.Unmarshal(payload, &mail))
		mails = append(mails, mail)
	}
	require.Len(t, mails, 3)
	assert.Equal(t, "erika.muster@example.org", mails[0].Recipient)
	assert.Equal(t, corrected, mails[1].Recipient)
	assert.Equal(t, corrected, mails[2].Recipient, "later retries keep the latest delivery address")
	assert.Equal(t, mails[0].PDF, mails[1].PDF)
	assert.Equal(t, mails[0].PDF, mails[2].PDF)
	detail := decodeSchoolPortalData[careContractDetailBody](t, traegerJSON(t, f.traeger, token, http.MethodGet, contractPath(id, ""), nil).Body.Bytes())
	assert.Equal(t, "erika.muster@example.org", detail.PartyEmail, "the original contract party stays frozen")
	require.Len(t, detail.Mailings, 3, "the complete delivery history remains")
}

func TestCareContractWithoutAPartnerCannotFreezeOrSendItsPDF(t *testing.T) {
	t.Parallel()
	f := newCareContractFixture(t, "Träger Vertrag ohne Namen")
	terms := map[string]any{"term_kind": "school_year", "notice_months": 1, "notice_to": "month_end"}
	token := f.office.token
	require.Equal(t, http.StatusOK, traegerJSON(t, f.traeger, token, http.MethodPut, templatePath(f.school, f.offering), terms).Code)
	rec := traegerJSON(t, f.traeger, token, http.MethodPost, "/traeger/contracts", map[string]any{"contracts": []any{f.key()}})
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	created := decodeSchoolPortalData[careContractCreateBody](t, rec.Body.Bytes()).Created
	require.Len(t, created, 1)
	id := created[0].ID
	rec = traegerJSON(t, f.traeger, token, http.MethodPost, "/traeger/contracts/send", map[string]any{"contract_ids": []int64{id}})
	require.Equal(t, http.StatusAccepted, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"reason":"partner_missing"`)
	detail := decodeSchoolPortalData[careContractDetailBody](t, traegerJSON(t, f.traeger, token, http.MethodGet, contractPath(id, ""), nil).Body.Bytes())
	assert.Equal(t, "entwurf", detail.Status)
	assert.False(t, detail.DocumentFrozen)
	var mails int
	require.NoError(t, f.db.NewRaw(`SELECT COUNT(*) FROM platform.email_outbox WHERE organization_id = ? AND kind = 'care_contract' AND related_entity_id = ?`, f.office.organization, id).Scan(context.Background(), &mails))
	assert.Zero(t, mails)
}
