// Tests of the conclusion at the acceptance (Weg b, #4680, ADR 0088 section
// 6): the setting of the Träger with a school override decides, the run
// creates, sends and concludes the contract in one Träger transaction, a
// repeated run concludes nothing twice, and a contract that cannot be sent
// leaves no half-made row behind.
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	auditCompose "github.com/moto-nrw/project-phoenix/modules/auditlog/compose"
	"github.com/moto-nrw/project-phoenix/modules/carecontract"
	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
	deliveryContract "github.com/moto-nrw/project-phoenix/modules/delivery/contract"
	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
	owners "github.com/moto-nrw/project-phoenix/roots/owners"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	querybudgets "github.com/moto-nrw/project-phoenix/test/query-budgets"
)

// careContractConclusionRun runs the Worker's conclusion over the same
// ports the serving root binds, for the fixture's Träger.
func careContractConclusionRun(t *testing.T, f careContractFixture) carecontract.ConclusionRun {
	t.Helper()
	database := testpkg.NewDatabase(t, f.db)
	unit := testpkg.TenantRuntime(t, f.db)
	organizations, err := owners.NewOrganizationTenancy(database)
	require.NoError(t, err)
	audit, err := auditCompose.NewTenantPersistence(f.db)
	require.NoError(t, err)
	settings, err := owners.BuildTraegerSettings(database, unit, slog.Default(), organizations, nil)
	require.NoError(t, err)
	delivery, err := deliveryCompose.NewModule(deliveryCompose.ModuleDependencies{
		DB: f.db, People: owners.EnrollmentFlowGuardianDirectory{}, Observe: func(deliveryContract.Observation) {},
	})
	require.NoError(t, err)
	schoolSettings, err := owners.NewSettingsTestModule(database, unit)
	require.NoError(t, err)
	conclusion := NewCareContractConclusion(organizations, settings, schoolSettings.Settings, delivery.Module, audit.DataAccessLog)
	run, err := conclusion.ConcludeAccepted(tenant.WithUnitOfWork(context.Background(), unit), f.office.organization)
	require.NoError(t, err)
	return run
}

func setTraegerConclusionMode(t *testing.T, f careContractFixture, mode string) {
	t.Helper()
	_, err := f.db.ExecContext(context.Background(), `INSERT INTO config.traeger_setting_values (organization_id, setting_key, value)
		VALUES (?, 'care_contract.conclusion_mode', to_jsonb(?::text))
		ON CONFLICT (organization_id, setting_key) DO UPDATE SET value = EXCLUDED.value`, f.office.organization, mode)
	require.NoError(t, err)
}

func setSchoolConclusionMode(t *testing.T, f careContractFixture, mode string) {
	t.Helper()
	ctx := context.Background()
	_, err := f.db.ExecContext(ctx, `DELETE FROM config.setting_values WHERE tenant_id = ? AND setting_key = 'care_contract.conclusion_mode'`, f.school)
	require.NoError(t, err)
	if mode == "" {
		return
	}
	_, err = f.db.ExecContext(ctx, `INSERT INTO config.setting_values (tenant_id, setting_key, value)
		VALUES (?, 'care_contract.conclusion_mode', to_jsonb(?::text))`, f.school, mode)
	require.NoError(t, err)
}

func careContractRows(t *testing.T, f careContractFixture) int {
	t.Helper()
	var count int
	require.NoError(t, f.db.NewRaw(`SELECT COUNT(*) FROM care_contract.contracts WHERE organization_id = ?`,
		f.office.organization).Scan(context.Background(), &count))
	return count
}

func TestCareContractIsConcludedAtTheAcceptanceWhereTheSchoolSaysSo(t *testing.T) {
	t.Parallel()
	f := newCareContractFixture(t, "Träger Vertrag Abschluss")
	token := f.office.token
	require.Equal(t, http.StatusOK, traegerJSON(t, f.traeger, token, http.MethodPut, templatePath(f.school, f.offering), careContractTemplate).Code)

	// Weg a is the default: the run leaves the child to the Träger-Büro.
	run := careContractConclusionRun(t, f)
	assert.Empty(t, run.Concluded)
	assert.Zero(t, careContractRows(t, f))

	// The Träger concludes at the acceptance; the school keeps Weg a.
	setTraegerConclusionMode(t, f, carecontract.ModeOnAcceptance)
	setSchoolConclusionMode(t, f, carecontract.ModeSendLater)
	assert.Empty(t, careContractConclusionRun(t, f).Concluded, "the school's own value wins over the Träger's")
	assert.Zero(t, careContractRows(t, f))

	// Without the school's own value the Träger's holds.
	setSchoolConclusionMode(t, f, "")
	accepted := recordCareContractAcceptance(t, f)
	run = careContractConclusionRun(t, f)
	require.Len(t, run.Concluded, 1)
	assert.Empty(t, run.Skipped)
	id := run.Concluded[0]

	rec := traegerJSON(t, f.traeger, token, http.MethodGet, contractPath(id, ""), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	detail := decodeSchoolPortalData[struct {
		careContractDetailBody
		Concluded *struct {
			Method string `json:"method"`
			At     string `json:"at"`
		} `json:"concluded"`
	}](t, rec.Body.Bytes())
	assert.Equal(t, "aktiv", detail.Status)
	assert.Equal(t, "wirksam", detail.Version.Status)
	require.NotNil(t, detail.Concluded)
	assert.Equal(t, carecontract.MethodWithApplication, detail.Concluded.Method)
	acceptedAt, err := time.Parse(time.RFC3339Nano, detail.Concluded.At)
	require.NoError(t, err)
	assert.WithinDuration(t, accepted, acceptedAt, time.Millisecond, "the conclusion keeps the instant of the school's acceptance")
	assert.True(t, detail.DocumentFrozen, "the parents received the frozen PDF")
	require.Len(t, detail.Mailings, 1)
	assert.Equal(t, "erika.muster@example.org", detail.Mailings[0].Recipient)

	// The mail says that the contract is concluded and carries the PDF.
	var mails []struct {
		Payload json.RawMessage `bun:"payload"`
	}
	require.NoError(t, f.db.NewRaw(`SELECT payload FROM platform.email_outbox WHERE organization_id = ? AND kind = 'care_contract'
		AND related_entity_id = ?`, f.office.organization, id).Scan(context.Background(), &mails))
	require.Len(t, mails, 1)
	var mail carecontract.ContractMail
	require.NoError(t, json.Unmarshal(mails[0].Payload, &mail))
	assert.True(t, mail.Concluded)
	assert.True(t, bytes.HasPrefix(mail.PDF, []byte("%PDF")))

	// The trail records the creation and the conclusion without an account.
	var events []struct {
		Action    string `bun:"action"`
		AccountID *int64 `bun:"account_id"`
		Method    string `bun:"method"`
	}
	require.NoError(t, f.db.NewRaw(`SELECT action, account_id, COALESCE(details->>'method', '') AS method
		FROM care_contract.contract_events WHERE organization_id = ? AND contract_id = ? AND action IN ('create', 'send', 'conclude')
		ORDER BY id`, f.office.organization, id).Scan(context.Background(), &events))
	require.Len(t, events, 3)
	assert.Equal(t, []string{"create", "send", "conclude"}, []string{events[0].Action, events[1].Action, events[2].Action})
	assert.Equal(t, carecontract.MethodWithApplication, events[2].Method)
	for _, event := range events {
		assert.Nil(t, event.AccountID, "the automatic run acts for no account")
	}

	// A second run concludes nothing twice, and the child is no candidate.
	again := careContractConclusionRun(t, f)
	assert.Empty(t, again.Concluded)
	assert.Equal(t, 1, careContractRows(t, f))
	rec = traegerJSON(t, f.traeger, token, http.MethodGet, "/traeger/contracts/candidates", nil)
	assert.Empty(t, decodeSchoolPortalData[[]careContractCandidateBody](t, rec.Body.Bytes()))

	// An active contract cannot be discarded; a pending mailing still
	// prevents duplicate sends.
	rec = traegerJSON(t, f.traeger, token, http.MethodPost, "/traeger/contracts/send", map[string]any{"contract_ids": []int64{id}})
	require.Equal(t, http.StatusAccepted, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "mail_pending")
	// A terminal failure can be corrected without changing the conclusion or PDF.
	pdf := traegerJSON(t, f.traeger, token, http.MethodGet, contractPath(id, "/document"), nil)
	require.Equal(t, http.StatusOK, pdf.Code, pdf.Body.String())
	beforePDF := pdf.Body.Bytes()
	assert.True(t, bytes.HasPrefix(beforePDF, []byte("%PDF")))
	_, err = f.db.ExecContext(context.Background(), `UPDATE platform.email_outbox SET status = 'dead_letter', dead_letter_at = NOW(), last_error = 'recipient rejected'
  WHERE organization_id = ? AND kind = 'care_contract' AND related_entity_id = ?`, f.office.organization, id)
	require.NoError(t, err)
	rec = traegerJSON(t, f.traeger, token, http.MethodPost, "/traeger/contracts/send", map[string]any{"contract_ids": []int64{id}, "recipient_email": "korrigiert@example.org"})
	require.Equal(t, http.StatusAccepted, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), fmt.Sprintf(`"sent":[%d]`, id))
	afterPDF := traegerJSON(t, f.traeger, token, http.MethodGet, contractPath(id, "/document"), nil).Body.Bytes()
	assert.Equal(t, beforePDF, afterPDF)
	rec = traegerJSON(t, f.traeger, token, http.MethodGet, contractPath(id, ""), nil)
	after := decodeSchoolPortalData[struct {
		careContractDetailBody
		Concluded *carecontract.Concluded `json:"concluded"`
	}](t, rec.Body.Bytes())
	assert.Equal(t, "aktiv", after.Status)
	require.NotNil(t, after.Concluded)
	assert.WithinDuration(t, accepted, after.Concluded.At, time.Millisecond, "a corrected send keeps the conclusion")
	require.Len(t, after.Mailings, 2)
	assert.Equal(t, "korrigiert@example.org", after.Mailings[0].Recipient)
	rec = traegerJSON(t, f.traeger, token, http.MethodDelete, contractPath(id, ""), nil)
	assert.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
	rec = traegerJSON(t, f.traeger, token, http.MethodGet, "/traeger/contracts?status=aktiv", nil)
	assert.Contains(t, rec.Body.String(), `"aktiv":1`)
}

func TestCareContractConclusionHoldsBackAChildItCannotConcludeYet(t *testing.T) {
	t.Parallel()
	f := newCareContractFixture(t, "Träger Vertrag Zurückhalten")
	setTraegerConclusionMode(t, f, carecontract.ModeOnAcceptance)

	// Without conditions for the module there is no contract.
	run := careContractConclusionRun(t, f)
	assert.Empty(t, run.Concluded)
	require.Len(t, run.Skipped, 1)
	assert.Equal(t, carecontract.SkipTemplateMissing, run.Skipped[0].Reason)
	assert.Zero(t, careContractRows(t, f))

	// Without the parents' address the contract cannot reach them: the
	// whole Träger transaction rolls back, so no draft blocks a later run.
	require.Equal(t, http.StatusOK, traegerJSON(t, f.traeger, f.office.token, http.MethodPut, templatePath(f.school, f.offering), careContractTemplate).Code)
	recordCareContractAcceptance(t, f)
	_, err := f.db.ExecContext(context.Background(), `UPDATE enrollment.requests SET guardian_email = ''
		WHERE id = (SELECT request_id FROM enrollment.request_children WHERE id = ?)`, f.child)
	require.NoError(t, err)
	run = careContractConclusionRun(t, f)
	assert.Empty(t, run.Concluded)
	require.Len(t, run.Skipped, 1)
	assert.Equal(t, carecontract.SkipRecipientMissing, run.Skipped[0].Reason)
	assert.Zero(t, careContractRows(t, f))
	var outbox int
	require.NoError(t, f.db.NewRaw(`SELECT COUNT(*) FROM platform.email_outbox WHERE organization_id = ? AND kind = 'care_contract'`,
		f.office.organization).Scan(context.Background(), &outbox))
	assert.Zero(t, outbox)

	// Once the address is there, the next run concludes it.
	_, err = f.db.ExecContext(context.Background(), `UPDATE enrollment.requests SET guardian_email = 'erika.muster@example.org'
		WHERE id = (SELECT request_id FROM enrollment.request_children WHERE id = ?)`, f.child)
	require.NoError(t, err)
	assert.Len(t, careContractConclusionRun(t, f).Concluded, 1)
}

func TestCareContractConclusionModesNameTheWayOfEverySchool(t *testing.T) {
	t.Parallel()
	f := newCareContractFixture(t, "Träger Vertrag Abschlussweg")
	token := f.office.token
	setTraegerConclusionMode(t, f, carecontract.ModeOnAcceptance)
	setSchoolConclusionMode(t, f, carecontract.ModeSendLater)

	counter := testpkg.CaptureQueriesForContext(t, f.db)
	req := httptest.NewRequest(http.MethodGet, "/traeger/contracts/conclusion-modes", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	f.traeger.ServeHTTP(rec, req.WithContext(counter.Context(req.Context())))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	modes := map[int64]string{}
	for _, school := range decodeSchoolPortalData[[]carecontract.SchoolConclusion](t, rec.Body.Bytes()) {
		modes[school.SchoolID] = school.Mode
	}
	require.Len(t, modes, len(f.office.schools))
	for _, school := range f.office.schools {
		want := carecontract.ModeOnAcceptance
		if school == f.school {
			want = carecontract.ModeSendLater
		}
		assert.Equal(t, want, modes[school], "school %d: its own value, else the Träger's", school)
	}
	querybudgets.AssertQueryBudget(t, "api.traeger.care_contract_conclusion_modes", counter.Selects(""))

	// Without the right "Verträge verwalten" the route answers 403.
	billing := traegerColleague(t, f.reviewFixture, identityaccess.TraegerRoleBilling)
	rec = traegerJSON(t, f.traeger, billing, http.MethodGet, "/traeger/contracts/conclusion-modes", nil)
	assert.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
}

func TestCareContractConclusionRequiresTheTickedConditions(t *testing.T) {
	t.Parallel()
	f := newCareContractFixture(t, "Träger Vertrag Nachweis")
	require.Equal(t, http.StatusOK, traegerJSON(t, f.traeger, f.office.token, http.MethodPut, templatePath(f.school, f.offering), careContractTemplate).Code)
	setTraegerConclusionMode(t, f, carecontract.ModeOnAcceptance)
	assert.Equal(t, carecontract.SkipAcceptanceMissing, candidateAcceptanceIssue(t, f))
	run := careContractConclusionRun(t, f)
	assert.Empty(t, run.Concluded, "approval alone is no proof of agreement to the current contract")
	require.Len(t, run.Skipped, 1)
	assert.Equal(t, "acceptance_missing", run.Skipped[0].Reason)
	assert.Zero(t, careContractRows(t, f))
	recordCareContractAcceptance(t, f)
	assert.Empty(t, candidateAcceptanceIssue(t, f), "ticked conditions before the acceptance are proof")
	_, err := f.db.ExecContext(context.Background(), `UPDATE enrollment.requests
  SET legal_blocks_snapshot = jsonb_set(legal_blocks_snapshot, '{0,blocks,0,text_sha256}', '"invalid"'::jsonb)
  WHERE id = (SELECT request_id FROM enrollment.request_children WHERE id = ?)`, f.child)
	require.NoError(t, err)
	run = careContractConclusionRun(t, f)
	assert.Empty(t, run.Concluded, "an invalid checksum cannot establish agreement")
	require.Len(t, run.Skipped, 1)
	assert.Equal(t, carecontract.SkipAcceptanceMissing, run.Skipped[0].Reason)
	assert.Zero(t, careContractRows(t, f))
}

func TestCareContractAutomaticModeRefusesManualDrafts(t *testing.T) {
	t.Parallel()
	f := newCareContractFixture(t, "Träger Vertrag Keine Entwürfe")
	require.Equal(t, http.StatusOK, traegerJSON(t, f.traeger, f.office.token, http.MethodPut, templatePath(f.school, f.offering), careContractTemplate).Code)
	setTraegerConclusionMode(t, f, carecontract.ModeOnAcceptance)
	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodPost, "/traeger/contracts", map[string]any{"contracts": []any{f.key()}})
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	result := decodeSchoolPortalData[careContractCreateBody](t, rec.Body.Bytes())
	assert.Empty(t, result.Created)
	assert.Contains(t, rec.Body.String(), "conclusion_automatic")
	assert.Zero(t, careContractRows(t, f))
}

// recordCareContractAcceptance arranges a parent submission whose required
// conditions block was ticked, with the checksum taken when it was shown,
// after the conditions were saved; the school accepted it an hour later. The
// instants come from the database clock, which also stamps the templates.
// It returns the instant of the acceptance.
func recordCareContractAcceptance(t *testing.T, f careContractFixture) time.Time {
	t.Helper()
	ctx := context.Background()
	var submitted time.Time
	require.NoError(t, f.db.NewRaw(`SELECT NOW()`).Scan(ctx, &submitted))
	accepted := submitted.Add(time.Hour)
	text := "Es gelten die Teilnahmebedingungen des Trägers mit Laufzeit, Kündigungsfrist und Satzung."
	snapshot, err := json.Marshal([]enrollment.LegalBlocksSnapshotEntry{{
		SnapshotAt: submitted, ConsentFlags: json.RawMessage(`{"terms":true}`),
		Blocks: []enrollment.LegalBlockSnapshot{{Key: "terms", Kind: enrollment.LegalBlockKindTerms,
			Required: true, Text: text, TextSHA256: securityruntime.Fingerprint([]byte(text))}},
	}})
	require.NoError(t, err)
	_, err = f.db.ExecContext(ctx, `UPDATE enrollment.requests SET legal_blocks_snapshot = ?::jsonb, submission_source = 'public'
  WHERE id = (SELECT request_id FROM enrollment.request_children WHERE id = ?)`, string(snapshot), f.child)
	require.NoError(t, err)
	_, err = f.db.ExecContext(ctx, `UPDATE enrollment.request_children SET reviewed_at = ? WHERE id = ?`, accepted, f.child)
	require.NoError(t, err)
	return accepted
}

func TestCareContractConclusionRefusesChangedConditions(t *testing.T) {
	t.Parallel()
	f := newCareContractFixture(t, "Träger Vertrag Geänderte Bedingungen")
	require.Equal(t, http.StatusOK, traegerJSON(t, f.traeger, f.office.token, http.MethodPut, templatePath(f.school, f.offering), careContractTemplate).Code)
	recordCareContractAcceptance(t, f)
	changed := map[string]any{}
	for key, value := range careContractTemplate {
		changed[key] = value
	}
	changed["notice_months"] = 3
	require.Equal(t, http.StatusOK, traegerJSON(t, f.traeger, f.office.token, http.MethodPut, templatePath(f.school, f.offering), changed).Code)
	setTraegerConclusionMode(t, f, carecontract.ModeOnAcceptance)
	assert.Equal(t, carecontract.SkipAcceptanceChanged, candidateAcceptanceIssue(t, f),
		"the office sees why the contract does not come about")
	run := careContractConclusionRun(t, f)
	assert.Empty(t, run.Concluded)
	require.Len(t, run.Skipped, 1)
	assert.Equal(t, carecontract.SkipAcceptanceChanged, run.Skipped[0].Reason)
	assert.Zero(t, careContractRows(t, f), "no draft or mail survives the rejected evidence")
}

// candidateAcceptanceIssue reads the proof state the candidates list gives
// for the fixture's child.
func candidateAcceptanceIssue(t *testing.T, f careContractFixture) string {
	t.Helper()
	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodGet, "/traeger/contracts/candidates", nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	candidates := decodeSchoolPortalData[[]carecontract.Candidate](t, rec.Body.Bytes())
	require.Len(t, candidates, 1)
	return candidates[0].AcceptanceIssue
}
