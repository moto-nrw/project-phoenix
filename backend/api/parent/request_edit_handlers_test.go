package parent_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"time"

	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// TestSickNoteEnvelopeAndRecipientValidation pins the two create-path changes
// of #2267: the bare status-day array stays the default (old tabs call .map()
// on it), ?envelope=1 opts into {status_days, pending_request}, and a
// malformed recipient id is refused with its own code before anything is
// written.
func TestSickNoteEnvelopeAndRecipientValidation(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	router := newWriteRouterWithSettings(t, db, absenceApprovalOnSettings{})
	token := parentToken(t, chain.AccountID)
	path := fmt.Sprintf("/me/children/%d/sick-note", chain.StudentID)

	bare := doRequest(t, router, http.MethodPost, path, token, map[string]any{
		"dates": []string{testDay(6).String()}, "reason": "Familienfeier",
		"status": absenceStatusExcused,
	})
	require.Equal(t, http.StatusCreated, bare.Code, bare.Body.String())
	var bareEnv envelope
	require.NoError(t, json.Unmarshal(bare.Body.Bytes(), &bareEnv))
	var days []map[string]any
	require.NoError(t, json.Unmarshal(bareEnv.Data, &days), "the default response stays a bare array")
	assert.Empty(t, days, "a gated absence writes no status day")

	enveloped := doRequest(t, router, http.MethodPost, path+"?envelope=1", token, map[string]any{
		"dates": []string{testDay(7).String()}, "reason": "Familienfeier",
		"status": absenceStatusExcused,
	})
	require.Equal(t, http.StatusCreated, enveloped.Code, enveloped.Body.String())
	var envEnv envelope
	require.NoError(t, json.Unmarshal(enveloped.Body.Bytes(), &envEnv))
	var result struct {
		StatusDays     []map[string]any `json:"status_days"`
		PendingRequest *struct {
			ID string `json:"id"`
		} `json:"pending_request"`
	}
	require.NoError(t, json.Unmarshal(envEnv.Data, &result))
	assert.Empty(t, result.StatusDays)
	require.NotNil(t, result.PendingRequest, "the envelope names the request the school gated the absence into")
	assert.NotEmpty(t, result.PendingRequest.ID)

	bad := doRequest(t, router, http.MethodPost, path, token, map[string]any{
		"dates": []string{testDay(8).String()}, "reason": "Familienfeier",
		"status":                         absenceStatusExcused,
		"recipient_guardian_profile_ids": []string{"nicht-numerisch"},
	})
	require.Equal(t, http.StatusBadRequest, bad.Code, bad.Body.String())
	assert.Contains(t, bad.Body.String(), "invalid_recipients")
}

// TestEditExcusedRequestEndpoint pins the wire contract of the guardian edit
// (#2267): 200 with the rewritten request, 409 change_request_stale on an
// outdated version, 404 for a request that is not the caller's.
func TestEditExcusedRequestEndpoint(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	router := newWriteRouterWithSettings(t, db, absenceApprovalOnSettings{})
	token := parentToken(t, chain.AccountID)
	day := testDay(4)

	created := doRequest(t, router, http.MethodPost,
		fmt.Sprintf("/me/children/%d/sick-note", chain.StudentID), token,
		map[string]any{"dates": []string{day.String()}, "reason": "Familienfeier", "status": absenceStatusExcused})
	require.Equal(t, http.StatusCreated, created.Code, created.Body.String())

	listed := doRequest(t, router, http.MethodGet,
		fmt.Sprintf("/me/children/%d/excused-requests", chain.StudentID), token, nil)
	require.Equal(t, http.StatusOK, listed.Code, listed.Body.String())
	var listEnv envelope
	require.NoError(t, json.Unmarshal(listed.Body.Bytes(), &listEnv))
	var requests []struct {
		ID              string   `json:"id"`
		Dates           []string `json:"dates"`
		ExpectedVersion string   `json:"expected_version"`
	}
	require.NoError(t, json.Unmarshal(listEnv.Data, &requests))
	require.Len(t, requests, 1)
	requestID := requests[0].ID

	corrected := testDayPlus(day, 1)
	path := fmt.Sprintf("/me/children/%d/excused-requests/%s", chain.StudentID, requestID)
	ok := doRequest(t, router, http.MethodPut, path, token, map[string]any{
		"dates":            []string{corrected.String()},
		"note":             "Doch einen Tag später",
		"expected_version": requests[0].ExpectedVersion,
	})
	require.Equal(t, http.StatusOK, ok.Code, ok.Body.String())
	var okEnv envelope
	require.NoError(t, json.Unmarshal(ok.Body.Bytes(), &okEnv))
	var edited struct {
		ID    string   `json:"id"`
		Dates []string `json:"dates"`
	}
	require.NoError(t, json.Unmarshal(okEnv.Data, &edited))
	assert.Equal(t, requestID, edited.ID, "the edit keeps the request id")
	assert.Equal(t, []string{corrected.String()}, edited.Dates)

	// A version that is not the request's current one is refused, so an edit
	// taken on an outdated view can never overwrite a newer state.
	stale := doRequest(t, router, http.MethodPut, path, token, map[string]any{
		"dates":            []string{testDayPlus(corrected, 1).String()},
		"note":             "Nochmal",
		"expected_version": "2020-01-01T00:00:00Z",
	})
	require.Equal(t, http.StatusConflict, stale.Code, stale.Body.String())
	assert.Contains(t, stale.Body.String(), "change_request_stale")

	// A request id that belongs to nobody the caller may act for is missing.
	missing := doRequest(t, router, http.MethodPut,
		fmt.Sprintf("/me/children/%d/excused-requests/999999999", chain.StudentID), token,
		map[string]any{"dates": []string{corrected.String()}, "note": "Fremd"})
	assert.Equal(t, http.StatusNotFound, missing.Code, missing.Body.String())
}

// absenceStatusExcused is the wire value of the "excused" absence kind. It is
// spelled out rather than imported so this external test package stays clear
// of the student-presence domain (backend architecture policy).
const absenceStatusExcused = "excused"

// testDate is a calendar day as the wire carries it (YYYY-MM-DD).
type testDate string

func (d testDate) String() string { return string(d) }

func testDay(offset int) testDate {
	return testDate(time.Now().AddDate(0, 0, offset).Format(time.DateOnly))
}

func testDayPlus(day testDate, offset int) testDate {
	parsed, err := time.Parse(time.DateOnly, string(day))
	if err != nil {
		return day
	}
	return testDate(parsed.AddDate(0, 0, offset).Format(time.DateOnly))
}
