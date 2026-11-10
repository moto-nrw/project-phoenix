// The moto team records the instructions of a Träger for its Belege and
// sees the clarification list in the Operator portal (#4720, ADR 0184).
// Every read and every change is in the operator audit; only an operator
// session reaches the routes.
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/api/testutil/routetest"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken/tokentest"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	organizationCompose "github.com/moto-nrw/project-phoenix/modules/organizationtenancy/compose"
	provisioningAPI "github.com/moto-nrw/project-phoenix/modules/organizationtenancy/inbound/operator"
	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func TestOperatorRecordsBelegRetentionWithAudit(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	service, _ := buildOperatorProvisioningWithIdentity(t, db)
	operatorID := testpkg.CreateTestOperator(t, db).ID
	organizationID, _ := provisionTestSchool(t, db, service, operatorID, "traeger-belege")
	ctx := provisioningContext(t, db)
	operator := chi.NewRouter()
	mountOperatorBelegRetentionRoutes(operator, provisioningAPI.NewProvisioningResource(provisioningAPI.ProvisioningConfig{Service: service, BelegRetentionAudit: belegRetentionAuditOf(t, service)}))
	call := func(method, path string, body any) *httptest.ResponseRecorder {
		t.Helper()
		var payload bytes.Buffer
		if body != nil {
			require.NoError(t, json.NewEncoder(&payload).Encode(body))
		}
		req := httptest.NewRequest(method, path, &payload)
		req.Header.Set("Content-Type", "application/json")
		req = req.WithContext(context.WithValue(ctx, sessiontoken.CtxClaims, sessiontoken.AppClaims{ID: int(operatorID), Scope: "platform"}))
		rec := httptest.NewRecorder()
		operator.ServeHTTP(rec, req)
		return rec
	}
	audits := func(action string) int {
		var count int
		require.NoError(t, db.NewRaw(`SELECT count(*) FROM platform.operator_audit_log
			WHERE operator_id = ? AND resource_type = 'beleg_retention' AND resource_id = ? AND action = ?`,
			operatorID, organizationID, action).Scan(ctx, &count))
		return count
	}
	base := fmt.Sprintf("/organizations/%d/beleg-retention", organizationID)

	rec := call(http.MethodGet, base, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, 1, audits("view"), "the read is in the operator audit")

	invalid := map[string]any{"beleg_type": "care_contract", "trigger_event": "issued", "period": 6, "basis": "business_letter",
		"legal_basis": "§ 257 HGB", "instructed_by": "Kämmerei", "instructed_on": "2026-11-09", "instruction_reference": "AVV-Nachtrag"}
	rec = call(http.MethodPost, base+"/instructions", invalid)
	assert.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"code":"general.input"`)
	assert.Zero(t, audits("create"), "a refused change leaves no audit entry")

	valid := invalid
	valid["trigger_event"] = "contract_ended"
	rec = call(http.MethodPost, base+"/instructions", valid)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	valid["period"] = 8
	rec = call(http.MethodPost, base+"/instructions", valid)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	assert.Equal(t, 2, audits("create"))

	rec = call(http.MethodPost, base+"/suspensions", map[string]any{"beleg_type": "sepa_debit", "reason": "Betriebsprüfung",
		"scope": "alle Lastschriften 2026", "responsible": "Kämmerei", "review_on": "2099-01-01"})
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	suspension := decodeSchoolPortalData[parentbilling.RetentionSuspension](t, rec.Body.Bytes())

	rec = call(http.MethodGet, base, nil)
	require.Equal(t, http.StatusOK, rec.Code)
	overview := decodeSchoolPortalData[parentbilling.BelegRetentionOverview](t, rec.Body.Bytes())
	require.Len(t, overview.Instructions, 1, "a new instruction replaces the active one of its type")
	assert.Equal(t, 8, overview.Instructions[0].Period)
	require.Len(t, overview.Suspensions, 1)

	path := fmt.Sprintf("%s/suspensions/%d", base, suspension.ID)
	require.Equal(t, http.StatusNoContent, call(http.MethodDelete, path, nil).Code)
	assert.Equal(t, 1, audits("delete"))
	rec = call(http.MethodDelete, path, nil)
	assert.Equal(t, http.StatusNotFound, rec.Code, "a lifted suspension cannot be lifted again")

	short := map[string]any{"beleg_type": "care_invoice", "trigger_event": "issued", "period": 1, "basis": "booking_voucher",
		"legal_basis": "§ 147 AO", "instructed_by": "Kämmerei", "instructed_on": "2026-11-09", "instruction_reference": "AVV-Nachtrag"}
	rec = call(http.MethodPost, base+"/instructions", short)
	assert.Equal(t, http.StatusBadRequest, rec.Code, "an invoice under § 147 AO keeps eight years: %s", rec.Body.String())
	short["basis"], short["legal_basis"] = "not_applicable", "Satzung des Trägers"
	short["basis_justification"] = "Träger ist nicht buchführungspflichtig, Bescheid des Finanzamts"
	short["statutes_not_applicable_confirmed"] = true
	rec = call(http.MethodPost, base+"/instructions", short)
	require.Equal(t, http.StatusCreated, rec.Code, "a shorter period with the written reason why the statutes do not apply: %s", rec.Body.String())
	recorded := decodeSchoolPortalData[parentbilling.RetentionInstruction](t, rec.Body.Bytes())
	assert.Equal(t, parentbilling.BasisNotApplicable, recorded.Basis)
	assert.Equal(t, "Träger ist nicht buchführungspflichtig, Bescheid des Finanzamts", recorded.BasisJustification)

	assertConfirmationGatesTheShortPeriod(t, db, call, audits, base, operatorID)
}

// assertConfirmationGatesTheShortPeriod: below the statutory periods the
// structured confirmation of the Träger decides, not the text of the basis
// (#4720, fourth review). Without it no spelling passes; with it the
// instruction is recorded with the operator and time of the confirmation
// and audited. Whether a basis cites the statutes only the database decides,
// so the application and the database cannot disagree.
func assertConfirmationGatesTheShortPeriod(t *testing.T, db *bun.DB, call func(string, string, any) *httptest.ResponseRecorder,
	audits func(string) int, base string, operatorID int64,
) {
	t.Helper()
	ctx := context.Background()
	before := audits("create")
	short := map[string]any{"beleg_type": "sepa_debit", "trigger_event": "issued", "period": 1, "basis": "not_applicable",
		"basis_justification": "Träger ist nicht buchführungspflichtig", "instructed_by": "Frau Beispiel, Kämmerei",
		"instructed_on": "2026-11-09", "instruction_reference": "Mail der Kämmerei vom 09.11.2026"}
	for _, spelling := range []string{"Satzung des Trägers", "\u0410\u041e", "Abgabenordn.", "Abg.-Ord.", "Handelsges.", "§ einhundertsiebenundvierzig Abgabenordn."} {
		short["legal_basis"] = spelling
		rec := call(http.MethodPost, base+"/instructions", short)
		assert.Equal(t, http.StatusBadRequest, rec.Code, "%q without the confirmation: %s", spelling, rec.Body.String())
		assert.Contains(t, rec.Body.String(), `"code":"general.input"`, spelling)
	}
	assert.Equal(t, before, audits("create"), "a refused instruction leaves no audit entry")

	short["legal_basis"], short["statutes_not_applicable_confirmed"] = "\u0410\u041e", true
	rec := call(http.MethodPost, base+"/instructions", short)
	require.Equal(t, http.StatusCreated, rec.Code, "the confirmation carries the short period: %s", rec.Body.String())
	recorded := decodeSchoolPortalData[map[string]any](t, rec.Body.Bytes())
	assert.Equal(t, true, recorded["statutes_not_applicable_confirmed"])
	assert.NotEmpty(t, recorded["confirmed_at"])
	var confirmedBy int64
	require.NoError(t, db.NewRaw(`SELECT confirmed_by_operator_id FROM parent_billing.beleg_retention_instructions WHERE id = ?`,
		int64(recorded["id"].(float64))).Scan(ctx, &confirmedBy))
	assert.Equal(t, operatorID, confirmedBy, "the confirmation names the operator who recorded it")
	assert.Equal(t, before+1, audits("create"), "the confirmed instruction is audited")

	short["legal_basis"] = "§ 147 AO"
	rec = call(http.MethodPost, base+"/instructions", short)
	assert.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"code":"parent_billing.beleg_retention_basis_cites_statute"`)

	// The outlined letters U+1CCD6 U+1CCE4 ("AO") split the earlier Go and
	// SQL folding; now the database alone answers for both.
	outlined := "\U0001CCD6\U0001CCE4"
	var cites bool
	require.NoError(t, db.NewRaw(`SELECT parent_billing.beleg_basis_cites_statute(?)`, outlined).Scan(ctx, &cites))
	short["legal_basis"] = outlined
	rec = call(http.MethodPost, base+"/instructions", short)
	if cites {
		assert.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
		assert.Contains(t, rec.Body.String(), `"code":"parent_billing.beleg_retention_basis_cites_statute"`)
	} else {
		assert.Equal(t, http.StatusCreated, rec.Code, "the application accepts what the database accepts: %s", rec.Body.String())
	}
}

// TestOperatorRecordsTheReviewOfAClarificationEntry: the moto team notes
// what it clarified with the Träger, escalates to Florian with a reason and
// sets the next review (Löschkonzept section 3); the overview shows the
// review and the entry is no longer overdue. Every review is audited.
func TestOperatorRecordsTheReviewOfAClarificationEntry(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	service, _ := buildOperatorProvisioningWithIdentity(t, db)
	operatorID := testpkg.CreateTestOperator(t, db).ID
	organizationID, _ := provisionTestSchool(t, db, service, operatorID, "traeger-klaerung")
	ctx := provisioningContext(t, db)
	operator := chi.NewRouter()
	mountOperatorBelegRetentionRoutes(operator, provisioningAPI.NewProvisioningResource(provisioningAPI.ProvisioningConfig{Service: service, BelegRetentionAudit: belegRetentionAuditOf(t, service)}))
	call := func(method, path string, body any) *httptest.ResponseRecorder {
		t.Helper()
		var payload bytes.Buffer
		if body != nil {
			require.NoError(t, json.NewEncoder(&payload).Encode(body))
		}
		req := httptest.NewRequest(method, path, &payload)
		req.Header.Set("Content-Type", "application/json")
		req = req.WithContext(context.WithValue(ctx, sessiontoken.CtxClaims, sessiontoken.AppClaims{ID: int(operatorID), Scope: "platform"}))
		rec := httptest.NewRecorder()
		operator.ServeHTTP(rec, req)
		return rec
	}
	// Two contracts of the Träger have waited on the list since January,
	// so their review is overdue.
	_, err := db.ExecContext(ctx, `INSERT INTO parent_billing.beleg_retentions (organization_id, beleg_type, beleg_id, state, reason,
			listed_on, review_on, responsible, evaluated_on)
		VALUES (?0, 'care_contract', 1, 'clarification', 'no_instruction', '2026-01-02', '2026-02-01', ?1, CURRENT_DATE),
			(?0, 'care_contract', 2, 'clarification', 'no_instruction', '2026-01-02', '2026-02-01', ?1, CURRENT_DATE)`,
		organizationID, parentbilling.ClarificationOwner)
	require.NoError(t, err)
	base := fmt.Sprintf("/organizations/%d/beleg-retention", organizationID)
	next := calendar.TodayDate().AddDays(30).String()
	review := map[string]any{"beleg_type": "care_contract", "reason": "no_instruction", "note": "Träger klärt die Einordnung der Verträge",
		"escalated": true, "next_review_on": next}

	rec := call(http.MethodPost, base+"/clarification-reviews", review)
	assert.Equal(t, http.StatusBadRequest, rec.Code, "an escalation names its reason: %s", rec.Body.String())
	review["escalation_reason"] = "Keine Antwort des Trägers seit 30 Tagen"
	rec = call(http.MethodPost, base+"/clarification-reviews", map[string]any{"beleg_type": "sepa_mandate", "reason": "no_collection",
		"note": "nichts gelistet", "next_review_on": next})
	assert.Equal(t, http.StatusNotFound, rec.Code, "only a listed entry can be reviewed: %s", rec.Body.String())
	rec = call(http.MethodPost, base+"/clarification-reviews", review)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	recorded := decodeSchoolPortalData[parentbilling.BelegClarificationReview](t, rec.Body.Bytes())

	var audited int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM platform.operator_audit_log
		WHERE operator_id = ? AND resource_type = 'beleg_retention' AND resource_id = ? AND action = 'create'
			AND changes->>'kind' = 'clarification_review'`, operatorID, organizationID).Scan(ctx, &audited))
	assert.Equal(t, 1, audited, "the review is in the operator audit, a refused one is not")

	rec = call(http.MethodGet, base, nil)
	require.Equal(t, http.StatusOK, rec.Code)
	overview := decodeSchoolPortalData[parentbilling.BelegRetentionOverview](t, rec.Body.Bytes())
	require.Len(t, overview.Clarifications, 1)
	entry := overview.Clarifications[0]
	assert.Zero(t, entry.Overdue, "the next review moved, the entry is no longer overdue")
	assert.Equal(t, recorded.NextReviewOn, entry.ReviewOn, "the entry's review moves to the recorded next review")
	require.NotNil(t, entry.LastReview)
	assert.True(t, entry.LastReview.Escalated)
	assert.Equal(t, "Keine Antwort des Trägers seit 30 Tagen", entry.LastReview.EscalationReason)
	assert.Equal(t, "Träger klärt die Einordnung der Verträge", entry.LastReview.Note)
	assert.Equal(t, recorded.NextReviewOn, entry.LastReview.NextReviewOn)
	assert.Equal(t, recorded.ID, entry.LastReview.ID)
}

// TestClarificationReviewEscalatesBeforeTheReviewMoves: an entry still
// open at its review goes to Florian with a reason before its review date
// moves (Löschkonzept section 3, #4720 third review). An overdue entry used
// to lose its overdue status by a new date alone; now that is 400, nothing
// is stored or audited and the entry stays overdue. Before the deadline a
// review without escalation may keep or advance the date, not push it out.
func TestClarificationReviewEscalatesBeforeTheReviewMoves(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	service, _ := buildOperatorProvisioningWithIdentity(t, db)
	operatorID := testpkg.CreateTestOperator(t, db).ID
	organizationID, _ := provisionTestSchool(t, db, service, operatorID, "traeger-eskalation")
	ctx := provisioningContext(t, db)
	operator := chi.NewRouter()
	mountOperatorBelegRetentionRoutes(operator, provisioningAPI.NewProvisioningResource(provisioningAPI.ProvisioningConfig{Service: service, BelegRetentionAudit: belegRetentionAuditOf(t, service)}))
	call := func(body any) *httptest.ResponseRecorder {
		t.Helper()
		var payload bytes.Buffer
		require.NoError(t, json.NewEncoder(&payload).Encode(body))
		req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/organizations/%d/beleg-retention/clarification-reviews", organizationID), &payload)
		req.Header.Set("Content-Type", "application/json")
		req = req.WithContext(context.WithValue(ctx, sessiontoken.CtxClaims, sessiontoken.AppClaims{ID: int(operatorID), Scope: "platform"}))
		rec := httptest.NewRecorder()
		operator.ServeHTTP(rec, req)
		return rec
	}
	today := calendar.TodayDate()
	// Contracts listed 40 days ago are past their review; notices listed 20
	// days ago are due in ten days.
	_, err := db.ExecContext(ctx, `INSERT INTO parent_billing.beleg_retentions (organization_id, beleg_type, beleg_id, state, reason,
			listed_on, review_on, responsible, evaluated_on)
		VALUES (?0, 'care_contract', 1, 'clarification', 'no_instruction', ?1, ?2, ?5, ?6),
			(?0, 'care_notice', 2, 'clarification', 'no_instruction', ?3, ?4, ?5, ?6)`,
		organizationID, today.AddDays(-40), today.AddDays(-10), today.AddDays(-20), today.AddDays(10), parentbilling.ClarificationOwner, today)
	require.NoError(t, err)
	reviewOn := func(belegType string) calendar.Date {
		t.Helper()
		var day calendar.Date
		require.NoError(t, db.NewRaw(`SELECT review_on FROM parent_billing.beleg_retentions WHERE organization_id = ? AND beleg_type = ?`,
			organizationID, belegType).Scan(ctx, &day))
		return day
	}
	reviews := func() int {
		t.Helper()
		var count int
		require.NoError(t, db.NewRaw(`SELECT count(*) FROM parent_billing.beleg_clarification_reviews WHERE organization_id = ?`,
			organizationID).Scan(ctx, &count))
		return count
	}
	review := func(belegType string, next calendar.Date, escalation string) map[string]any {
		return map[string]any{"beleg_type": belegType, "reason": "no_instruction", "note": "Träger hat noch nicht geantwortet",
			"escalated": escalation != "", "escalation_reason": escalation, "next_review_on": next.String()}
	}

	overdueOn, dueOn := reviewOn("care_contract"), reviewOn("care_notice")
	recordedNext := func(rec *httptest.ResponseRecorder) calendar.Date {
		t.Helper()
		return decodeSchoolPortalData[parentbilling.BelegClarificationReview](t, rec.Body.Bytes()).NextReviewOn
	}

	rec := call(review("care_contract", today.AddDays(30), ""))
	assert.Equal(t, http.StatusBadRequest, rec.Code, "an overdue entry still open goes to Florian: %s", rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"code":"general.input"`)
	assert.Equal(t, overdueOn, reviewOn("care_contract"), "the entry stays overdue")
	assert.Zero(t, reviews(), "a refused review is not stored")

	rec = call(review("care_notice", today.AddDays(20), ""))
	assert.Equal(t, http.StatusBadRequest, rec.Code, "a review before the deadline cannot push it out without escalation: %s", rec.Body.String())
	assert.Equal(t, dueOn, reviewOn("care_notice"))
	rec = call(review("care_notice", today.AddDays(5), ""))
	require.Equal(t, http.StatusCreated, rec.Code, "a review may advance the deadline: %s", rec.Body.String())
	assert.True(t, recordedNext(rec).Before(dueOn), "the review advanced the deadline")
	assert.Equal(t, recordedNext(rec), reviewOn("care_notice"))

	rec = call(review("care_contract", today.AddDays(30), "Keine Antwort des Trägers seit 40 Tagen"))
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	assert.Equal(t, recordedNext(rec), reviewOn("care_contract"), "with the escalation the review moves")
	assert.True(t, reviewOn("care_contract").After(overdueOn))
	assert.Equal(t, 2, reviews())
}

// TestBelegRetentionRoutesRefuseASchoolSession: the routes sit in the
// operator chain, so a tenant token gets 403 before any audit or read.
func TestBelegRetentionRoutesRefuseASchoolSession(t *testing.T) {
	t.Parallel()
	authService := &mockOperatorAuthService{}
	provisioningService := &protectedRouteProvisioningService{}
	router := testOperatorRouter(operatorIdentity{access: authService, sessions: operatorTestSessions(t, authService)}, provisioningService)
	token, err := tokentest.CreateJWT(routetest.TestTokenAuth(t), routetest.Claims{
		ID: 42, Sub: "school-user", Roles: []string{"admin"}, Scope: "tenant", FamilyID: "school-family",
	})
	require.NoError(t, err)
	for _, method := range []string{http.MethodGet, http.MethodPost} {
		path := "/organizations/7/beleg-retention"
		if method == http.MethodPost {
			path += "/instructions"
		}
		req := httptest.NewRequest(method, path, bytes.NewBufferString(`{}`))
		req.Header.Set("Authorization", "Bearer "+token)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusForbidden, rr.Code, "%s %s", method, rr.Body.String())
	}
	assert.Empty(t, provisioningService.reads, "no audit and no read for a school session")
}

func belegRetentionAuditOf(t *testing.T, provisioning organizationtenancy.OperatorProvisioning) organizationtenancy.BelegRetentionChangeAudit {
	t.Helper()
	audit, err := organizationCompose.BelegRetentionAudit(provisioning.Provisioning)
	require.NoError(t, err)
	return audit
}
