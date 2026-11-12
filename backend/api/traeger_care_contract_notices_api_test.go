package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-ogs/project-phoenix/modules/carecontract"
	"github.com/moto-ogs/project-phoenix/modules/enrollment"
	"github.com/moto-ogs/project-phoenix/modules/identityaccess"
	"github.com/moto-ogs/project-phoenix/modules/securityruntime"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-ogs/project-phoenix/test"
)

// noticeOptionsBody is the answer of /traeger/contract-notices/{id}.
type noticeOptionsBody struct {
	Status              string         `json:"status"`
	ReinterpretedEndsOn *calendar.Date `json:"reinterpreted_ends_on"`
	OrdinaryEndsOn      *calendar.Date `json:"ordinary_ends_on"`
	WithdrawalUntil     *calendar.Date `json:"withdrawal_until"`
	EarlyStart          bool           `json:"early_start"`
	Notice              *struct {
		Kind             string         `json:"kind"`
		Channel          string         `json:"channel"`
		EndsOn           *calendar.Date `json:"ends_on"`
		TerminationKind  string         `json:"termination_kind"`
		Reason           string         `json:"reason"`
		ConfirmationText string         `json:"confirmation_text"`
		EndDecision      string         `json:"end_decision"`
		EndNoticeText    string         `json:"end_notice_text"`
		EndNoticeSent    bool           `json:"end_notice_sent"`
	} `json:"notice"`
}

// concludeWithEarlyStart concludes the fixture's contract at the
// acceptance (Weg b): the parents ticked the conditions and the early start
// in the application. It answers the contract.
func concludeWithEarlyStart(t *testing.T, f careContractFixture) int64 {
	t.Helper()
	ctx := context.Background()
	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodGet, "/traeger/contract-templates", nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var conditions, earlyStart string
	for _, offering := range decodeSchoolPortalData[[]carecontract.OfferingTemplate](t, rec.Body.Bytes()) {
		if offering.SchoolID == f.school && offering.OfferingID == f.offering {
			conditions, earlyStart = offering.AcceptanceText, offering.EarlyStartText
			assert.Contains(t, offering.WithdrawalText, "Widerrufsbelehrung", "the school copies the notice into the form")
		}
	}
	require.NotEmpty(t, conditions)
	require.NotEmpty(t, earlyStart)
	var submitted time.Time
	require.NoError(t, f.db.NewRaw(`SELECT NOW()`).Scan(ctx, &submitted))
	// As the form editor saves them: a required own block as terms, an
	// optional one as consent.
	block := func(key, text string, required bool) enrollment.LegalBlockSnapshot {
		kind := enrollment.LegalBlockKindConsent
		if required {
			kind = enrollment.LegalBlockKindTerms
		}
		return enrollment.LegalBlockSnapshot{Key: key, Kind: kind, Source: enrollment.LegalBlockSourceCustom,
			Required: required, Text: text, TextSHA256: securityruntime.Fingerprint([]byte(text))}
	}
	snapshot, err := json.Marshal([]enrollment.LegalBlocksSnapshotEntry{{
		SnapshotAt: submitted, ConsentFlags: json.RawMessage(`{"terms":true,"early_start":true}`), FormSchemaID: 81, DisplayedFormSchemaID: 81,
		Blocks: []enrollment.LegalBlockSnapshot{block("terms", conditions, true), block("early_start", earlyStart, false)},
	}})
	require.NoError(t, err)
	_, err = f.db.ExecContext(ctx, `UPDATE enrollment.requests SET legal_blocks_snapshot = ?::jsonb, submission_source = 'public'
		WHERE id = (SELECT request_id FROM enrollment.request_children WHERE id = ?)`, string(snapshot), f.child)
	require.NoError(t, err)
	_, err = f.db.ExecContext(ctx, `UPDATE enrollment.request_children SET reviewed_at = ? WHERE id = ?`, submitted.Add(time.Second), f.child)
	require.NoError(t, err)
	setTraegerConclusionMode(t, f, carecontract.ModeOnAcceptance)
	run := careContractConclusionRun(t, f)
	require.Len(t, run.Concluded, 1, "skipped: %v", run.Skipped)
	return run.Concluded[0]
}

// The Träger-Büro records a termination it received (#4682): the right
// "Verträge verwalten" is required, another Träger sees nothing, every read
// and the declaration land in the trail and the read in the school's access
// log, and the confirmation goes out as a mail of the Träger.
func TestCareContractNoticeNeedsTheRightStaysWithItsTraegerAndIsAudited(t *testing.T) {
	t.Parallel()
	f := newCareContractFixture(t, "Träger Vertrag Kündigung")
	token := f.office.token
	require.Equal(t, http.StatusOK, traegerJSON(t, f.traeger, token, http.MethodPut, templatePath(f.school, f.offering), careContractTemplate).Code)
	contract := concludeWithEarlyStart(t, f)
	path := "/traeger/contract-notices/" + strconv.FormatInt(contract, 10)
	ctx := context.Background()

	var informed bool
	var concludedOn calendar.Date
	require.NoError(t, f.db.NewRaw(`SELECT withdrawal_informed_at IS NOT NULL AS informed,
			(concluded_at AT TIME ZONE 'Europe/Berlin')::date AS concluded_on FROM care_contract.contracts WHERE id = ?`, contract).
		Scan(ctx, &informed, &concludedOn))
	assert.False(t, informed, "queueing alone proves no notice provision")
	_, err := f.db.ExecContext(ctx, `UPDATE platform.email_outbox AS o SET status = 'sent', sent_at = c.concluded_at,
      delivery_state = 'delivered', delivery_state_at = c.concluded_at
      FROM care_contract.contracts AS c WHERE o.organization_id = c.organization_id
        AND o.related_entity_type = 'care_contract' AND o.related_entity_id = c.id AND c.id = ?`, contract)
	require.NoError(t, err)

	rec := traegerJSON(t, f.traeger, token, http.MethodGet, path, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	options := decodeSchoolPortalData[noticeOptionsBody](t, rec.Body.Bytes())
	assert.Equal(t, carecontract.StatusActive, options.Status)
	assert.True(t, options.EarlyStart, "the ticked early start is stored with the agreement")
	require.NotNil(t, options.WithdrawalUntil)
	// 14 days after the delivered notice; a Saturday, Sunday or nationwide
	// holiday moves the end to the next working day (§ 193 BGB).
	base := concludedOn.AddDays(carecontract.WithdrawalDays)
	until := *options.WithdrawalUntil
	assert.False(t, until.Before(base), "never shorter than 14 days")
	assert.LessOrEqual(t, base.DaysUntil(until), 4, "moved by weekend and holidays only")
	assert.NotContains(t, []time.Weekday{time.Saturday, time.Sunday}, until.Weekday(), "ends on a working day")
	require.NotNil(t, options.OrdinaryEndsOn)

	billing := traegerColleague(t, f.reviewFixture, identityaccess.TraegerRoleBilling)
	for _, method := range []string{http.MethodGet, http.MethodPost} {
		rec = traegerJSON(t, f.traeger, billing, method, path, map[string]any{"kind": "widerruf", "received_on": concludedOn.String()})
		assert.Equal(t, http.StatusForbidden, rec.Code, "%s: %s", method, rec.Body.String())
	}
	other := newTraegerOffice(t, f.db, f.traeger, "Träger Vertrag Kündigung Fremd", identityaccess.TraegerRoleAdministration)
	testpkg.OwnTraegerCareContracts(t, f.db, other.organization)
	for _, method := range []string{http.MethodGet, http.MethodPost} {
		rec = traegerJSON(t, f.traeger, other.token, method, path, map[string]any{"kind": "widerruf", "received_on": concludedOn.String()})
		assert.Equal(t, http.StatusNotFound, rec.Code, "%s: %s", method, rec.Body.String())
	}

	rec = traegerJSON(t, f.traeger, token, http.MethodPost, path, map[string]any{"kind": "kuendigung", "termination_kind": "sonderkuendigung",
		"received_on": concludedOn.String()})
	assert.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "care_contract.special_termination_closed")
	rec = traegerJSON(t, f.traeger, token, http.MethodPost, path, map[string]any{"kind": "kuendigung", "termination_kind": "ordentlich",
		"received_on": "2099-01-01"})
	assert.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "care_contract.field_invalid")

	rec = traegerJSON(t, f.traeger, token, http.MethodPost, path, map[string]any{"kind": "kuendigung", "termination_kind": "ordentlich",
		"received_on": concludedOn.String()})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	done := decodeSchoolPortalData[noticeOptionsBody](t, rec.Body.Bytes())
	assert.Equal(t, carecontract.StatusTerminated, done.Status)
	require.NotNil(t, done.Notice)
	assert.Equal(t, carecontract.NoticeChannelOffice, done.Notice.Channel)
	assert.Contains(t, done.Notice.ConfirmationText, "Lina", "the default template names the child")
	rec = traegerJSON(t, f.traeger, token, http.MethodPost, path, map[string]any{"kind": "widerruf", "received_on": concludedOn.String()})
	assert.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "care_contract.contract_not_endable", "a contract leaves aktiv once")

	var mails, terminations, reads, accessLogs int
	require.NoError(t, f.db.NewRaw(`SELECT COUNT(*) FROM platform.email_outbox WHERE organization_id = ? AND kind = ?
		AND related_entity_type = 'care_contract' AND related_entity_id = ?`, f.office.organization, carecontract.MailKindNotice, contract).Scan(ctx, &mails))
	require.NoError(t, f.db.NewRaw(`SELECT COUNT(*) FROM care_contract.contract_events WHERE contract_id = ? AND action = 'terminate'
		AND account_id IS NOT NULL`, contract).Scan(ctx, &terminations))
	require.NoError(t, f.db.NewRaw(`SELECT COUNT(*) FROM care_contract.contract_events WHERE contract_id = ? AND action = 'read'
		AND details->>'view' = 'notice_options'`, contract).Scan(ctx, &reads))
	require.NoError(t, f.db.NewRaw(`SELECT COUNT(*) FROM audit.data_access_log WHERE tenant_id = ? AND metadata->>'view' = 'care_contract_notice'`,
		f.school).Scan(ctx, &accessLogs))
	assert.Equal(t, 1, mails, "one confirmation in Textform")
	assert.Equal(t, 1, terminations, "the declaration under the office account")
	assert.Equal(t, 1, reads, "the read of the notice page")
	assert.Equal(t, 1, accessLogs, "the read in the access log of the school")

	rec = traegerJSON(t, f.traeger, token, http.MethodGet, path+"?received_on="+concludedOn.String(), nil)
	assert.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	for _, day := range []string{"2026-02-30", "2099-01-01", concludedOn.AddDays(-1).String()} {
		rec = traegerJSON(t, f.traeger, token, http.MethodGet, path+"?received_on="+day, nil)
		assert.Equal(t, http.StatusBadRequest, rec.Code, "%s: %s", day, rec.Body.String())
	}
}

// An extraordinary termination the office received keeps the parents'
// reason and an open end (#4682, § 312k Abs. 2 BGB); only the right
// "Verträge verwalten" of its own Träger sets the end, once, not before the
// receipt.
func TestCareContractNoticeEndIsSetByTheOfficeAfterItsCheck(t *testing.T) {
	t.Parallel()
	f := newCareContractFixture(t, "Träger Vertrag außerordentlich")
	token := f.office.token
	require.Equal(t, http.StatusOK, traegerJSON(t, f.traeger, token, http.MethodPut, templatePath(f.school, f.offering), careContractTemplate).Code)
	contract := concludeWithEarlyStart(t, f)
	linkContractGuardian(t, f, contract)
	path := "/traeger/contract-notices/" + strconv.FormatInt(contract, 10)
	var concludedOn calendar.Date
	require.NoError(t, f.db.NewRaw(`SELECT (concluded_at AT TIME ZONE 'Europe/Berlin')::date FROM care_contract.contracts WHERE id = ?`,
		contract).Scan(context.Background(), &concludedOn))

	rec := traegerJSON(t, f.traeger, token, http.MethodPost, path, map[string]any{"kind": "kuendigung",
		"termination_kind": "ausserordentlich", "received_on": concludedOn.String()})
	assert.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "reason", "an extraordinary termination names its reason")
	rec = traegerJSON(t, f.traeger, token, http.MethodPost, path, map[string]any{"kind": "kuendigung",
		"termination_kind": "ausserordentlich", "reason": "Umzug ins Ausland", "received_on": concludedOn.String()})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	done := decodeSchoolPortalData[noticeOptionsBody](t, rec.Body.Bytes())
	assert.Equal(t, carecontract.StatusTerminated, done.Status)
	require.NotNil(t, done.Notice)
	assert.Equal(t, carecontract.TerminationExtraordinary, done.Notice.TerminationKind)
	assert.Equal(t, "Umzug ins Ausland", done.Notice.Reason)
	assert.Nil(t, done.Notice.EndsOn)
	assert.Contains(t, done.Notice.ConfirmationText, "Umzug ins Ausland", "the default template repeats the reason")

	end := concludedOn.AddDays(60).String()
	billing := traegerColleague(t, f.reviewFixture, identityaccess.TraegerRoleBilling)
	rec = traegerJSON(t, f.traeger, billing, http.MethodPut, path+"/ends-on", map[string]any{"ends_on": end})
	assert.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
	other := newTraegerOffice(t, f.db, f.traeger, "Träger Vertrag außerordentlich Fremd", identityaccess.TraegerRoleAdministration)
	testpkg.OwnTraegerCareContracts(t, f.db, other.organization)
	rec = traegerJSON(t, f.traeger, other.token, http.MethodPut, path+"/ends-on", map[string]any{"ends_on": end})
	assert.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
	rec = traegerJSON(t, f.traeger, token, http.MethodPut, path+"/ends-on", map[string]any{"ends_on": concludedOn.AddDays(-1).String()})
	assert.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())

	rec = traegerJSON(t, f.traeger, token, http.MethodPut, path+"/ends-on", map[string]any{"ends_on": end})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	set := decodeSchoolPortalData[noticeOptionsBody](t, rec.Body.Bytes())
	require.NotNil(t, set.Notice)
	require.NotNil(t, set.Notice.EndsOn)
	assert.Equal(t, end, set.Notice.EndsOn.String())
	rec = traegerJSON(t, f.traeger, token, http.MethodPut, path+"/ends-on", map[string]any{"ends_on": end})
	assert.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "care_contract.notice_end_not_open")
	// The parents learn the last day in Textform (#4682), once.
	assert.Equal(t, carecontract.EndDecisionSet, set.Notice.EndDecision)
	assert.Contains(t, set.Notice.EndNoticeText, set.Notice.EndsOn.Format("02.01.2006"), "the default template names the last day")
	assert.Equal(t, 1, noticeEndMails(t, f, contract), "one notice of the end, also after the repetition")
}

// noticeEndMails counts the notices of a decided end in the outbox.
func noticeEndMails(t *testing.T, f careContractFixture, contract int64) int {
	t.Helper()
	var mails int
	require.NoError(t, f.db.NewRaw(`SELECT COUNT(*) FROM platform.email_outbox WHERE organization_id = ? AND kind = ?
		AND related_entity_type = 'care_contract' AND related_entity_id = ? AND payload->>'decision' <> ''`,
		f.office.organization, carecontract.MailKindNotice, contract).Scan(context.Background(), &mails))
	return mails
}

// The Träger-Büro does not accept the reason of an extraordinary
// termination (#4682, decision of 07.11.2026): only with "Verträge
// verwalten", only at its own Träger, only once and only while the end is
// open. The declaration holds as an ordinary termination to the end the
// office was shown; the parents get it in Textform once, and the refusal
// lands in the trail.
func TestCareContractNoticeReasonIsRefusedByTheOfficeOnce(t *testing.T) {
	t.Parallel()
	f := newCareContractFixture(t, "Träger Vertrag Grund abgelehnt")
	token := f.office.token
	require.Equal(t, http.StatusOK, traegerJSON(t, f.traeger, token, http.MethodPut, templatePath(f.school, f.offering), careContractTemplate).Code)
	contract := concludeWithEarlyStart(t, f)
	linkContractGuardian(t, f, contract)
	path := "/traeger/contract-notices/" + strconv.FormatInt(contract, 10)
	var concludedOn calendar.Date
	require.NoError(t, f.db.NewRaw(`SELECT (concluded_at AT TIME ZONE 'Europe/Berlin')::date FROM care_contract.contracts WHERE id = ?`,
		contract).Scan(context.Background(), &concludedOn))
	rec := traegerJSON(t, f.traeger, token, http.MethodPost, path+"/reason-rejection", map[string]any{})
	assert.Equal(t, http.StatusConflict, rec.Code, "an active contract has no reason to refuse: %s", rec.Body.String())
	rec = traegerJSON(t, f.traeger, token, http.MethodPost, path, map[string]any{"kind": "kuendigung",
		"termination_kind": "ausserordentlich", "reason": "Umzug ins Ausland", "received_on": concludedOn.String()})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	shown := decodeSchoolPortalData[noticeOptionsBody](t, rec.Body.Bytes()).ReinterpretedEndsOn
	require.NotNil(t, shown, "the office sees the end before it refuses the reason")

	billing := traegerColleague(t, f.reviewFixture, identityaccess.TraegerRoleBilling)
	rec = traegerJSON(t, f.traeger, billing, http.MethodPost, path+"/reason-rejection", map[string]any{"expected_ends_on": shown.String()})
	assert.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
	other := newTraegerOffice(t, f.db, f.traeger, "Träger Vertrag Grund abgelehnt Fremd", identityaccess.TraegerRoleAdministration)
	testpkg.OwnTraegerCareContracts(t, f.db, other.organization)
	rec = traegerJSON(t, f.traeger, other.token, http.MethodPost, path+"/reason-rejection", map[string]any{"expected_ends_on": shown.String()})
	assert.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
	rec = traegerJSON(t, f.traeger, token, http.MethodPost, path+"/reason-rejection", map[string]any{"expected_ends_on": shown.AddDays(-1).String()})
	assert.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "expected_ends_on")
	assert.Zero(t, noticeEndMails(t, f, contract), "no notice before the decision")

	rec = traegerJSON(t, f.traeger, token, http.MethodPost, path+"/reason-rejection", map[string]any{"expected_ends_on": shown.String()})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	refused := decodeSchoolPortalData[noticeOptionsBody](t, rec.Body.Bytes())
	require.NotNil(t, refused.Notice)
	require.NotNil(t, refused.Notice.EndsOn)
	assert.Equal(t, *shown, *refused.Notice.EndsOn)
	assert.Equal(t, carecontract.EndDecisionReinterpreted, refused.Notice.EndDecision)
	assert.Contains(t, refused.Notice.EndNoticeText, "ordentliche Kündigung", "the default template")
	assert.Contains(t, refused.Notice.EndNoticeText, shown.Format("02.01.2006"), "the notice names the new end")

	rec = traegerJSON(t, f.traeger, token, http.MethodPost, path+"/reason-rejection", map[string]any{"expected_ends_on": shown.String()})
	assert.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "care_contract.notice_end_not_open")
	rec = traegerJSON(t, f.traeger, token, http.MethodPut, path+"/ends-on", map[string]any{"ends_on": shown.String()})
	assert.Equal(t, http.StatusConflict, rec.Code, "the refusal and the set end exclude each other: %s", rec.Body.String())
	assert.Equal(t, 1, noticeEndMails(t, f, contract), "one notice, also after the repetition")
	var trail int
	require.NoError(t, f.db.NewRaw(`SELECT COUNT(*) FROM care_contract.contract_events WHERE contract_id = ? AND action = 'reject_reason'
		AND account_id IS NOT NULL`, contract).Scan(context.Background(), &trail))
	assert.Equal(t, 1, trail, "the refusal under the office account")
}

// linkContractGuardian makes the contract's child a student of the school
// whose guardian holds the contract party's address and account with the
// role of a primary guardian, which grants the enrollment submission. It
// answers the relationship.
func linkContractGuardian(t *testing.T, f careContractFixture, contract int64) int64 {
	t.Helper()
	ctx := context.Background()
	student := testpkg.CreateTestStudentForTenant(t, f.db, f.school, "Lina", "Muster", "1a")
	guardian := testpkg.CreateTestGuardianProfileForTenant(t, f.db, f.school, "Erika", "Muster", "erika.muster@example.org")
	account := testpkg.CreateTestAccount(t, f.db, fmt.Sprintf("erika-konto-%d@example.org", testpkg.UniqueTestTenantID(t)))
	_, err := f.db.ExecContext(ctx, `UPDATE users.guardian_profiles SET email = 'erika.muster@example.org', account_id = ?, has_account = true
		WHERE id = ?`, account.ID, guardian.ID)
	require.NoError(t, err)
	link := testpkg.CreateTestStudentGuardianLinkForTenant(t, f.db, f.school, student.ID, guardian.ID, "primary_guardian")
	require.True(t, link.Permissions[carecontract.GuardianPermissionEnrollmentSubmit] == true, "the preset grants the enrollment submission")
	_, err = f.db.ExecContext(ctx, `UPDATE care_contract.contracts SET student_id = ? WHERE id = ?`, student.ID, contract)
	require.NoError(t, err)
	_, err = f.db.ExecContext(ctx, `UPDATE care_contract.contract_parties SET account_id = ? WHERE contract_id = ?`, account.ID, contract)
	require.NoError(t, err)
	return link.ID
}

// The notice of a decided end reaches only a guardian whose relationship
// to the child still grants the enrollment submission (#4682,
// .claude/rules/guardian-parent-permissions.md). Through the real router:
// the school revoked the parent's right after the declaration; the office
// sets the end, the decision stands, no mail goes to the old address, and
// the card tells the office that the parents got no notice.
func TestCareContractEndNoticeSkipsAParentWhoseRightWasRevoked(t *testing.T) {
	t.Parallel()
	f := newCareContractFixture(t, "Träger Vertrag Recht entzogen")
	token := f.office.token
	require.Equal(t, http.StatusOK, traegerJSON(t, f.traeger, token, http.MethodPut, templatePath(f.school, f.offering), careContractTemplate).Code)
	contract := concludeWithEarlyStart(t, f)
	link := linkContractGuardian(t, f, contract)
	path := "/traeger/contract-notices/" + strconv.FormatInt(contract, 10)
	var concludedOn calendar.Date
	require.NoError(t, f.db.NewRaw(`SELECT (concluded_at AT TIME ZONE 'Europe/Berlin')::date FROM care_contract.contracts WHERE id = ?`,
		contract).Scan(context.Background(), &concludedOn))
	rec := traegerJSON(t, f.traeger, token, http.MethodPost, path, map[string]any{"kind": "kuendigung",
		"termination_kind": "ausserordentlich", "reason": "Umzug ins Ausland", "received_on": concludedOn.String()})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	require.NoError(t, testpkg.SetTestStudentGuardianPermissions(context.Background(), f.db, link,
		map[string]interface{}{"parent_portal.access": true}))
	rec = traegerJSON(t, f.traeger, token, http.MethodPut, path+"/ends-on", map[string]any{"ends_on": concludedOn.AddDays(30).String()})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	set := decodeSchoolPortalData[noticeOptionsBody](t, rec.Body.Bytes())
	require.NotNil(t, set.Notice)
	assert.Equal(t, carecontract.EndDecisionSet, set.Notice.EndDecision, "the decision stands")
	assert.False(t, set.Notice.EndNoticeSent, "the office sees that no mail went out")
	assert.Zero(t, noticeEndMails(t, f, contract), "no mail to the address of the parent whose right was revoked")
}
