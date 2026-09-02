package parent

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	parentService "github.com/moto-nrw/project-phoenix/services/parent"
)

func TestGetChildConsentsReturnsParentFacingStates(t *testing.T) {
	t.Parallel()

	changedAt := time.Date(2026, time.August, 31, 9, 30, 0, 0, time.UTC)
	fake := &fakeParentService{consents: []parentService.ChildConsent{
		{Key: "agb", State: "granted", ChangedAt: &changedAt},
		{Key: "photo", State: "granted", ChangedAt: &changedAt, CanWithdraw: true},
	}}
	rs := &Resource{ParentService: fake}
	req := parentRequestWithStudentID(http.MethodGet, "/children/42/consents", "", "42")
	w := httptest.NewRecorder()

	rs.getChildConsents(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, int64(1234), fake.gotConsentAccount)
	assert.Equal(t, int64(42), fake.gotConsentStudent)
	assert.Contains(t, w.Body.String(), `"key":"photo"`)
	assert.Contains(t, w.Body.String(), `"changed_at":"2026-08-31T09:30:00Z"`)
	assert.Contains(t, w.Body.String(), `"can_withdraw":true`)
	assert.Contains(t, w.Body.String(), `"can_grant":false`)
}

func TestWithdrawPhotoConsentReturnsUpdatedState(t *testing.T) {
	t.Parallel()

	changedAt := time.Date(2026, time.August, 31, 9, 45, 0, 0, time.UTC)
	fake := &fakeParentService{withdrawConsents: []parentService.ChildConsent{
		{Key: "photo", State: "withdrawn", ChangedAt: &changedAt},
	}}
	rs := &Resource{ParentService: fake}
	req := parentRequestWithStudentID(http.MethodDelete, "/children/42/consents/photo", "", "42")
	w := httptest.NewRecorder()

	rs.withdrawPhotoConsent(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, int64(1234), fake.gotConsentAccount)
	assert.Equal(t, int64(42), fake.gotConsentStudent)
	assert.Contains(t, w.Body.String(), `"state":"withdrawn"`)
}

func TestWithdrawPhotoConsentMapsGuardianPermissionError(t *testing.T) {
	t.Parallel()

	fake := &fakeParentService{withdrawConsentErr: parentService.ErrGuardianPermissionDenied}
	rs := &Resource{ParentService: fake}
	req := parentRequestWithStudentID(http.MethodDelete, "/children/42/consents/photo", "", "42")
	w := httptest.NewRecorder()

	rs.withdrawPhotoConsent(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Contains(t, w.Body.String(), `"code":"guardian_permission_denied"`)
}

func TestGrantPhotoConsentReturnsUpdatedState(t *testing.T) {
	t.Parallel()

	changedAt := time.Date(2026, time.September, 1, 8, 15, 0, 0, time.UTC)
	fake := &fakeParentService{grantConsents: []parentService.ChildConsent{
		{Key: "photo", State: "granted", ChangedAt: &changedAt, CanWithdraw: true},
	}}
	rs := &Resource{ParentService: fake}
	req := parentRequestWithStudentID(http.MethodPut, "/children/42/consents/photo", "{}", "42")
	w := httptest.NewRecorder()

	rs.grantPhotoConsent(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, int64(1234), fake.gotConsentAccount)
	assert.Equal(t, int64(42), fake.gotConsentStudent)
	assert.Contains(t, w.Body.String(), `"state":"granted"`)
	assert.Contains(t, w.Body.String(), `"can_withdraw":true`)
}

func TestGrantPhotoConsentRequiresPreviousWithdrawal(t *testing.T) {
	t.Parallel()

	fake := &fakeParentService{grantConsentErr: parentService.ErrPhotoConsentNotWithdrawn}
	rs := &Resource{ParentService: fake}
	req := parentRequestWithStudentID(http.MethodPut, "/children/42/consents/photo", "{}", "42")
	w := httptest.NewRecorder()

	rs.grantPhotoConsent(w, req)

	assert.Equal(t, http.StatusConflict, w.Code)
	assert.Contains(t, w.Body.String(), `"code":"photo_consent_not_withdrawn"`)
}

func TestGrantPhotoConsentMapsEndedCareError(t *testing.T) {
	t.Parallel()

	fake := &fakeParentService{grantConsentErr: parentService.ErrChildCareEnded}
	rs := &Resource{ParentService: fake}
	req := parentRequestWithStudentID(http.MethodPut, "/children/42/consents/photo", "{}", "42")
	w := httptest.NewRecorder()

	rs.grantPhotoConsent(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Contains(t, w.Body.String(), `"code":"child_care_ended"`)
}
