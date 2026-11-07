package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/carecontract"
	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// noticeOptionsBody is the answer of /traeger/contract-notices/{id}.
type noticeOptionsBody struct {
	Status          string         `json:"status"`
	OrdinaryEndsOn  *calendar.Date `json:"ordinary_ends_on"`
	WithdrawalUntil *calendar.Date `json:"withdrawal_until"`
	EarlyStart      bool           `json:"early_start"`
	Notice          *struct {
		Kind             string         `json:"kind"`
		Channel          string         `json:"channel"`
		EndsOn           *calendar.Date `json:"ends_on"`
		ConfirmationText string         `json:"confirmation_text"`
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
	block := func(key, text string, required bool) enrollment.LegalBlockSnapshot {
		return enrollment.LegalBlockSnapshot{Key: key, Kind: enrollment.LegalBlockKindTerms, Source: enrollment.LegalBlockSourceCustom,
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
	assert.True(t, informed, "the PDF of the conclusion carries the withdrawal notice")

	rec := traegerJSON(t, f.traeger, token, http.MethodGet, path, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	options := decodeSchoolPortalData[noticeOptionsBody](t, rec.Body.Bytes())
	assert.Equal(t, carecontract.StatusActive, options.Status)
	assert.True(t, options.EarlyStart, "the ticked early start is stored with the agreement")
	require.NotNil(t, options.WithdrawalUntil)
	assert.Equal(t, concludedOn.AddDays(carecontract.WithdrawalDays), *options.WithdrawalUntil)
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
	assert.Contains(t, rec.Body.String(), "care_contract.contract_input_invalid")

	rec = traegerJSON(t, f.traeger, token, http.MethodPost, path, map[string]any{"kind": "kuendigung", "termination_kind": "ordentlich",
		"received_on": concludedOn.String()})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	done := decodeSchoolPortalData[noticeOptionsBody](t, rec.Body.Bytes())
	assert.Equal(t, carecontract.StatusTerminated, done.Status)
	require.NotNil(t, done.Notice)
	assert.Equal(t, carecontract.ChannelOffice, done.Notice.Channel)
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
