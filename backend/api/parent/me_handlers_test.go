package parent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	mealplanModels "github.com/moto-nrw/project-phoenix/models/mealplan"
	parentModels "github.com/moto-nrw/project-phoenix/models/parent"
	suggestionsModels "github.com/moto-nrw/project-phoenix/models/suggestions"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	parentService "github.com/moto-nrw/project-phoenix/services/parent"
)

// fakeParentService implements parentService.Service. Only the profile methods
// carry behaviour; the rest satisfy the interface for the handler tests.
type fakeParentService struct {
	getProfile    *parentService.Profile
	getProfileErr error

	updateProfile     *parentService.Profile
	updateProfileErr  error
	gotUpdateLocale   string
	updateCalledCount int

	masterData       *parentService.ChildMasterData
	masterDataErr    error
	updateMasterData *parentService.ChildMasterData
	updateMasterErr  error
	gotMasterAccount int64
	gotMasterStudent int64
	gotMasterTarget  string
	gotMasterField   string
	gotMasterValue   json.RawMessage
	submitRows       []*userModels.StudentDataChangeRequest
	submitErr        error
	gotSubmitChanges []parentService.MasterDataFieldChange
	listRows         []*userModels.StudentDataChangeRequest
	listErr          error
	submitStatus     *parentModels.GuardianSubmitStatus
}

func (f *fakeParentService) GetProfile(_ context.Context, _ int64) (*parentService.Profile, error) {
	return f.getProfile, f.getProfileErr
}

func (f *fakeParentService) UpdatePortalLocale(_ context.Context, _ int64, locale string) (*parentService.Profile, error) {
	f.updateCalledCount++
	f.gotUpdateLocale = locale
	return f.updateProfile, f.updateProfileErr
}

func (f *fakeParentService) ListChildrenForAccount(context.Context, int64) ([]*parentModels.ChildSummary, error) {
	return nil, nil
}
func (f *fakeParentService) ListEnrollableForAccount(context.Context, int64) ([]*parentModels.EnrollablePhase, error) {
	return nil, nil
}
func (f *fakeParentService) GetEnrollmentSubmitStatus(context.Context, int64, int64) (*parentModels.GuardianSubmitStatus, error) {
	if f.submitStatus != nil {
		return f.submitStatus, nil
	}
	return &parentModels.GuardianSubmitStatus{}, nil
}
func (f *fakeParentService) ListEnrollmentsForAccount(context.Context, int64) ([]*parentModels.EnrollmentRequestSummary, error) {
	return nil, nil
}
func (f *fakeParentService) SubmitSickNote(context.Context, int64, int64, []timezone.Date, string, string) (*parentService.SickNoteResult, error) {
	return &parentService.SickNoteResult{}, nil
}
func (f *fakeParentService) ListSickDays(context.Context, int64, int64, timezone.Date, timezone.Date) ([]*activeModels.StudentStatusDay, error) {
	return nil, nil
}
func (f *fakeParentService) ListExcusedRequests(context.Context, int64, int64) ([]*activeModels.ExcusedAbsenceRequest, error) {
	return nil, nil
}
func (f *fakeParentService) WithdrawExcusedRequest(context.Context, int64, int64, int64) (*activeModels.ExcusedAbsenceRequest, error) {
	return nil, nil
}
func (f *fakeParentService) ChildFeatures(context.Context, int64, int64) (parentService.ChildFeatureFlags, error) {
	return parentService.ChildFeatureFlags{}, nil
}
func (f *fakeParentService) MealPlanWeek(context.Context, int64, int64, timezone.Date) ([]*mealplanModels.MealPlanEntry, error) {
	return nil, nil
}
func (f *fakeParentService) ListRelatedAccounts(context.Context, int64, int64) ([]*parentService.RelatedAccount, error) {
	return nil, nil
}
func (f *fakeParentService) InviteRelatedAccount(context.Context, int64, int64, string, string, string) (*parentService.InviteRelatedAccountResult, error) {
	return nil, nil
}
func (f *fakeParentService) RemoveRelatedAccount(context.Context, int64, int64, int64) error {
	return nil
}

func (f *fakeParentService) ListMessageThreads(context.Context, int64) ([]*userModels.InboxThread, error) {
	return nil, nil
}

func (f *fakeParentService) ListChildThreads(context.Context, int64, int64) ([]*userModels.InboxThread, error) {
	return nil, nil
}

func (f *fakeParentService) UnreadMessageCount(context.Context, int64) (int, error) {
	return 0, nil
}

func (f *fakeParentService) GetChildConversation(context.Context, int64, int64) (*parentService.MessageThreadView, error) {
	return nil, nil
}

func (f *fakeParentService) PostChildMessage(context.Context, int64, int64, string) (*parentService.MessageThreadView, error) {
	return nil, nil
}

func (f *fakeParentService) GetChildCareSchedule(context.Context, int64, int64) (*parentService.ChildCareSchedule, error) {
	return nil, nil
}

func (f *fakeParentService) CreateCareScheduleRequest(context.Context, int64, int64, map[string]any) (*parentService.ChildCareSchedule, error) {
	return nil, nil
}

func (f *fakeParentService) WithdrawCareScheduleRequest(context.Context, int64, int64, int64) (*parentService.ChildCareSchedule, error) {
	return nil, nil
}

func (f *fakeParentService) SubmitCareException(context.Context, int64, int64, timezone.Date, *time.Time, *time.Time) (*parentService.CareException, error) {
	return nil, nil
}

func (f *fakeParentService) ListCareExceptions(context.Context, int64, int64, timezone.Date, timezone.Date) ([]*parentService.CareException, error) {
	return nil, nil
}

func (f *fakeParentService) DeleteCareException(context.Context, int64, int64, timezone.Date) error {
	return nil
}

func (f *fakeParentService) GetChildMasterData(context.Context, int64, int64) (*parentService.ChildMasterData, error) {
	return f.masterData, f.masterDataErr
}

func (f *fakeParentService) UpdateMasterDataField(_ context.Context, accountID, studentID int64, target, field string, value json.RawMessage) (*parentService.ChildMasterData, error) {
	f.gotMasterAccount = accountID
	f.gotMasterStudent = studentID
	f.gotMasterTarget = target
	f.gotMasterField = field
	f.gotMasterValue = value
	return f.updateMasterData, f.updateMasterErr
}

func (f *fakeParentService) SubmitMasterDataChangeRequest(_ context.Context, accountID, studentID int64, changes []parentService.MasterDataFieldChange) ([]*userModels.StudentDataChangeRequest, error) {
	f.gotMasterAccount = accountID
	f.gotMasterStudent = studentID
	f.gotSubmitChanges = changes
	return f.submitRows, f.submitErr
}

func (f *fakeParentService) ListMyMasterDataRequests(context.Context, int64, int64) ([]*userModels.StudentDataChangeRequest, error) {
	return f.listRows, f.listErr
}

func (f *fakeParentService) ListChildGuardians(context.Context, int64, int64) ([]*parentService.ChildGuardian, error) {
	return nil, nil
}

func (f *fakeParentService) UpdateGuardianContact(context.Context, int64, int64, int64, parentService.GuardianContactInput) (*parentService.ChildGuardian, error) {
	return nil, nil
}

func (f *fakeParentService) UpdateGuardianRelationship(context.Context, int64, int64, int64, parentService.GuardianRelationshipInput) (*parentService.ChildGuardian, error) {
	return nil, nil
}

// Parent-news feed (#1669) — interface stubs; the announcement handlers have
// their own tests.
func (f *fakeParentService) ListAnnouncements(context.Context, int64) ([]*userModels.AnnouncementFeedItem, error) {
	return nil, nil
}

func (f *fakeParentService) UnreadAnnouncementCount(context.Context, int64) (int, error) {
	return 0, nil
}

func (f *fakeParentService) MarkAnnouncementRead(context.Context, int64, int64, time.Time) error {
	return nil
}

func (f *fakeParentService) AcknowledgeAnnouncement(context.Context, int64, int64, time.Time) error {
	return nil
}

// Feedback board (#1678) — stubs so the fake keeps satisfying parent.Service.
// The board's own handler tests drive the real service against the database.

func (f *fakeParentService) ListFeedbackSchools(context.Context, int64) ([]*parentService.FeedbackSchool, error) {
	return nil, nil
}

func (f *fakeParentService) ListFeedback(context.Context, int64, int64, string) ([]*suggestionsModels.Post, error) {
	return nil, nil
}

func (f *fakeParentService) CreateFeedback(context.Context, int64, int64, string, string) (*suggestionsModels.Post, error) {
	return nil, nil
}

func (f *fakeParentService) GetFeedback(context.Context, int64, int64, int64) (*suggestionsModels.Post, error) {
	return nil, nil
}

func (f *fakeParentService) UpdateFeedback(context.Context, int64, int64, int64, string, string) (*suggestionsModels.Post, error) {
	return nil, nil
}

func (f *fakeParentService) DeleteFeedback(context.Context, int64, int64, int64) error {
	return nil
}

func (f *fakeParentService) VoteFeedback(context.Context, int64, int64, int64, string) (*suggestionsModels.Post, error) {
	return nil, nil
}

func (f *fakeParentService) RemoveFeedbackVote(context.Context, int64, int64, int64) (*suggestionsModels.Post, error) {
	return nil, nil
}

func (f *fakeParentService) ListFeedbackComments(context.Context, int64, int64, int64) ([]*suggestionsModels.Comment, error) {
	return nil, nil
}

func (f *fakeParentService) CreateFeedbackComment(context.Context, int64, int64, int64, string) error {
	return nil
}

func (f *fakeParentService) DeleteFeedbackComment(context.Context, int64, int64, int64) error {
	return nil
}

func (f *fakeParentService) MarkFeedbackCommentsRead(context.Context, int64, int64, int64) error {
	return nil
}

func (f *fakeParentService) FeedbackUnreadCount(context.Context, int64) (int, error) {
	return 0, nil
}

// withClaims attaches a parent account id to the request context the way the
// JWT middleware does in production.
func withClaims(r *http.Request, accountID int) *http.Request {
	ctx := context.WithValue(r.Context(), jwt.CtxClaims, jwt.AppClaims{ID: accountID})
	return r.WithContext(ctx)
}

func TestGetMyProfile_Unauthorized_WhenNoClaims(t *testing.T) {
	rs := &Resource{ParentService: &fakeParentService{}}
	req := httptest.NewRequest(http.MethodGet, "/me/profile", nil)
	w := httptest.NewRecorder()

	rs.getMyProfile(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code,
		"a missing/zero account id must be rejected, never defaulted to account 0")
}

func TestGetMyProfile_ReturnsNullWhenNeverChosen(t *testing.T) {
	rs := &Resource{ParentService: &fakeParentService{
		getProfile: &parentService.Profile{Locale: "de", Explicit: false},
	}}
	req := withClaims(httptest.NewRequest(http.MethodGet, "/me/profile", nil), 1234)
	w := httptest.NewRecorder()

	rs.getMyProfile(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"portal_locale":null`,
		"an unset preference must serialize as null so the client keeps the anonymous locale")
}

func TestGetMyProfile_ReturnsExplicitLocale(t *testing.T) {
	rs := &Resource{ParentService: &fakeParentService{
		getProfile: &parentService.Profile{Locale: "en", Explicit: true},
	}}
	req := withClaims(httptest.NewRequest(http.MethodGet, "/me/profile", nil), 1234)
	w := httptest.NewRecorder()

	rs.getMyProfile(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"portal_locale":"en"`)
}

func TestGetMyProfile_PropagatesServiceError(t *testing.T) {
	rs := &Resource{ParentService: &fakeParentService{
		getProfileErr: errors.New("boom"),
	}}
	req := withClaims(httptest.NewRequest(http.MethodGet, "/me/profile", nil), 1234)
	w := httptest.NewRecorder()

	rs.getMyProfile(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestUpdateMyProfile_RejectsUnsupportedLocale(t *testing.T) {
	fake := &fakeParentService{}
	rs := &Resource{ParentService: fake}
	req := withClaims(
		httptest.NewRequest(http.MethodPut, "/me/profile", strings.NewReader(`{"portal_locale":"xx"}`)),
		1234,
	)
	w := httptest.NewRecorder()

	rs.updateMyProfile(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code,
		"an unknown locale must fail loudly, not be silently coerced to the default")
	assert.Equal(t, 0, fake.updateCalledCount, "the service must not be touched for an invalid locale")
}

func TestUpdateMyProfile_RejectsMissingLocale(t *testing.T) {
	fake := &fakeParentService{}
	rs := &Resource{ParentService: fake}
	req := withClaims(
		httptest.NewRequest(http.MethodPut, "/me/profile", strings.NewReader(`{}`)),
		1234,
	)
	w := httptest.NewRecorder()

	rs.updateMyProfile(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Equal(t, 0, fake.updateCalledCount)
}

func TestUpdateMyProfile_RejectsMalformedBody(t *testing.T) {
	fake := &fakeParentService{}
	rs := &Resource{ParentService: fake}
	req := withClaims(
		httptest.NewRequest(http.MethodPut, "/me/profile", strings.NewReader(`not json`)),
		1234,
	)
	w := httptest.NewRecorder()

	rs.updateMyProfile(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Equal(t, 0, fake.updateCalledCount)
}

func TestUpdateMyProfile_Unauthorized_WhenNoClaims(t *testing.T) {
	rs := &Resource{ParentService: &fakeParentService{}}
	req := httptest.NewRequest(http.MethodPut, "/me/profile", strings.NewReader(`{"portal_locale":"en"}`))
	w := httptest.NewRecorder()

	rs.updateMyProfile(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestUpdateMyProfile_PersistsSupportedLocale(t *testing.T) {
	fake := &fakeParentService{
		updateProfile: &parentService.Profile{Locale: "en", Explicit: true},
	}
	rs := &Resource{ParentService: fake}
	req := withClaims(
		httptest.NewRequest(http.MethodPut, "/me/profile", strings.NewReader(`{"portal_locale":"en"}`)),
		1234,
	)
	w := httptest.NewRecorder()

	rs.updateMyProfile(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, 1, fake.updateCalledCount)
	assert.Equal(t, "en", fake.gotUpdateLocale)

	var envelope struct {
		Data ParentProfileResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &envelope))
	require.NotNil(t, envelope.Data.PortalLocale)
	assert.Equal(t, "en", *envelope.Data.PortalLocale)
}

// A missing guardian_profiles row for an authenticated parent is a data
// integrity fault (the role and the profile link are written together), not a
// transient server error — the handler must surface it as 409, not 500, so it
// reads as a permanent state conflict rather than something worth retrying.
func TestUpdateMyProfile_MapsMissingProfileToConflict(t *testing.T) {
	fake := &fakeParentService{
		updateProfileErr: fmt.Errorf("parent: update portal locale: %w", userModels.ErrGuardianProfileNotFound),
	}
	rs := &Resource{ParentService: fake}
	req := withClaims(
		httptest.NewRequest(http.MethodPut, "/me/profile", strings.NewReader(`{"portal_locale":"en"}`)),
		1234,
	)
	w := httptest.NewRecorder()

	rs.updateMyProfile(w, req)

	assert.Equal(t, http.StatusConflict, w.Code,
		"missing guardian profile must map to 409, not 500")
}

// Any other service error stays a 500 — only the missing-profile sentinel is
// remapped, so genuine faults aren't downgraded to a client-conflict status.
func TestUpdateMyProfile_PropagatesOtherErrorsAs500(t *testing.T) {
	fake := &fakeParentService{
		updateProfileErr: errors.New("db exploded"),
	}
	rs := &Resource{ParentService: fake}
	req := withClaims(
		httptest.NewRequest(http.MethodPut, "/me/profile", strings.NewReader(`{"portal_locale":"en"}`)),
		1234,
	)
	w := httptest.NewRecorder()

	rs.updateMyProfile(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}
